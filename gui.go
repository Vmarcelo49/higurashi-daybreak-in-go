package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// GUI represents the GUI application
type GUI struct {
	app               fyne.App
	window            fyne.Window
	currentDatFile    string
	fileEntries       []*FileEntry
	fileList          *widget.List
	detailsView       *widget.Label
	outputDir         string
	searchEntry       *widget.Entry
	statusBar         *widget.Label
	tableData         *widget.Label
	extractButton     *widget.Button
	patchButton       *widget.Button
	singlePatchButton *widget.Button
	selectedID        int // Stores the currently selected item ID
}

// NewGUI creates a new GUI
func NewGUI(initialFile string) *GUI {
	app := app.New()
	app.Settings().SetTheme(theme.DarkTheme())
	window := app.NewWindow("Daybreak Bundle Tools")
	window.Resize(fyne.NewSize(900, 600))

	gui := &GUI{
		app:            app,
		window:         window,
		selectedID:     -1,          // Initialize with invalid ID
		currentDatFile: initialFile, // Store the initial file path
	}

	return gui
}

// Run starts the GUI
func (g *GUI) Run() {
	// File selection button
	openButton := widget.NewButtonWithIcon("Open .DAT File", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, g.window)
				return
			}
			if reader == nil {
				return // User cancelled
			}

			filePath := reader.URI().Path()
			if runtime.GOOS == "windows" {
				// Convert URI path to Windows path
				filePath = strings.TrimPrefix(filePath, "/")
			}
			g.loadDatFile(filePath)
		}, g.window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".dat", ".DAT"}))
		fd.Show()
	})

	// Search field
	g.searchEntry = widget.NewEntry()
	g.searchEntry.SetPlaceHolder("Search files...")
	g.searchEntry.OnChanged = func(text string) {
		g.filterFileList(text)
	}

	// Extract All button (repurposed from the output directory selection)
	outputDirButton := widget.NewButtonWithIcon("Extract All Files", theme.DownloadIcon(), func() {
		if g.currentDatFile == "" {
			dialog.ShowInformation("Error", "Please load a DAT file first", g.window)
			return
		}

		fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, g.window)
				return
			}
			if uri == nil {
				return // User cancelled
			}

			outputPath := uri.Path()
			if runtime.GOOS == "windows" {
				// Convert URI path to Windows path
				outputPath = strings.TrimPrefix(outputPath, "/")
			}
			g.outputDir = outputPath
			g.statusBar.SetText(fmt.Sprintf("Output directory set to: %s", g.outputDir))

			// Start extracting all files
			g.extractAll()

			// Enable patch button as we now have set an output dir
			g.patchButton.Enable()
		}, g.window)
		fd.Show()
	})

	// Status bar
	g.statusBar = widget.NewLabel("Welcome to Daybreak Bundle Tools")

	// Table data (shows bundle metadata)
	g.tableData = widget.NewLabel("No DAT file loaded")
	g.tableData.Wrapping = fyne.TextWrapWord

	// File list
	g.fileList = widget.NewList(
		func() int { return len(g.fileEntries) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(g.fileEntries) {
				label := obj.(*widget.Label)
				label.SetText(g.fileEntries[id].Name)
			}
		},
	)

	// Details view
	g.detailsView = widget.NewLabel("Select a file to view details")
	g.detailsView.Wrapping = fyne.TextWrapWord

	// Extract button
	g.extractButton = widget.NewButtonWithIcon("Extract Selected", theme.DownloadIcon(), func() {
		selected := g.selectedID
		if selected >= 0 && selected < len(g.fileEntries) {
			entry := g.fileEntries[selected]

			// Use file save dialog for all platforms
			saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, g.window)
					return
				}
				if writer == nil {
					return // User cancelled
				}
				// Get the file path from the URI
				uri := writer.URI()
				var outputPath string
				// Handle path conversion in a cross-platform manner
				if uri.Scheme() == "file" {
					// For local files, convert to OS path format
					outputPath = uri.Path()

					// Handle platform-specific path conversions
					if runtime.GOOS == "windows" {
						// Remove leading slash and convert to Windows path format
						outputPath = strings.TrimPrefix(outputPath, "/")
						outputPath = filepath.FromSlash(outputPath)
					}
				} else {
					// For non-file URIs, just use the path portion
					// Close the writer - we'll handle file writing manually
					writer.Close()
					g.showError(fmt.Sprintf("Unsupported URI scheme: %s", uri.Scheme()))
					return
				}

				// Close the writer - we'll handle file writing manually
				writer.Close()

				// Attempt to extract to the selected path
				g.extractFileToPath(entry, outputPath)
			}, g.window)

			// Set file filters based on file type
			var fileExtension string
			if strings.HasSuffix(strings.ToLower(entry.Name), ".cnv") {
				// For cnv files, try to guess the right extension
				if strings.Contains(strings.ToLower(entry.Name), "wav") ||
					strings.Contains(strings.ToLower(entry.Name), "audio") ||
					strings.Contains(strings.ToLower(entry.Name), "sound") {
					fileExtension = "wav"
				} else {
					fileExtension = "bmp" // Use BMP format
				}
			} else {
				// For non-cnv files, use the original extension
				fileExtension = strings.TrimPrefix(filepath.Ext(entry.Name), ".")
			}

			if fileExtension != "" {
				saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{"." + fileExtension}))
			} // Set default filename based on the entry name
			defaultFilename := sanitizeFileName(entry.Name) // Sanitize the filename first
			if strings.HasSuffix(strings.ToLower(defaultFilename), ".cnv") {
				// Check the data key to determine the proper extension
				// We're just guessing the extension here without looking at the data
				if strings.Contains(strings.ToLower(defaultFilename), "wav") ||
					strings.Contains(strings.ToLower(defaultFilename), "audio") ||
					strings.Contains(strings.ToLower(defaultFilename), "sound") {
					defaultFilename = defaultFilename[:len(defaultFilename)-4] + ".wav"
				} else {
					defaultFilename = defaultFilename[:len(defaultFilename)-4] + ".bmp"
				}
			}

			saveDialog.SetFileName(defaultFilename)
			saveDialog.Show()
		} else {
			dialog.ShowInformation("Error", "Please select a file to extract", g.window)
		}
	})
	g.extractButton.Disable()

	// Patch button
	g.patchButton = widget.NewButtonWithIcon("Patch DAT", theme.UploadIcon(), func() {
		if g.currentDatFile != "" && g.outputDir != "" {
			g.patchDat()
		} else if g.outputDir == "" {
			dialog.ShowInformation("Error", "Please set output directory first", g.window)
		} else {
			dialog.ShowInformation("Error", "Please load a DAT file first", g.window)
		}
	})
	g.patchButton.Disable()

	// Single file patch button
	g.singlePatchButton = widget.NewButtonWithIcon("Patch Selected File", theme.UploadIcon(), func() {
		selected := g.selectedID
		if selected >= 0 && selected < len(g.fileEntries) {
			fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil {
					dialog.ShowError(err, g.window)
					return
				}
				if reader == nil {
					return // User cancelled
				}

				// Get the file path
				filePath := reader.URI().Path()
				if runtime.GOOS == "windows" {
					// Convert URI path to Windows path
					filePath = strings.TrimPrefix(filePath, "/")
				}

				// Close the reader as we'll open the file directly
				reader.Close()

				// Attempt to patch the file
				g.patchSingleFile(selected, filePath)
			}, g.window)
			fd.Show()
		} else {
			dialog.ShowInformation("Error", "Please select a file to patch", g.window)
		}
	})
	g.singlePatchButton.Disable() // Initially disabled until a file is selected

	// File selection event
	g.fileList.OnSelected = func(id widget.ListItemID) {
		if id < len(g.fileEntries) {
			g.selectedID = int(id) // Store the selected ID
			entry := g.fileEntries[id]
			g.showFileDetails(entry)
			g.extractButton.Enable()     // Enable extract button whenever a file is selected
			g.singlePatchButton.Enable() // Enable single patch button whenever a file is selected
		}
	}

	// Layout
	fileControls := container.NewHBox(
		openButton,
		outputDirButton, // This is now our Extract All button
	)

	searchContainer := container.NewBorder(
		nil, nil,
		widget.NewIcon(theme.SearchIcon()), nil,
		g.searchEntry,
	)

	actionButtons := container.NewHBox(
		g.extractButton,
		g.singlePatchButton,
		g.patchButton,
	)

	leftPanel := container.NewBorder(
		container.NewVBox(
			searchContainer,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		g.fileList,
	)

	rightPanel := container.NewBorder(
		g.tableData,
		actionButtons,
		nil, nil,
		container.NewScroll(g.detailsView),
	)

	content := container.NewBorder(
		fileControls,
		g.statusBar,
		nil, nil,
		container.NewHSplit(
			leftPanel,
			rightPanel,
		),
	)
	g.window.SetContent(content)

	// If an initial file was provided, load it after the UI is initialized
	if g.currentDatFile != "" {
		initialFile := g.currentDatFile
		// Use a goroutine for the file loading to keep the UI responsive
		go func() {
			g.loadDatFile(initialFile)
		}()
	}

	g.window.Show()
	g.app.Run()
}

