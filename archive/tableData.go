package archive

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"bundleTools/crypto"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// getTableData reads the table data from the file and returns a map of the table and a slice of the table
func getTableData(inputFile *os.File) (map[string]*FileEntry, []*FileEntry, error) {
	// Move to the beginning of the file
	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("error seeking to start of file: %v", err)
	}

	buffer := make([]byte, 2)
	bytesRead, err := inputFile.Read(buffer)
	if err != nil || bytesRead != len(buffer) {
		return nil, nil, fmt.Errorf("error reading table length: %v", err)
	}
	numFiles := binary.LittleEndian.Uint16(buffer)

	// Read the file table
	buffer = make([]byte, 268*int(numFiles))
	bytesRead, err = inputFile.Read(buffer)
	if err != nil || bytesRead != len(buffer) {
		return nil, nil, fmt.Errorf("error reading table: %v", err)
	}

	decryptedData := crypto.DecryptFileTableBlock(0, buffer)

	// Tables to be returned
	fileEntries := make([]*FileEntry, 0, numFiles)
	fileEntryMap := make(map[string]*FileEntry, numFiles)

	// Loop to process each file
	for i := 0; i < int(numFiles); i++ {
		entry := decryptedData[i*268 : (i+1)*268]
		filename := string(entry[:260])
		length := binary.LittleEndian.Uint32(entry[260:264])
		offset := binary.LittleEndian.Uint32(entry[264:268])

		// Remove null bytes from the filename
		filename = strings.TrimRight(filename, "\x00")

		// Decode shift_jis
		decodedFilenameReader := transform.NewReader(strings.NewReader(filename), japanese.ShiftJIS.NewDecoder())
		decodedFilenameData, err := io.ReadAll(decodedFilenameReader)
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding shift_jis: %v", err)
		}

		// Create the table entry
		decodedFilename := string(decodedFilenameData)
		fileEntry := &FileEntry{
			Index:  i,
			Offset: offset,
			Length: length,
			Name:   decodedFilename,
		}

		fileEntries = append(fileEntries, fileEntry)
		fileEntryMap[decodedFilename] = fileEntry
	}

	return fileEntryMap, fileEntries, nil
}
