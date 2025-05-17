package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// detectImageFormat determines if the file is a TGA or PNG based on file extension
// and decodes the image data appropriately
func detectImageFormat(filePath string, data []byte) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".png":
		// For PNG, we need to create a reader from the byte slice
		reader := bytes.NewReader(data)
		return png.Decode(reader)
	case ".tga":
		// For TGA, we can use our custom decoder
		return decodeTGA(data)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}
}

// decodeTGA decodes a TGA image file into an image.Image
func decodeTGA(data []byte) (image.Image, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("TGA data too short")
	}
	// Parse TGA header
	// Note: idLength is at index 0, but we don't need it
	colorMapType := data[1]
	imageType := data[2]
	// We only support uncompressed true-color images
	if colorMapType != 0 || imageType != 2 {
		return nil, fmt.Errorf("unsupported TGA format: colorMapType=%d, imageType=%d", colorMapType, imageType)
	}

	// Read dimensions
	width := int(data[12]) | int(data[13])<<8
	height := int(data[14]) | int(data[15])<<8
	bpp := int(data[16])

	if bpp != 32 {
		return nil, fmt.Errorf("only 32-bit TGA supported, got %d", bpp)
	}

	// Read image descriptor
	imgDesc := data[17]
	topToBottom := (imgDesc & 0x20) == 0x20

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Parse pixel data
	pixelData := data[18:]
	for y := range height {
		yPos := y
		if !topToBottom {
			yPos = height - 1 - y
		}

		for x := range width {
			pos := (y*width + x) * 4
			if pos+3 >= len(pixelData) {
				return nil, fmt.Errorf("TGA data truncated")
			} // TGA stores in BGRA format
			b := pixelData[pos]
			g := pixelData[pos+1]
			r := pixelData[pos+2]
			a := pixelData[pos+3]

			img.SetRGBA(x, yPos, color.RGBA{r, g, b, a})
		}
	}

	return img, nil
}

// convertImageToCnv converts a TGA or PNG file back to the proprietary CNV format
func convertImageToCnv(filePath string) ([]byte, error) {
	// Read the image file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading image file: %w", err)
	}

	// Determine input file format based on extension
	ext := strings.ToLower(filepath.Ext(filePath))
	fmt.Printf("Converting image from %s format to CNV\n", ext)

	// Decode the image using the appropriate format handler
	img, err := detectImageFormat(filePath, fileData)
	if err != nil {
		return nil, fmt.Errorf("error decoding image: %w", err)
	}

	// Get image dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create CNV header (17 bytes)
	cnvData := make([]byte, 17)
	cnvData[0] = 32 // BPP - we always use 32-bit

	// Write dimensions
	binary.LittleEndian.PutUint32(cnvData[1:5], uint32(width))
	binary.LittleEndian.PutUint32(cnvData[5:9], uint32(height))
	binary.LittleEndian.PutUint32(cnvData[9:13], uint32(width))
	// Last 4 bytes are reserved

	// Convert pixel data
	pixelData := make([]byte, width*height*4)

	// CNV format expects data in bottom-up order (like TGA)
	// If source is PNG (top-down), we need to flip the rows
	// If source is TGA (already bottom-up), we keep as is
	for y := range height {
		for x := range width {
			r, g, b, a := img.At(x, y).RGBA()

			var pos int
			if ext == ".png" {
				// Flip the row order for PNG source (top-down to bottom-up)
				pos = 4 * ((height-1-y)*width + x)
			} else {
				// TGA is already in bottom-up order, so we use direct mapping
				pos = 4 * (y*width + x)
			}

			if pos+3 >= len(pixelData) {
				return nil, fmt.Errorf("pixel data buffer overflow at position (%d,%d)", x, y)
			}
			pixelData[pos] = byte(b >> 8)
			pixelData[pos+1] = byte(g >> 8)
			pixelData[pos+2] = byte(r >> 8)
			pixelData[pos+3] = byte(a >> 8)
		}
	}

	// Combine header and pixel data
	cnvData = append(cnvData, pixelData...)

	fmt.Printf("Successfully converted image: %dx%d pixels, %d bytes\n", width, height, len(cnvData))
	return cnvData, nil
}
