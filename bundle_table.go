package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	decryptedData := decryptFileTableBlock(0, buffer)

	// Tables to be returned
	fileEntries := make([]*FileEntry, 0, numFiles)
	fileEntryMap := make(map[string]*FileEntry, numFiles)

	// Loop to process each file
	for i := range int(numFiles) {
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

// recursivePatchDir processes directories recursively for patching operations
// index-based lookups are faster than name-based lookups
func recursivePatchDir(outputFile *os.File, dirPath string, relPath string, fileEntries []*FileEntry, modificationTime float64) {
	// Open the directory
	dir, err := os.Open(dirPath)
	if err != nil {
		fmt.Printf("Unable to open %s: %v\n", dirPath, err)
		return
	}
	defer dir.Close()

	// Get all files in the directory
	fileInfos, err := dir.Readdir(0)
	if err != nil {
		fmt.Printf("Error reading directory %s: %v\n", dirPath, err)
		return
	}

	// Process each file
	for _, fileInfo := range fileInfos {
		fullPath := filepath.Join(dirPath, fileInfo.Name())
		localPath := filepath.Join(relPath, fileInfo.Name())

		// If it's a directory, recursively process it
		if fileInfo.IsDir() {
			recursivePatchDir(outputFile, fullPath, localPath, fileEntries, modificationTime)
			continue
		}

		// Check if the file is recent enough to be patched
		hoursSinceModified := float64(0)
		if modificationTime > 0 {
			hoursSinceModified = float64(fileInfo.ModTime().Hour()) / 24
		}

		if hoursSinceModified <= modificationTime {
			// Find matching index for this file
			index, err := matchFileToIndex(fullPath, fileEntries)
			if err != nil {
				fmt.Printf("Skipping %s: %v\n", fullPath, err)
				continue
			}

			// Update the file using the index
			err = patchFileByIndex(outputFile, fullPath, fileEntries, index)
			if err != nil {
				fmt.Printf("Error patching %s: %v\n", fullPath, err)
			} else {
				fmt.Printf("Successfully patched %s (index: %d)\n", fullPath, index)
			}
		}
	}
}