// loadDatFile loads a DAT file and populates the UI
func (g *GUI) loadDatFile(filePath string) {
	g.currentDatFile = filePath
	g.statusBar.SetText(fmt.Sprintf("Loading %s...", filePath))

	file, err := os.Open(filePath)
	if err != nil {
		g.showError(fmt.Sprintf("Unable to open %s: %v", filePath, err))
		return
	}
	defer file.Close()

	_, fileEntries, err := getTableData(file)
	if err != nil {
		g.showError(fmt.Sprintf("Error getting table data: %v", err))
		return
	}

	// Sort entries by name
	sort.Slice(fileEntries, func(i, j int) bool {
		return fileEntries[i].Name < fileEntries[j].Name
	})
	g.fileEntries = fileEntries
	g.fileList.Refresh()
	g.selectedID = -1 // Reset selected ID

	// Update info panel
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		g.tableData.SetText(fmt.Sprintf("File: %s\nSize: %.2f MB\nEntries: %d",
			filepath.Base(filePath),
			float64(fileInfo.Size())/1024/1024,
			len(fileEntries),
		))
	}
	g.extractButton.Disable()
	g.singlePatchButton.Disable()
	if g.outputDir != "" {
		g.patchButton.Enable()
	}

	g.statusBar.SetText(fmt.Sprintf("Loaded %s with %d files", filepath.Base(filePath), len(fileEntries)))
}

