package main

import (
	"encoding/binary"
	"errors"
	"fmt"
)

func convertImage(data *[]byte) error {
	return convertImageToBMP(data)
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

// convertImageToBMP converts CNV image data to BMP format
func convertImageToBMP(data *[]byte) error {
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
	}

	// Calculate BMP parameters
	rowSize := ((int(width)*4 + 3) / 4) * 4 // BMP rows must be padded to 4-byte boundaries
	imageSize := rowSize * int(height)
	fileSize := 54 + imageSize // 54 bytes for BMP header + image data

	// Create BMP header (54 bytes total)
	bmpData := make([]byte, fileSize)

	// BMP File Header (14 bytes)
	bmpData[0] = 'B'                                              // Signature
	bmpData[1] = 'M'                                              // Signature
	binary.LittleEndian.PutUint32(bmpData[2:6], uint32(fileSize)) // File size
	binary.LittleEndian.PutUint32(bmpData[6:10], 0)               // Reserved
	binary.LittleEndian.PutUint32(bmpData[10:14], 54)             // Data offset

	// BMP Info Header (40 bytes)
	binary.LittleEndian.PutUint32(bmpData[14:18], 40)                // Info header size
	binary.LittleEndian.PutUint32(bmpData[18:22], uint32(width))     // Width
	binary.LittleEndian.PutUint32(bmpData[22:26], uint32(height))    // Height (positive = bottom-up)
	binary.LittleEndian.PutUint16(bmpData[26:28], 1)                 // Planes
	binary.LittleEndian.PutUint16(bmpData[28:30], 32)                // Bits per pixel (always 32 for BGRA)
	binary.LittleEndian.PutUint32(bmpData[30:34], 0)                 // Compression (0 = none)
	binary.LittleEndian.PutUint32(bmpData[34:38], uint32(imageSize)) // Image size
	binary.LittleEndian.PutUint32(bmpData[38:42], 2835)              // X pixels per meter
	binary.LittleEndian.PutUint32(bmpData[42:46], 2835)              // Y pixels per meter
	binary.LittleEndian.PutUint32(bmpData[46:50], 0)                 // Colors used
	binary.LittleEndian.PutUint32(bmpData[50:54], 0)                 // Important colors

	// Convert pixel data (CNV is BGRA, BMP expects BGRA but bottom-up)
	pixelIndex := 54
	for r := int(height) - 1; r >= 0; r-- { // BMP is bottom-up
		for c := 0; c < int(width); c++ {
			srcPos := headerSize + 4*(r*int(width)+c)
			if srcPos+4 > len(*data) {
				return errors.New("data overflow while reading pixels")
			}

			// Copy BGRA directly (CNV and BMP both use BGRA)
			bmpData[pixelIndex] = (*data)[srcPos]     // B
			bmpData[pixelIndex+1] = (*data)[srcPos+1] // G
			bmpData[pixelIndex+2] = (*data)[srcPos+2] // R
			bmpData[pixelIndex+3] = (*data)[srcPos+3] // A
			pixelIndex += 4
		}

		// Add padding to reach 4-byte boundary
		for pad := int(width) * 4; pad < rowSize; pad++ {
			if pixelIndex < len(bmpData) {
				bmpData[pixelIndex] = 0
				pixelIndex++
			}
		}
	}
	*data = bmpData
	return nil
}
