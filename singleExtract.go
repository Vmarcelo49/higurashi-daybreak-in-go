package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// patchSingleFile patches a single file in the DAT file and updates all file table entries accordingly
func patchSingleFile(datFilePath string, inputFilePath string, targetIndex int) error {
	// Create a backup filename by adding a timestamp
	timeStamp := time.Now().Format("20060102-150405")
	backupFileName := fmt.Sprintf("%s.%s.bak", datFilePath, timeStamp)

	// First, create a copy of the original DAT file
	sourceData, err := os.ReadFile(datFilePath)
	if err != nil {
		return fmt.Errorf("unable to read source DAT file %s: %v", datFilePath, err)
	}

	// Keep a backup of the original file
	err = os.WriteFile(backupFileName, sourceData, 0644)
	if err != nil {
		log.Printf("Warning: Unable to create backup file %s: %v", backupFileName, err)
	} else {
		fmt.Printf("Created backup of original file: %s\n", backupFileName)
	}

	// Open the source DAT file for reading table data
	sourceFile, err := os.Open(datFilePath)
	if err != nil {
		return fmt.Errorf("unable to open source DAT file %s: %v", datFilePath, err)
	}
	defer sourceFile.Close()

	// Get the table data from the file
	_, fileEntries, err := getTableData(sourceFile)
	if err != nil {
		return fmt.Errorf("error reading file table: %v", err)
	}

	// Validate target index
	if targetIndex < 0 || targetIndex >= len(fileEntries) {
		return fmt.Errorf("invalid file index: %d (valid range: 0-%d)", targetIndex, len(fileEntries)-1)
	} // Read the new file data
	var newFileData []byte

	// Check if this is an image file that needs conversion to CNV format (BMP only)
	if strings.HasSuffix(strings.ToLower(fileEntries[targetIndex].Name), ".cnv") {
		ext := strings.ToLower(filepath.Ext(inputFilePath))
		if ext == ".bmp" {
			// Convert the image back to CNV format
			fmt.Printf("Converting %s back to CNV format...\n", filepath.Base(inputFilePath))
			convertedData, err := convertImageToCnv(inputFilePath)
			if err != nil {
				return fmt.Errorf("error converting image to CNV: %w", err)
			}
			newFileData = convertedData
			fmt.Printf("Successfully converted %s to CNV format (%d bytes)\n",
				filepath.Base(inputFilePath), len(newFileData))
		} else {
			// For non-image files, read directly
			newFileData, err = os.ReadFile(inputFilePath)
			if err != nil {
				return fmt.Errorf("error reading input file %s: %v", inputFilePath, err)
			}
		}
	} else {
		// For non-CNV files, read directly
		newFileData, err = os.ReadFile(inputFilePath)
		if err != nil {
			return fmt.Errorf("error reading input file %s: %v", inputFilePath, err)
		}
	}

	// Create a temporary file for the patched version
	patchedFileName := fmt.Sprintf("%s.patched", datFilePath)
	patchedFile, err := os.Create(patchedFileName)
	if err != nil {
		return fmt.Errorf("unable to create patched file %s: %v", patchedFileName, err)
	}
	defer patchedFile.Close()

	// Calculate the size difference between the new file and the original
	targetEntry := fileEntries[targetIndex]
	sizeDiff := len(newFileData) - int(targetEntry.Length)

	fmt.Printf("Original file size: %d bytes\n", targetEntry.Length)
	fmt.Printf("New file size: %d bytes\n", len(newFileData))
	fmt.Printf("Size difference: %d bytes\n", sizeDiff)

	// Update offsets for all entries after the target file
	for i := targetIndex + 1; i < len(fileEntries); i++ {
		fileEntries[i].Offset = uint32(int(fileEntries[i].Offset) + sizeDiff)
	}

	// Update the target entry's length
	fileEntries[targetIndex].Length = uint32(len(newFileData))

	// Write the updated file table to the patched file
	err = writeUpdatedFileTable(patchedFile, fileEntries)
	if err != nil {
		return fmt.Errorf("error writing updated file table: %v", err)
	}

	// Calculate the table size to know where file data starts
	tableSize := 2 + (268 * len(fileEntries)) // 2 bytes for count + entries

	// Seek back to the beginning of the source file
	_, err = sourceFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking in source file: %v", err)
	}

	// Skip the table in the source file
	_, err = sourceFile.Seek(int64(tableSize), io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking past table in source file: %v", err)
	}

	// Seek to the end of the table in the patched file
	_, err = patchedFile.Seek(int64(tableSize), io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking in patched file: %v", err)
	}

	// Copy files one by one, inserting our modified file at the right position
	for i, entry := range fileEntries {
		// If this is the target file, write the new data instead
		if i == targetIndex {
			// Encrypt the new file data
			encryptionKey := getFileKey(int64(entry.Offset))
			encryptedData := make([]byte, len(newFileData))
			for j := 0; j < len(newFileData); j++ {
				encryptedData[j] = newFileData[j] ^ byte(encryptionKey)
			}

			// Write the encrypted data to the patched file
			_, err = patchedFile.Write(encryptedData)
			if err != nil {
				return fmt.Errorf("error writing patched file data: %v", err)
			}

			// Skip the original file in the source
			_, err = sourceFile.Seek(int64(targetEntry.Length), io.SeekCurrent)
			if err != nil {
				return fmt.Errorf("error seeking in source file: %v", err)
			}
		} else {
			// For other files, just copy them from source to destination
			buffer := make([]byte, entry.Length)
			bytesRead, err := sourceFile.Read(buffer)
			if err != nil {
				return fmt.Errorf("error reading file data at index %d: %v", i, err)
			}
			if bytesRead != int(entry.Length) {
				return fmt.Errorf("read %d bytes but expected %d at index %d", bytesRead, entry.Length, i)
			}

			_, err = patchedFile.Write(buffer)
			if err != nil {
				return fmt.Errorf("error writing file data at index %d: %v", i, err)
			}
		}
	}

	// Close both files to ensure all data is written and flushed
	sourceFile.Close()
	patchedFile.Close()

	// Replace the original file with the patched one
	err = os.Rename(patchedFileName, datFilePath)
	if err != nil {
		return fmt.Errorf("error replacing original file with patched version: %v", err)
	}

	fmt.Printf("Successfully patched file at index %d (%s) in %s\n",
		targetIndex, fileEntries[targetIndex].Name, datFilePath)
	return nil
}