// showFileDetails displays detailed information about a file entry
func (g *GUI) showFileDetails(entry *FileEntry) {
	g.detailsView.SetText(fmt.Sprintf(
		"Filename: %s\nIndex: %d\nOffset: %d\nLength: %d (%.2f KB)\nType: %s",
		entry.Name,
		entry.Index,
		entry.Offset,
		entry.Length,
		float64(entry.Length)/1024,
		getFileType(entry.Name),
	))

	// Always enable the extract button when a file is selected
	g.extractButton.Enable()
	g.singlePatchButton.Enable()
}

// filterFileList filters the file list based on search text
func (g *GUI) filterFileList(searchText string) {
	if g.currentDatFile == "" {
		return
	}

	file, err := os.Open(g.currentDatFile)
	if err != nil {
		g.showError(fmt.Sprintf("Unable to open %s: %v", g.currentDatFile, err))
		return
	}
	defer file.Close()

	_, allEntries, err := getTableData(file)
	if err != nil {
		g.showError(fmt.Sprintf("Error getting table data: %v", err))
		return
	}

	if searchText == "" {
		g.fileEntries = allEntries
	} else {
		// Filter entries based on search text
		filteredEntries := make([]*FileEntry, 0)
		regex, err := regexp.Compile("(?i)" + regexp.QuoteMeta(searchText))
		if err == nil {
			for _, entry := range allEntries {
				if regex.MatchString(entry.Name) {
					filteredEntries = append(filteredEntries, entry)
				}
			}
			g.fileEntries = filteredEntries
		}
	}

	// Sort entries by name
	sort.Slice(g.fileEntries, func(i, j int) bool {
		return g.fileEntries[i].Name < g.fileEntries[j].Name
	})
	g.fileList.Refresh()
	g.selectedID = -1 // Reset selected ID when filtering
	g.statusBar.SetText(fmt.Sprintf("Found %d matching files", len(g.fileEntries)))
}

