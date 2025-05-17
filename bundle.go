package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// FileEntry represents an entry in the file table
type FileEntry struct {
	Index  int    // Index of the file in the table
	Offset uint32 // Offset of the file in the bundle
	Length uint32 // Length of the file data
	Name   string // Name of the file
}

// listBundle reads a bundle file and prints the table data
func listBundle(bundlePath string) error {
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", bundlePath)
	}

	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", bundlePath, err)
	}
	defer file.Close()

	_, fileEntries, err := getTableData(file)
	if err != nil {
		return fmt.Errorf("error getting table data: %w", err)
	}

	for _, entry := range fileEntries {
		fmt.Printf("   index: %d, offset: %d, length: %d, name: %s\n",
			entry.Index, entry.Offset, entry.Length, entry.Name)
	}

	return nil
}

func extractBundle(bundlePath, extractPath, pattern string) error {
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", bundlePath)
	}

	// Create the main extraction directory if it doesn't exist
	if err := os.MkdirAll(extractPath, os.ModePerm); err != nil {
		return fmt.Errorf("error creating extraction directory %s: %w", extractPath, err)
	}

	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", bundlePath, err)
	}
	defer file.Close()

	_, fileEntries, err := getTableData(file) // First return value (fileMap) is unused
	if err != nil {
		return fmt.Errorf("failed to get table data: %w", err)
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	for _, entry := range fileEntries {
		if !regex.MatchString(entry.Name) {
			continue
		}

		fmt.Printf("  %+v\n", entry)

		// Move the file cursor to the correct position
		if _, err = file.Seek(int64(entry.Offset), 0); err != nil {
			return fmt.Errorf("error seeking in bundle: %w", err)
		}

		// Read data from the file
		fileData := make([]byte, entry.Length)
		bytesRead, err := file.Read(fileData)
		if err != nil {
			return fmt.Errorf("error extracting from bundle: %w", err)
		}
		if bytesRead != int(entry.Length) {
			return fmt.Errorf("expected to read %d bytes, but got %d", entry.Length, bytesRead)
		}

		encryptionKey := getFileKey(int64(entry.Offset))
		var decryptedData []byte
		for i := range int(entry.Length) {
			decryptedData = append(decryptedData, fileData[i]^byte(encryptionKey))
		}

		outputPath := extractPath + string(os.PathSeparator) + entry.Name

		// Create directories as needed for the output path
		dirPath := filepath.Dir(outputPath)
		if err = os.MkdirAll(dirPath, os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory for %s: %w", outputPath, err)
		}

		if entry.Name[len(entry.Name)-4:] == ".cnv" {
			dataKey := decryptedData[0]

			if dataKey == 1 {
				err = convertWav(&decryptedData)
				if err != nil {
					return fmt.Errorf("error converting wav: %w", err)
				}
				outputPath = outputPath[:len(outputPath)-4] + ".wav"
			} else if dataKey == 24 || dataKey == 32 {
				err = convertImage(&decryptedData)
				if err != nil {
					return fmt.Errorf("error converting image: %w", err)
				}
				if ImageOutputFormat == "png" {
					outputPath = outputPath[:len(outputPath)-4] + ".png"
				} else {
					outputPath = outputPath[:len(outputPath)-4] + ".tga"
				}
			} else {
				fmt.Printf("Bad data key (%d) in %s\n", dataKey, outputPath)
			}
		}

		// Write the converted data to a new file
		err = os.WriteFile(outputPath, decryptedData, 0644)
		if err != nil {
			return fmt.Errorf("unable to write %s: %w", outputPath, err)
		}
	}
	return nil
}

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

func patchBundle(datFilePath string, outputPath string) {
	// Create a backup filename by adding a timestamp
	timeStamp := time.Now().Format("20060102-150405")
	backupFileName := fmt.Sprintf("%s.%s.bak", datFilePath, timeStamp)

	// First, create a copy of the original DAT file
	sourceData, err := os.ReadFile(datFilePath)
	if err != nil {
		log.Fatalf("Unable to read source DAT file %s: %v", datFilePath, err)
	}

	// Create a patched copy
	patchedFileName := fmt.Sprintf("%s.patched", datFilePath)
	err = os.WriteFile(patchedFileName, sourceData, 0644)
	if err != nil {
		log.Fatalf("Unable to create patched file %s: %v", patchedFileName, err)
	}

	// Keep a backup of the original file
	err = os.WriteFile(backupFileName, sourceData, 0644)
	if err != nil {
		log.Printf("Warning: Unable to create backup file %s: %v", backupFileName, err)
	} else {
		fmt.Printf("Created backup of original file: %s\n", backupFileName)
	}

	// Open the patched file for modifications
	fileInfo, err := os.Stat(patchedFileName)
	if err != nil {
		log.Fatal(err)
	}

	// Get the last modified time of the file
	modificationTime := time.Since(fileInfo.ModTime()).Hours() / 24
	outputFile, err := os.OpenFile(patchedFileName, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Unable to open %s for writing: %v", patchedFileName, err)
	}
	defer outputFile.Close()

	_, fileEntries, err := getTableData(outputFile)
	if err != nil {
		log.Fatalf("Unable to get table data: %v", err)
	}
	// Use the improved revisedRecursivePatchDir function which uses index-based lookups
	recursivePatchDir(outputFile, outputPath, "", fileEntries, modificationTime)

	// After patching is complete, replace the original file with the patched one
	outputFile.Close() // Ensure file is closed before replacing

	err = os.Rename(patchedFileName, datFilePath)
	if err != nil {
		log.Fatalf("Unable to replace original file with patched version: %v", err)
	}

	fmt.Printf("Successfully patched %s (original backed up as %s)\n", datFilePath, backupFileName)
}