// writeUpdatedFileTable writes the updated file table to the output file
func writeUpdatedFileTable(outputFile *os.File, fileEntries []*FileEntry) error {
	// Write the number of files (2 bytes, little endian)
	numFilesBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(numFilesBytes, uint16(len(fileEntries)))
	_, err := outputFile.Write(numFilesBytes)
	if err != nil {
		return fmt.Errorf("error writing file count: %v", err)
	}

	// Create a buffer for the unencrypted table data
	tableData := make([]byte, 268*len(fileEntries))

	// Fill the table data
	for i, entry := range fileEntries {
		// Convert filename to Shift JIS
		var shiftJISFilename []byte
		var filenameBuffer strings.Builder
		writer := transform.NewWriter(&filenameBuffer, japanese.ShiftJIS.NewEncoder())
		_, err := writer.Write([]byte(entry.Name))
		if err != nil {
			return fmt.Errorf("error encoding filename to Shift JIS: %v", err)
		}
		writer.Close()
		shiftJISFilename = []byte(filenameBuffer.String())

		// Ensure name doesn't exceed fixed size
		if len(shiftJISFilename) > 260 {
			return fmt.Errorf("filename too long: %s (max 260 bytes in Shift JIS)", entry.Name)
		}

		// Copy name to table (fixed 260 bytes)
		entryOffset := i * 268
		copy(tableData[entryOffset:entryOffset+260], shiftJISFilename)

		// Fill remaining name space with zeros
		for j := len(shiftJISFilename); j < 260; j++ {
			tableData[entryOffset+j] = 0
		}

		// Write length (4 bytes)
		binary.LittleEndian.PutUint32(tableData[entryOffset+260:entryOffset+264], entry.Length)

		// Write offset (4 bytes)
		binary.LittleEndian.PutUint32(tableData[entryOffset+264:entryOffset+268], entry.Offset)
	}

	// Encrypt the table data and write it
	encryptedTableData := encryptFileTableBlock(0, tableData)
	_, err = outputFile.Write(encryptedTableData)
	if err != nil {
		return fmt.Errorf("error writing file table: %v", err)
	}
	return nil
}