// extractFileToPath extracts a single file from the current DAT file to a specific path
func (g *GUI) extractFileToPath(entry *FileEntry, outputPath string) {
	if g.currentDatFile == "" {
		return
	}
	g.statusBar.SetText(fmt.Sprintf("Extracting %s...", entry.Name))

	file, err := os.Open(g.currentDatFile)
	if err != nil {
		g.showError(fmt.Sprintf("Unable to open %s: %v", g.currentDatFile, err))
		return
	}
	defer file.Close()

	// Move the file cursor to the correct position
	if _, err = file.Seek(int64(entry.Offset), 0); err != nil {
		g.showError(fmt.Sprintf("Error seeking in bundle: %v", err))
		return
	}

	// Read data from the file
	fileData := make([]byte, entry.Length)
	bytesRead, err := file.Read(fileData)
	if err != nil {
		g.showError(fmt.Sprintf("Error extracting from bundle: %v", err))
		return
	}
	if bytesRead != int(entry.Length) {
		g.showError(fmt.Sprintf("Expected to read %d bytes, but got %d", entry.Length, bytesRead))
		return
	}
	encryptionKey := getFileKey(int64(entry.Offset))
	var decryptedData []byte
	for i := 0; i < int(entry.Length); i++ {
		decryptedData = append(decryptedData, fileData[i]^byte(encryptionKey))
	}
	// Normalize the output path for the current OS
	outputPath = filepath.Clean(outputPath)

	// Ensure the base filename part doesn't contain any invalid characters
	// This splits the path, sanitizes just the filename, and puts it back together
	dir := filepath.Dir(outputPath)
	filename := filepath.Base(outputPath)
	filename = sanitizeFileName(filename)
	outputPath = filepath.Join(dir, filename)

	g.statusBar.SetText(fmt.Sprintf("Saving to: %s", outputPath))
	// Create directories as needed for the output path
	dirPath := filepath.Dir(outputPath)
	if err = os.MkdirAll(dirPath, 0755); err != nil {
		g.showError(fmt.Sprintf("Error creating directory for %s: %v", outputPath, err))
		return
	}

	// Handle file conversion if needed
	if strings.HasSuffix(strings.ToLower(entry.Name), ".cnv") {
		dataKey := decryptedData[0]
		if dataKey == 1 {
			g.statusBar.SetText(fmt.Sprintf("Converting %s as WAV file...", entry.Name))
			err = convertWav(&decryptedData)
			if err != nil {
				g.showError(fmt.Sprintf("Error converting wav: %v", err))
				return
			}
			// Don't change the output path as it was selected by the user		} else if dataKey == 24 || dataKey == 32 {
			// Always convert image files
			g.statusBar.SetText(fmt.Sprintf("Converting %s as image file (BPP: %d) to bmp format...",
				entry.Name, dataKey))
			err = convertImage(&decryptedData)
			if err != nil {
				g.showError(fmt.Sprintf("Error converting image: %v", err))
				return
			}
			g.statusBar.SetText(fmt.Sprintf("Successfully converted to bmp format, data size: %d bytes",
				len(decryptedData)))
			// Don't change the output path as it was selected by the user
		} else {
			g.statusBar.SetText(fmt.Sprintf("Bad data key (%d) in %s - extracting without conversion", dataKey, outputPath))
		}
	}
	// Try to open and write the file with explicit permissions
	outFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		g.showError(fmt.Sprintf("Unable to create file %s: %v\nPath: %s", outputPath, err, dirPath))
		return
	}
	defer outFile.Close()

	g.statusBar.SetText(fmt.Sprintf("Writing %d bytes to %s", len(decryptedData), outputPath))

	_, err = outFile.Write(decryptedData)
	if err != nil {
		g.showError(fmt.Sprintf("Error writing to %s: %v", outputPath, err))
		return
	}

	g.statusBar.SetText(fmt.Sprintf("Successfully extracted %s to %s", entry.Name, outputPath))
}

