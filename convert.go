package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// ImageOutputFormat controls the output format for converted images
// Possible values: "tga", "png"
var ImageOutputFormat = "tga"

// convertToPng converts the image data to PNG format
func convertToPng(width, height uint32, pixelData []byte) ([]byte, error) {
	// Create a new RGBA image
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	// Copy pixel data to the image
	// Note: Assuming the pixel data is in BGRA format (common in TGA)
	for y := range int(height) {
		for x := range int(width) {
			// Get the pixel index in the byte array
			idx := (y*int(width) + x) * 4
			if idx+3 >= len(pixelData) {
				return nil, errors.New("pixel data is corrupted or incomplete")
			}

			// Read BGRA values
			b := pixelData[idx]
			g := pixelData[idx+1]
			r := pixelData[idx+2]
			a := pixelData[idx+3]

			// Set pixel in the image (in RGBA order)
			img.SetRGBA(x, y, color.RGBA{r, g, b, a})
		}
	}

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	fmt.Printf("Successfully converted to PNG: %dx%d pixels\n", width, height)
	return buf.Bytes(), nil
}

func convertImage(data *[]byte) error {
	const headerSize = 17
	if len(*data) < headerSize {
		return errors.New("data is too short to read image header")
	}

	// Unpack image header
	bpp := (*data)[0]
	width := binary.LittleEndian.Uint32((*data)[1:5])
	height := binary.LittleEndian.Uint32((*data)[5:9])
	width2 := binary.LittleEndian.Uint32((*data)[9:13])
	zero := (*data)[headerSize-1]

	// Check width consistency
	if width != width2 {
		fmt.Printf(" *** Warning ----: Two width values disagree: %d %d\n", width, width2)
	}

	// Check bits per pixel
	if bpp != 24 && bpp != 32 {
		return fmt.Errorf("BPP must be 24 or 32, not %d", bpp)
	}

	// Check data length consistency
	if int(width2)*int(height)*4+headerSize != len(*data) {
		return fmt.Errorf("data lengths disagree: %d vs %d", int(width2)*int(height)*4+headerSize, len(*data))
	}

	if zero != 0 {
		return errors.New("nonzero value in final header block")
	} // Extract pixel data before we prepare the output data
	pixelData := make([]byte, int(width)*int(height)*4)
	pixelIndex := 0

	// Read rows based on output format
	// For TGA: Reverse row order (bottom-up)
	// For PNG: Keep normal row order (top-down)
	if ImageOutputFormat == "tga" {
		// Reverse row order for TGA (bottom-up)
		for r := int(height) - 1; r >= 0; r-- {
			for c := range int(width) {
				start := headerSize + 4*(int(r)*int(width)+c)
				if start+4 > len(*data) {
					return errors.New("data overflow while reading pixels")
				}

				pixelData[pixelIndex] = (*data)[start]
				pixelData[pixelIndex+1] = (*data)[start+1]
				pixelData[pixelIndex+2] = (*data)[start+2]
				pixelData[pixelIndex+3] = (*data)[start+3]
				pixelIndex += 4
			}
		}
	} else {
		// Normal row order for PNG (top-down)
		for r := range int(height) {
			for c := range int(width) {
				start := headerSize + 4*(int(r)*int(width)+c)
				if start+4 > len(*data) {
					return errors.New("data overflow while reading pixels")
				}

				pixelData[pixelIndex] = (*data)[start]
				pixelData[pixelIndex+1] = (*data)[start+1]
				pixelData[pixelIndex+2] = (*data)[start+2]
				pixelData[pixelIndex+3] = (*data)[start+3]
				pixelIndex += 4
			}
		}
	}

	// Output based on selected format
	var outData []byte
	var err error

	if ImageOutputFormat == "png" {
		// Convert to PNG
		outData, err = convertToPng(width, height, pixelData)
		if err != nil {
			return fmt.Errorf("PNG conversion failed: %w", err)
		}
	} else {
		// Default: Convert to TGA
		outData = make([]byte, 0, 18+len(pixelData))
		outData = append(outData, 0, 0, 2)    // Header
		outData = append(outData, 0, 0, 0, 0) // No image ID
		outData = append(outData, 0, 0)       // No color map
		outData = append(outData, byte(width), byte(width>>8), byte(height), byte(height>>8))
		outData = append(outData, 32, 0x08) // Bits per pixel and image descriptor
		outData = append(outData, pixelData...)
	}

	*data = outData
	return nil
}

func convertWav(data *[]byte) error {
	const headerSize = 22
	if len(*data) < headerSize {
		return errors.New("data is too short to read WAV header")
	}

	// Unpack WAV header
	audioFmt := binary.LittleEndian.Uint16((*data)[0:2])
	nChannels := binary.LittleEndian.Uint16((*data)[2:4])
	sampleRate := binary.LittleEndian.Uint32((*data)[4:8])
	byteRate := binary.LittleEndian.Uint32((*data)[8:12])
	blockAlign := binary.LittleEndian.Uint16((*data)[12:14])
	bitsPerSample := binary.LittleEndian.Uint16((*data)[14:16])
	subchunk2Size := binary.LittleEndian.Uint32((*data)[16:20])

	// Check subchunk size
	if subchunk2Size != uint32(len(*data))-headerSize {
		fmt.Printf(" *** Warning ----: Size mismatch: %d vs %d.\n", subchunk2Size, len(*data)-headerSize)
	}

	// Check byte rate
	if byteRate != (sampleRate * uint32(nChannels) * (uint32(bitsPerSample) / 8)) {
		return fmt.Errorf("byte rate mismatch: %d vs %d", byteRate, (sampleRate * uint32(nChannels) * (uint32(bitsPerSample) / 8)))
	}

	// Check block align
	if blockAlign != nChannels*(bitsPerSample/8) {
		return fmt.Errorf("block align mismatch: %d vs %d", blockAlign, nChannels*(bitsPerSample/8))
	}

	// Pack new WAV data
	outData := make([]byte, 0)

	// RIFF header
	outData = append(outData, []byte("RIFF")...)
	outData = append(outData, make([]byte, 4)...)
	binary.LittleEndian.PutUint32(outData[4:], subchunk2Size+36)
	outData = append(outData, []byte("WAVE")...)

	// fmt subchunk
	outData = append(outData, []byte("fmt ")...)
	outData = append(outData, 16, 0, 0, 0) // Subchunk size
	binary.LittleEndian.PutUint16(outData[16:], audioFmt)
	binary.LittleEndian.PutUint16(outData[18:], nChannels)
	binary.LittleEndian.PutUint32(outData[20:], sampleRate)
	binary.LittleEndian.PutUint32(outData[24:], byteRate)
	binary.LittleEndian.PutUint16(outData[28:], blockAlign)
	binary.LittleEndian.PutUint16(outData[30:], bitsPerSample)

	// Data subchunk
	outData = append(outData, []byte("data")...)
	binary.LittleEndian.PutUint32(outData[36:], subchunk2Size)
	outData = append(outData, (*data)[headerSize:]...)

	*data = outData
	return nil
}
