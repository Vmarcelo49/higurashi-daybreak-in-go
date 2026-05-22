package archive

import (
	"bundleTools/fileutil"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileEntry represents an entry in the file table
type FileEntry struct {
	Index  int    // Index of the file in the table
	Offset uint32 // Offset of the file in the bundle
	Length uint32 // Length of the file data
	Name   string // Name of the file
}

// ListBundle reads a bundle file and prints the table data
func ListBundle(bundlePath string) error {
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

func ExtractBundle(bundlePath, extractPath, pattern string) error {
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

		// Read and decrypt data from the bundle
		decryptedData, err := readAndDecrypt(file, entry.Offset, entry.Length)
		if err != nil {
			return fmt.Errorf("error extracting from bundle: %w", err)
		}

		outputPath := extractPath + string(os.PathSeparator) + entry.Name
		// Create directories as needed for the output path
		dirPath := filepath.Dir(outputPath)
		if err = os.MkdirAll(dirPath, os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory for %s: %w", outputPath, err)
		}

		// Handle CNV conversion using shared helper
		decryptedData, outputPath, err = handleCNVExtraction(decryptedData, outputPath)
		if err != nil {
			fmt.Printf("Conversion error for %s: %v, saving as .unknown\n", entry.Name, err)
			outputPath = fileutil.ChangeExt(outputPath, ".unknown")
		}

		// Write the converted data to a new file
		err = os.WriteFile(outputPath, decryptedData, 0644)
		if err != nil {
			return fmt.Errorf("unable to write %s: %w", outputPath, err)
		}
	}
	return nil
}

// probably useless
func matchFileToIndex(filePath string, fileEntries []*FileEntry) (int, error) {
	// Normalize input name and extension for case-insensitive matching
	fileName := filepath.Base(filePath)
	nameWithoutExt, ext := fileutil.GetLowerBaseAndExt(fileName)

	// Special-case known mappings (documented indices for specific DATs)
	specials := map[string]map[string]int{
		"title": {
			".ogg": 6,
			".wav": 6,
			".sfl": 5,
		},
		"titlesemi": {
			".ogg": 4,
			".wav": 4,
			".sfl": 3,
		},
	}
	if m, ok := specials[nameWithoutExt]; ok {
		if idx, ok2 := m[ext]; ok2 {
			return idx, nil
		}
	}

	// First pass: exact base-name match against table entries, return the entry's Index
	for _, entry := range fileEntries {
		entryBase, entryExt := fileutil.GetLowerBaseAndExt(entry.Name)
		if entryBase != nameWithoutExt {
			continue
		}
		if ext == entryExt || (ext == ".bmp" && entryExt == ".cnv") {
			return entry.Index, nil
		}
	}

	// Second pass (fallback): looser contains-based matching but prefer correct extension
	for _, entry := range fileEntries {
		entryName := strings.ToLower(entry.Name)
		if strings.Contains(entryName, nameWithoutExt) {
			if (ext == ".ogg" && fileutil.HasExtCI(entry.Name, ".ogg")) ||
				(ext == ".sfl" && fileutil.HasExtCI(entry.Name, ".sfl")) ||
				(ext == ".bmp" && fileutil.HasExtCI(entry.Name, ".cnv")) {
				return entry.Index, nil
			}
		}
	}

	return -1, fmt.Errorf("could not find a matching entry for %s", filePath)
}

// patchFileByIndex is the improved patchFile function that uses indices rather than names

// ExtractSingleFile extracts a single file from the bundle to a specified path
func ExtractSingleFile(bundlePath string, fileIndex int, outputPath string) error {
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

	// Read and decrypt file data
	decryptedData, err := readAndDecrypt(file, entry.Offset, entry.Length)
	if err != nil {
		return fmt.Errorf("error extracting from bundle: %w", err)
	}

	// Handle conversion using helper and write output
	finalOutputData, finalOutputPath, err := handleCNVExtraction(decryptedData, outputPath)
	if err != nil {
		return fmt.Errorf("conversion error: %w", err)
	}

	dirPath := filepath.Dir(finalOutputPath)
	if err = os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("error creating directory for %s: %w", finalOutputPath, err)
	}

	if err = os.WriteFile(finalOutputPath, finalOutputData, 0644); err != nil {
		return fmt.Errorf("unable to write %s: %w", finalOutputPath, err)
	}

	return nil
}
