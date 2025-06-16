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

func detectImageFormat(filePath string, data []byte) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	if ext != ".bmp" {
		return nil, fmt.Errorf("unsupported image format: %s (only BMP is supported)", ext)
	}

	// Decode BMP using our custom decoder
	return decodeBMP(data)
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

// convertImageToCnv converts a BMP file back to the proprietary CNV format
func convertImageToCnv(filePath string) ([]byte, error) {
	// Read the image file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading image file: %w", err)
	}

	// Verify this is a BMP file
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".bmp" {
		return nil, fmt.Errorf("only BMP files are supported, got: %s", ext)
	}

	fmt.Printf("Converting BMP image to CNV format\n")

	// Decode the BMP image
	img, err := detectImageFormat(filePath, fileData)
	if err != nil {
		return nil, fmt.Errorf("error decoding BMP image: %w", err)
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
	// Last 4 bytes are reserved (already zero-initialized)
	// Convert pixel data to CNV format (BGRA, top-down order)
	pixelData := make([]byte, width*height*4)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			// Calculate position in CNV data (top-down, left-to-right)
			pos := 4 * (y*width + x)
			if pos+3 >= len(pixelData) {
				return nil, fmt.Errorf("pixel data buffer overflow at position (%d,%d)", x, y)
			}

			// CNV format stores pixels as BGRA (Blue, Green, Red, Alpha)
			pixelData[pos] = byte(b >> 8)   // Blue
			pixelData[pos+1] = byte(g >> 8) // Green
			pixelData[pos+2] = byte(r >> 8) // Red
			pixelData[pos+3] = byte(a >> 8) // Alpha
		}
	}

	// Combine header and pixel data
	cnvData = append(cnvData, pixelData...)

	fmt.Printf("Successfully converted image: %dx%d pixels, %d bytes\n", width, height, len(cnvData))
	return cnvData, nil
}
