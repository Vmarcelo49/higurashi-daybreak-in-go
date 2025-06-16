package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
				// Add panic recovery for WAV conversion
				func() {
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("WAV conversion panicked for %s: %v\n", entry.Name, r)
							err = fmt.Errorf("WAV conversion panicked: %v", r)
						}
					}()
					err = convertWav(&decryptedData)
				}()

				if err != nil {
					fmt.Printf("Error converting WAV for %s: %v, saving as .unknown\n", entry.Name, err)
					outputPath = outputPath[:len(outputPath)-4] + ".unknown"
				} else {
					outputPath = outputPath[:len(outputPath)-4] + ".wav"
				}
			} else if dataKey == 24 || dataKey == 32 {
				err = convertImage(&decryptedData)
				if err != nil {
					return fmt.Errorf("error converting image: %w", err)
				}
				outputPath = outputPath[:len(outputPath)-4] + ".bmp"
			} else {
				fmt.Printf("Bad data key (%d) in %s, saving as .unknown\n", dataKey, outputPath)
				outputPath = outputPath[:len(outputPath)-4] + ".unknown"
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
		entryName := strings.ToLower(entry.Name) // Try to match by filename
		if strings.Contains(entryName, nameLC) { // If we find a match on the name, check for extension match
			if (ext == ".ogg" && strings.HasSuffix(entryName, ".ogg")) ||
				(ext == ".sfl" && strings.HasSuffix(entryName, ".sfl")) ||
				(ext == ".bmp" && strings.HasSuffix(entryName, ".cnv")) {
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
	fileEntry := fileEntries[targetIndex] // Read and process the input file
	var fileData []byte
	var err error

	// Check if this is a BMP file being patched to a CNV file
	if strings.HasSuffix(strings.ToLower(fileEntry.Name), ".cnv") {
		ext := strings.ToLower(filepath.Ext(inputFileName))
		if ext == ".bmp" {
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

// extractSingleFile extracts a single file from the bundle to a specified path
func extractSingleFile(bundlePath string, fileIndex int, outputPath string) error {
	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", bundlePath, err)
	}
	defer file.Close()

	_, fileEntries, err := getTableData(file)
	if err != nil {
		return fmt.Errorf("failed to get table data: %w", err)
	}

	if fileIndex < 0 || fileIndex >= len(fileEntries) {
		return fmt.Errorf("invalid file index %d", fileIndex)
	}

	entry := fileEntries[fileIndex]

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

	// Handle conversion based on file type
	finalOutputPath := outputPath
	if entry.Name[len(entry.Name)-4:] == ".cnv" {
		dataKey := decryptedData[0]

		if dataKey == 1 {
			// WAV conversion with panic recovery
			err = func() (convertErr error) {
				defer func() {
					if r := recover(); r != nil {
						convertErr = fmt.Errorf("WAV conversion panicked: %v", r)
					}
				}()
				return convertWav(&decryptedData)
			}()

			if err != nil {
				// Change extension to .unknown for failed WAV conversion
				if strings.HasSuffix(finalOutputPath, ".cnv") {
					finalOutputPath = finalOutputPath[:len(finalOutputPath)-4] + ".unknown"
				}
			} else {
				// Change extension to .wav for successful conversion
				if strings.HasSuffix(finalOutputPath, ".cnv") {
					finalOutputPath = finalOutputPath[:len(finalOutputPath)-4] + ".wav"
				}
			}
		} else if dataKey == 24 || dataKey == 32 {
			err = convertImage(&decryptedData)
			if err != nil {
				return fmt.Errorf("error converting image: %w", err)
			}
			// Change extension to .bmp for image conversion
			if strings.HasSuffix(finalOutputPath, ".cnv") {
				finalOutputPath = finalOutputPath[:len(finalOutputPath)-4] + ".bmp"
			}
		} else {
			// Unknown data key, save as .unknown
			if strings.HasSuffix(finalOutputPath, ".cnv") {
				finalOutputPath = finalOutputPath[:len(finalOutputPath)-4] + ".unknown"
			}
		}
	}

	// Create directories as needed for the output path
	dirPath := filepath.Dir(finalOutputPath)
	if err = os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("error creating directory for %s: %w", finalOutputPath, err)
	}

	// Write the converted data to a new file
	err = os.WriteFile(finalOutputPath, decryptedData, 0644)
	if err != nil {
		return fmt.Errorf("unable to write %s: %w", finalOutputPath, err)
	}

	return nil
}