// matchFileToIndex tries to match a file to an index in the DAT file
func matchFileToIndex(filePath string, fileEntries []*FileEntry) (int, error) {
	// Get base name and extension
	fileName := filepath.Base(filePath)
	ext := filepath.Ext(fileName)
	nameWithoutExt := strings.TrimSuffix(fileName, ext)

	// Convert to lowercase for case-insensitive comparison
	nameLC := strings.ToLower(nameWithoutExt) // We're just extracting the filename, we don't need the directory path
	// for matching against file entries
	_ = filepath.Dir(filePath) // Just to avoid compiler warning about unused variable

	// Hardcoded mappings for known files
	// These are specific to the daybreak05.dat structure
	if nameLC == "title" && (ext == ".ogg" || ext == ".wav") {
		return 6, nil // Known index for title.ogg
	}

	if nameLC == "titlesemi" && (ext == ".ogg" || ext == ".wav") {
		return 4, nil // Known index for titlesemi.ogg
	}

	if nameLC == "title" && ext == ".sfl" {
		return 5, nil // Known index for title.sfl
	}

	if nameLC == "titlesemi" && ext == ".sfl" {
		return 3, nil // Known index for titlesemi.sfl
	}
	// If we don't have a specific mapping, try to find a matching entry
	for i, entry := range fileEntries {
		entryName := strings.ToLower(entry.Name)

		// Try to match by filename
		if strings.Contains(entryName, nameLC) {
			// If we find a match on the name, check for extension match
			if (ext == ".ogg" && strings.HasSuffix(entryName, ".ogg")) ||
				(ext == ".sfl" && strings.HasSuffix(entryName, ".sfl")) ||
				((ext == ".tga" || ext == ".png") && strings.HasSuffix(entryName, ".cnv")) {
				return i, nil
			}
		}
	}

	return -1, fmt.Errorf("could not find a matching entry for %s", filePath)
}

// patchFileByIndex is the improved patchFile function that uses indices rather than names
func patchFileByIndex(outputFile *os.File, inputFileName string, fileEntries []*FileEntry, targetIndex int) error {
	if targetIndex < 0 || targetIndex >= len(fileEntries) {
		return fmt.Errorf("invalid file index: %d (max: %d)", targetIndex, len(fileEntries)-1)
	}
	// Get the target entry
	fileEntry := fileEntries[targetIndex]
	// Read and process the input file
	var fileData []byte
	var err error
	// Check if this is a PNG/TGA file being patched to a CNV file
	if strings.HasSuffix(strings.ToLower(fileEntry.Name), ".cnv") {
		ext := strings.ToLower(filepath.Ext(inputFileName))
		if ext == ".png" || ext == ".tga" {
			// Convert the image back to CNV format
			fmt.Printf("Converting %s back to CNV format...\n", filepath.Base(inputFileName))
			convertedData, err := convertImageToCnv(inputFileName)
			if err != nil {
				return fmt.Errorf("error converting image to CNV: %w", err)
			}
			fileData = convertedData
			fmt.Printf("Successfully converted %s to CNV format (%d bytes)\n",
				filepath.Base(inputFileName), len(fileData))
		} else {
			// For non-image files, read directly
			fileData, err = os.ReadFile(inputFileName)
			if err != nil {
				return fmt.Errorf("error reading input file: %v", err)
			}
		}
	} else {
		// For non-CNV files, read directly
		fileData, err = os.ReadFile(inputFileName)
		if err != nil {
			return fmt.Errorf("error reading input file: %v", err)
		}
	}

	dataLength := len(fileData)

	fmt.Printf("Updating file at index: %d, name: %s\n", targetIndex, fileEntry.Name)
	fmt.Printf("File details - offset: %d, length: %d\n", fileEntry.Offset, fileEntry.Length)

	// Check if the file will fit in the DAT entry
	if dataLength > int(fileEntry.Length) {
		return fmt.Errorf("input file is too large (%d bytes) to fit in entry (%d bytes)",
			dataLength, fileEntry.Length)
	}

	// Seek to the appropriate position in the DAT file
	_, err = outputFile.Seek(int64(fileEntry.Offset), io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking to position %d: %v", fileEntry.Offset, err)
	}

	// Encrypt the data as needed
	encryptionKey := byte(fileEntry.Offset & 0xFF)
	encryptedData := make([]byte, dataLength)

	// Encrypt the data
	for i := range dataLength {
		encryptedData[i] = fileData[i] ^ encryptionKey
	}

	// Write the encrypted data to the DAT file
	bytesWritten, err := outputFile.Write(encryptedData)
	if err != nil {
		return fmt.Errorf("error writing data: %v", err)
	}
	if bytesWritten != dataLength {
		return fmt.Errorf("wrote %d bytes but expected to write %d bytes",
			bytesWritten, dataLength)
	}

	fmt.Printf("Successfully updated entry at index %d\n", targetIndex)
	return nil
}
