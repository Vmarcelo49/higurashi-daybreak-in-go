package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
)

// detectImageFormat determines if the file is a TGA or BMP and decodes the image data
// Note: BMP is the preferred format; TGA support is maintained for backward compatibility
func detectImageFormat(filePath string, data []byte) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".tga":
		// For TGA, we can use our custom decoder
		return decodeTGA(data)
	case ".bmp":
		// For BMP, we can use our custom decoder
		return decodeBMP(data)
	default:
		return nil, fmt.Errorf("unsupported image format: %s (only TGA and BMP are supported, BMP is preferred)", ext)
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

// decodeBMP decodes a BMP image file into an image.Image
func decodeBMP(data []byte) (image.Image, error) {
	if len(data) < 54 {
		return nil, fmt.Errorf("BMP data too short (need at least 54 bytes)")
	}

	// Check BMP signature
	if data[0] != 0x42 || data[1] != 0x4D {
		return nil, fmt.Errorf("invalid BMP signature")
	}
	// Read BMP header fields
	fileSize := binary.LittleEndian.Uint32(data[2:6])
	dataOffset := binary.LittleEndian.Uint32(data[10:14])
	_ = binary.LittleEndian.Uint32(data[14:18]) // headerSize - not used but part of BMP format
	width := int32(binary.LittleEndian.Uint32(data[18:22]))
	height := int32(binary.LittleEndian.Uint32(data[22:26]))
	bpp := binary.LittleEndian.Uint16(data[28:30])

	fmt.Printf("BMP info: %dx%d, %d bpp, data offset: %d, file size: %d\n",
		width, height, bpp, dataOffset, fileSize)

	// We only support 24-bit and 32-bit BMPs
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("unsupported BMP bit depth: %d (only 24 and 32 bit supported)", bpp)
	}

	// Handle negative height (top-down BMP)
	topDown := height < 0
	if topDown {
		height = -height
	}

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	// Calculate row size (BMP rows are padded to 4-byte boundaries)
	bytesPerPixel := int(bpp) / 8
	rowSize := ((int(width)*bytesPerPixel + 3) / 4) * 4

	// Parse pixel data
	pixelData := data[dataOffset:]

	for y := 0; y < int(height); y++ {
		yPos := y
		if !topDown {
			yPos = int(height) - 1 - y // BMP is normally bottom-up
		}

		rowStart := y * rowSize
		if rowStart >= len(pixelData) {
			return nil, fmt.Errorf("BMP data truncated at row %d", y)
		}

		for x := 0; x < int(width); x++ {
			pos := rowStart + x*bytesPerPixel
			if pos+bytesPerPixel-1 >= len(pixelData) {
				return nil, fmt.Errorf("BMP data truncated at pixel (%d,%d)", x, y)
			}

			// BMP stores in BGR(A) format
			var r, g, b, a uint8
			if bpp == 24 {
				b = pixelData[pos]
				g = pixelData[pos+1]
				r = pixelData[pos+2]
				a = 255 // Full opacity for 24-bit
			} else { // 32-bit
				b = pixelData[pos]
				g = pixelData[pos+1]
				r = pixelData[pos+2]
				a = pixelData[pos+3]
			}

			img.SetRGBA(x, yPos, color.RGBA{r, g, b, a})
		}
	}

	return img, nil
}

// convertImageToCnv converts a TGA or BMP file back to the proprietary CNV format
// Note: BMP is the preferred format; TGA support is maintained for backward compatibility
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
	// TGA is already in bottom-up order, so we use direct mapping
	for y := range height {
		for x := range width {
			r, g, b, a := img.At(x, y).RGBA()

			pos := 4 * (y*width + x)
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
