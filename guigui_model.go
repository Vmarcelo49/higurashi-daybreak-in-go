package main

import (
	"os"
)

// Bundle represents a loaded .DAT bundle file
type Bundle struct {
	filePath     string
	fileEntries  []*FileEntry
	fileEntryMap map[string]*FileEntry
}

// LoadBundle loads a .DAT bundle file and returns a Bundle struct
func LoadBundle(filePath string) (*Bundle, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileEntryMap, fileEntries, err := getTableData(file)
	if err != nil {
		return nil, err
	}

	return &Bundle{
		filePath:     filePath,
		fileEntries:  fileEntries,
		fileEntryMap: fileEntryMap,
	}, nil
}

// Model holds the application state for the guigui interface
type Model struct {
	mode              string
	datFilePath       string
	bundle            *Bundle
	selectedFileIndex int
	searchQuery       string
	outputDir         string
	extractPattern    string
	status            string

	// Settings
	showHiddenFiles   bool
	autoExtractBmp    bool
	preserveStructure bool
	fileFilter        string // "all", "bmp", "txt", "dat", "other"

	// Callback for UI updates
	onUpdate func()
}

func (m *Model) Mode() string {
	if m.mode == "" {
		return "viewer"
	}
	return m.mode
}

func (m *Model) SetMode(mode string) {
	m.mode = mode
}

func (m *Model) LoadDatFile(filePath string) error {
	m.datFilePath = filePath

	bundle, err := LoadBundle(filePath)
	if err != nil {
		m.status = "Failed to load file: " + err.Error()
		m.triggerUpdate()
		return err
	}

	m.bundle = bundle
	m.selectedFileIndex = -1
	m.status = "File loaded successfully"
	m.triggerUpdate()
	return nil
}

func (m *Model) SelectedFileIndex() int {
	return m.selectedFileIndex
}

func (m *Model) SetSelectedFileIndex(index int) {
	m.selectedFileIndex = index
}

func (m *Model) SearchQuery() string {
	return m.searchQuery
}

func (m *Model) SetSearchQuery(query string) {
	m.searchQuery = query
}

func (m *Model) OutputDir() string {
	return m.outputDir
}

func (m *Model) SetOutputDir(dir string) {
	m.outputDir = dir
	m.triggerUpdate()
}

func (m *Model) ExtractPattern() string {
	return m.extractPattern
}

func (m *Model) SetExtractPattern(pattern string) {
	m.extractPattern = pattern
}

func (m *Model) Status() string {
	if m.status == "" {
		return "Ready"
	}
	return m.status
}

func (m *Model) SetStatus(status string) {
	m.status = status
}

func (m *Model) SetUpdateCallback(callback func()) {
	m.onUpdate = callback
}

func (m *Model) triggerUpdate() {
	if m.onUpdate != nil {
		m.onUpdate()
	}
}

// Settings getters and setters
func (m *Model) ShowHiddenFiles() bool {
	return m.showHiddenFiles
}

func (m *Model) SetShowHiddenFiles(show bool) {
	m.showHiddenFiles = show
	m.triggerUpdate()
}

func (m *Model) AutoExtractBmp() bool {
	return m.autoExtractBmp
}

func (m *Model) SetAutoExtractBmp(auto bool) {
	m.autoExtractBmp = auto
	m.triggerUpdate()
}

func (m *Model) PreserveStructure() bool {
	return m.preserveStructure
}

func (m *Model) SetPreserveStructure(preserve bool) {
	m.preserveStructure = preserve
	m.triggerUpdate()
}

func (m *Model) FileFilter() string {
	if m.fileFilter == "" {
		return "all"
	}
	return m.fileFilter
}

func (m *Model) SetFileFilter(filter string) {
	m.fileFilter = filter
	m.triggerUpdate()
}

// NewModel creates a new model with default settings
func NewModel() *Model {
	return &Model{
		mode:              "viewer",
		selectedFileIndex: -1,
		// Set all toggles to true by default
		showHiddenFiles:   true,
		autoExtractBmp:    true,
		preserveStructure: true,
		fileFilter:        "all",
	}
}
