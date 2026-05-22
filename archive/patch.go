package archive

import (
	"bundleTools/fileutil"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func PatchBundle(datFilePath string, outputPath string) {
	// Prepare patched file and backup using helper
	patchedFileName, backupFileName, err := preparePatchFile(datFilePath)
	if err != nil {
		log.Fatalf("unable to prepare patched file: %v", err)
	}
	if backupFileName != "" {
		fmt.Printf("Created backup of original file: %s\n", backupFileName)
	}

	outputFile, err := os.OpenFile(patchedFileName, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Unable to open %s for writing: %v", patchedFileName, err)
	}
	defer outputFile.Close()

	_, fileEntries, err := getTableData(outputFile)
	if err != nil {
		log.Fatalf("Unable to get table data: %v", err)
	}

	// Apply patches from the provided source tree.
	recursivePatchDir(outputFile, outputPath, "", fileEntries)

	// After patching is complete, replace the original file with the patched one
	outputFile.Close() // Ensure file is closed before replacing

	err = os.Rename(patchedFileName, datFilePath)
	if err != nil {
		log.Fatalf("Unable to replace original file with patched version: %v", err)
	}

	fmt.Printf("Successfully patched %s (original backed up as %s)\n", datFilePath, backupFileName)
}

func patchFileByIndex(outputFile *os.File, inputFileName string, fileEntries []*FileEntry, targetIndex int) error {
	if targetIndex < 0 || targetIndex >= len(fileEntries) {
		return fmt.Errorf("invalid file index: %d (max: %d)", targetIndex, len(fileEntries)-1)
	}
	// Get the target entry
	fileEntry := fileEntries[targetIndex]

	// Read/convert input as needed
	fileData, err := readInputPossiblyConvertToCNV(inputFileName, fileutil.HasExtCI(fileEntry.Name, ".cnv"))
	if err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	dataLength := len(fileData)

	fmt.Printf("Updating file at index: %d, name: %s\n", targetIndex, fileEntry.Name)
	fmt.Printf("File details - offset: %d, length: %d\n", fileEntry.Offset, fileEntry.Length)

	// Check if the file will fit in the DAT entry
	if dataLength > int(fileEntry.Length) {
		return fmt.Errorf("input file is too large (%d bytes) to fit in entry (%d bytes)", dataLength, fileEntry.Length)
	}

	// Seek to the appropriate position in the DAT file
	if _, err = outputFile.Seek(int64(fileEntry.Offset), io.SeekStart); err != nil {
		return fmt.Errorf("error seeking to position %d: %v", fileEntry.Offset, err)
	}

	// Encrypt and write
	encryptedData := encryptBytesWithOffset(fileData, fileEntry.Offset)
	bytesWritten, err := outputFile.Write(encryptedData)
	if err != nil {
		return fmt.Errorf("error writing data: %v", err)
	}
	if bytesWritten != dataLength {
		return fmt.Errorf("wrote %d bytes but expected to write %d bytes", bytesWritten, dataLength)
	}

	fmt.Printf("Successfully updated entry at index %d\n", targetIndex)
	return nil
}

// recursivePatchDir processes directories recursively for patching operations
// index-based lookups are faster than name-based lookups
func recursivePatchDir(outputFile *os.File, dirPath string, relPath string, fileEntries []*FileEntry) {
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
			recursivePatchDir(outputFile, fullPath, localPath, fileEntries)
			continue
		}

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
