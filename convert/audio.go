package convert

import (
	"encoding/binary"
	"errors"
	"fmt"
)

func ConvertWAV(data *[]byte) error {
	const headerSize = 22
	if len(*data) < headerSize {
		return errors.New("data is too short to read WAV header")
	}

	// Unpack custom header
	audioFmt := binary.LittleEndian.Uint16((*data)[0:2])
	nChannels := binary.LittleEndian.Uint16((*data)[2:4])
	sampleRate := binary.LittleEndian.Uint32((*data)[4:8])
	byteRate := binary.LittleEndian.Uint32((*data)[8:12])
	blockAlign := binary.LittleEndian.Uint16((*data)[12:14])
	bitsPerSample := binary.LittleEndian.Uint16((*data)[14:16])
	subchunk2Size := binary.LittleEndian.Uint32((*data)[16:20])

	// Check subchunk size
	if subchunk2Size != uint32(len(*data))-headerSize {
		fmt.Printf(" *** Warning: Size mismatch: %d vs %d.\n", subchunk2Size, uint32(len(*data))-headerSize)
	}

	// Check byte rate
	if byteRate != sampleRate*uint32(nChannels)*(uint32(bitsPerSample)/8) {
		return fmt.Errorf("byte rate mismatch: %d vs %d", byteRate, sampleRate*uint32(nChannels)*(uint32(bitsPerSample)/8))
	}

	// Check block align
	if blockAlign != nChannels*(bitsPerSample/8) {
		return fmt.Errorf("block align mismatch: %d vs %d", blockAlign, nChannels*(bitsPerSample/8))
	}

	outData := make([]byte, 44)

	// RIFF header
	copy(outData[0:], "RIFF")
	binary.LittleEndian.PutUint32(outData[4:], subchunk2Size+36)
	copy(outData[8:], "WAVE")

	// fmt subchunk
	copy(outData[12:], "fmt ")
	binary.LittleEndian.PutUint32(outData[16:], 16) // Subchunk1Size para PCM
	binary.LittleEndian.PutUint16(outData[20:], audioFmt)
	binary.LittleEndian.PutUint16(outData[22:], nChannels)
	binary.LittleEndian.PutUint32(outData[24:], sampleRate)
	binary.LittleEndian.PutUint32(outData[28:], byteRate)
	binary.LittleEndian.PutUint16(outData[32:], blockAlign)
	binary.LittleEndian.PutUint16(outData[34:], bitsPerSample)

	// data subchunk
	copy(outData[36:], "data")
	binary.LittleEndian.PutUint32(outData[40:], subchunk2Size)

	// Copia os bytes de áudio crus sem conversão
	outData = append(outData, (*data)[headerSize:]...)

	*data = outData
	return nil
}