// extractAll extracts all files from the current DAT file
func (g *GUI) extractAll() {
	if g.currentDatFile == "" || g.outputDir == "" {
		return
	}

	go func() {
		g.statusBar.SetText("Starting extraction of all files...")
		err := extractBundle(g.currentDatFile, g.outputDir, "")
		if err != nil {
			g.showError(fmt.Sprintf("Error during extraction: %v", err))
			return
		}
		g.statusBar.SetText(fmt.Sprintf("Successfully extracted all files to %s", g.outputDir))
	}()
}

// patchDat patches the current DAT file with files from the output directory
func (g *GUI) patchDat() {
	if g.currentDatFile == "" || g.outputDir == "" {
		return
	}

	dialog.ShowConfirm(
		"Confirm Patch",
		fmt.Sprintf("Are you sure you want to patch %s with files from %s?",
			filepath.Base(g.currentDatFile), g.outputDir),
		func(ok bool) {
			if ok {
				go func() {
					g.statusBar.SetText("Starting patch operation...")
					patchBundle(g.currentDatFile, g.outputDir)
					g.statusBar.SetText(fmt.Sprintf("Successfully patched %s", filepath.Base(g.currentDatFile)))
				}()
			}
		},
		g.window,
	)
}

// Handle patching of a single file
func (g *GUI) patchSingleFile(fileIndex int, inputFilePath string) {
	if g.currentDatFile == "" {
		g.showError("No DAT file is currently loaded")
		return
	}

	// Confirm with the user
	dialog.ShowConfirm(
		"Confirm Single File Patch",
		fmt.Sprintf("Are you sure you want to patch file at index %d with '%s'?\nThis will update the DAT file and adjust offsets for all files that follow.",
			fileIndex, filepath.Base(inputFilePath)),
		func(ok bool) {
			if ok {
				go func() {
					g.statusBar.SetText(fmt.Sprintf("Patching file at index %d...", fileIndex))

					err := patchSingleFile(g.currentDatFile, inputFilePath, fileIndex)
					if err != nil {
						g.showError(fmt.Sprintf("Error during single file patching: %v", err))
						return
					}

					// Reload the DAT file to reflect the changes
					g.loadDatFile(g.currentDatFile)
					g.statusBar.SetText(fmt.Sprintf("Successfully patched file at index %d with %s",
						fileIndex, filepath.Base(inputFilePath)))
				}()
			}
		},
		g.window,
	)
}

// showError displays an error dialog
func (g *GUI) showError(message string) {
	log.Println(message)
	g.statusBar.SetText("Error: " + message)
	dialog.ShowError(fmt.Errorf("%s", message), g.window)
}

// getFileType returns a human-readable file type based on file extension
func getFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".cnv":
		return "Image or audio file (will be converted)"
	case ".wav", ".mp3":
		return "Audio File"
	case ".bmp":
		return "Image File (BMP format)"
	case ".txt":
		return "Text File"
	case ".x":
		return "Binary directx 9 3d model with animations."
	case ".sfl":
		return "Temporary audio file made by vegas."
	default:
		return "Unknown"
	}
}

// sanitizeFileName removes illegal characters from file names
func sanitizeFileName(name string) string {
	// Replace any characters that are invalid in file names
	// This includes: < > : " / \ | ? *
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}
