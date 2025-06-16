package main

import (
	"fmt"
	"image"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/guigui"
	"github.com/hajimehoshi/guigui/basicwidget"
	"github.com/hajimehoshi/guigui/layout"
	"github.com/sqweek/dialog"
)

// sanitizeFilename removes directory separators and invalid characters from a filename
// to make it safe for use as a default filename in file dialogs
func sanitizeFilename(filename string) string {
	// Get just the base filename (removes any directory path)
	clean := filepath.Base(filename)

	// Replace any remaining directory separators that might be in the filename itself
	clean = strings.ReplaceAll(clean, "/", "_")
	clean = strings.ReplaceAll(clean, "\\", "_")

	// Remove any null bytes or other control characters
	clean = strings.ReplaceAll(clean, "\x00", "")

	// If the filename is empty after cleaning, provide a default
	if clean == "" || clean == "." || clean == ".." {
		clean = "extracted_file"
	}

	return clean
}

// BundleViewer widget for displaying bundle contents
type BundleViewer struct {
	guigui.DefaultWidget

	headerPanel    basicwidget.Panel
	headerContent  bundleHeader
	fileListPanel  basicwidget.Panel
	fileList       basicwidget.List[int]
	detailsPanel   basicwidget.Panel
	detailsContent fileDetails

	model *Model
	items []basicwidget.ListItem[int]
}

func (b *BundleViewer) SetModel(model *Model) {
	b.model = model
	b.headerContent.SetModel(model)
	b.detailsContent.SetModel(model)
}

func (b *BundleViewer) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	u := basicwidget.UnitSize(context)
	// Header panel setup
	b.headerPanel.SetStyle(basicwidget.PanelStyleDefault)
	b.headerPanel.SetBorder(basicwidget.PanelBorder{
		Bottom: true,
	})
	context.SetSize(&b.headerContent, image.Pt(context.ActualSize(b).X, 3*u))
	b.headerPanel.SetContent(&b.headerContent)

	// File list setup
	b.fileListPanel.SetStyle(basicwidget.PanelStyleDefault)
	b.fileListPanel.SetBorder(basicwidget.PanelBorder{
		End: true,
	})
	b.items = slices.Delete(b.items, 0, len(b.items))
	if b.model.bundle != nil {
		for i, entry := range b.model.bundle.fileEntries {
			b.items = append(b.items, basicwidget.ListItem[int]{
				Text: entry.Name,
				ID:   i,
			})
		}
	}
	b.fileList.SetItems(b.items)
	b.fileList.SetItemHeight(u)
	b.fileList.SetStripeVisible(true)
	b.fileList.SetOnItemSelected(func(index int) {
		if index >= 0 && b.model.bundle != nil && index < len(b.model.bundle.fileEntries) {
			b.model.SetSelectedFileIndex(index)
		}
	})

	b.fileListPanel.SetContent(&b.fileList)
	// Details panel setup
	b.detailsPanel.SetStyle(basicwidget.PanelStyleDefault)
	context.SetSize(&b.detailsContent, image.Pt(context.ActualSize(b).X/3, context.ActualSize(b).Y-4*u))
	b.detailsPanel.SetContent(&b.detailsContent)

	// Layout
	gl := layout.GridLayout{
		Bounds: context.Bounds(b),
		Heights: []layout.Size{
			layout.FixedSize(4 * u),
			layout.FlexibleSize(1),
		},
		Widths: []layout.Size{
			layout.FlexibleSize(2),
			layout.FlexibleSize(1),
		},
	}

	// Create header bounds manually (spanning across both columns)
	headerBounds := image.Rect(
		context.Bounds(b).Min.X,
		context.Bounds(b).Min.Y,
		context.Bounds(b).Max.X,
		context.Bounds(b).Min.Y+4*u,
	)
	appender.AppendChildWidgetWithBounds(&b.headerPanel, headerBounds)
	appender.AppendChildWidgetWithBounds(&b.fileListPanel, gl.CellBounds(0, 1))
	appender.AppendChildWidgetWithBounds(&b.detailsPanel, gl.CellBounds(1, 1))

	return nil
}

type bundleHeader struct {
	guigui.DefaultWidget

	form           basicwidget.Form
	filePathText   basicwidget.Text
	fileCountText  basicwidget.Text
	loadButton     basicwidget.Button
	searchEntry    basicwidget.TextInput
	filterText     basicwidget.Text
	filterDropdown basicwidget.DropdownList[string]

	model *Model
}

func (h *bundleHeader) SetModel(model *Model) {
	h.model = model
}

func (h *bundleHeader) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	// File path display
	if h.model.datFilePath != "" {
		h.filePathText.SetValue("File: " + h.model.datFilePath)
	} else {
		h.filePathText.SetValue("No file loaded")
	}

	// File count display
	if h.model.bundle != nil {
		h.fileCountText.SetValue(fmt.Sprintf("Files: %d", len(h.model.bundle.fileEntries)))
	} else {
		h.fileCountText.SetValue("Files: 0")
	} // Load button
	h.loadButton.SetText("Load DAT File")
	h.loadButton.SetOnUp(func() {
		filename, err := dialog.File().Filter("DAT files", "dat").Load()
		if err != nil {
			// User cancelled or error occurred
			if err.Error() != "Cancelled" {
				// TODO: Show error dialog
				fmt.Printf("Error selecting file: %v\n", err)
			}
			return
		}

		// Load the selected file
		err = h.model.LoadDatFile(filename)
		if err != nil {
			// TODO: Show error dialog
			fmt.Printf("Error loading file: %v\n", err)
		}
	})
	// Search entry (no placeholder functionality in guigui, so we'll use a label)
	h.searchEntry.SetOnValueChanged(func(text string, committed bool) {
		if committed {
			h.model.SetSearchQuery(text)
		}
	})

	// Filter dropdown
	h.filterText.SetValue("Filter:")
	filterItems := []basicwidget.DropdownListItem[string]{
		{Text: "All Files", ID: "all"},
		{Text: "Images (.bmp)", ID: "bmp"},
		{Text: "Text (.txt)", ID: "txt"},
		{Text: "Data (.dat)", ID: "dat"},
		{Text: "Other", ID: "other"},
	}
	h.filterDropdown.SetItems(filterItems)
	h.filterDropdown.SetOnItemSelected(func(index int) {
		if item, ok := h.filterDropdown.ItemByIndex(index); ok {
			h.model.SetFileFilter(item.ID)
		}
	})

	h.form.SetItems([]basicwidget.FormItem{
		{
			PrimaryWidget:   &h.filePathText,
			SecondaryWidget: &h.loadButton,
		},
		{
			PrimaryWidget:   &h.fileCountText,
			SecondaryWidget: &h.searchEntry,
		},
		{
			PrimaryWidget:   &h.filterText,
			SecondaryWidget: &h.filterDropdown,
		},
	})

	appender.AppendChildWidgetWithBounds(&h.form, context.Bounds(h))
	return nil
}

type fileDetails struct {
	guigui.DefaultWidget

	form          basicwidget.Form
	nameText      basicwidget.Text
	sizeText      basicwidget.Text
	offsetText    basicwidget.Text
	extractButton basicwidget.Button
	handlerSet    bool // Track if button handler has been set

	model *Model
}

func (d *fileDetails) SetModel(model *Model) {
	d.model = model
}

func (d *fileDetails) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	selectedIndex := d.model.SelectedFileIndex()

	if selectedIndex >= 0 && d.model.bundle != nil && selectedIndex < len(d.model.bundle.fileEntries) {
		entry := d.model.bundle.fileEntries[selectedIndex]
		d.nameText.SetValue("Name: " + entry.Name)
		d.sizeText.SetValue(fmt.Sprintf("Size: %d bytes", entry.Length))
		d.offsetText.SetValue(fmt.Sprintf("Offset: 0x%X", entry.Offset))
		d.extractButton.SetText("Extract File") // Only set the handler once to prevent issues
		if !d.handlerSet {
			d.extractButton.SetOnUp(func() {
				// Check if a file is selected
				selectedIndex := d.model.SelectedFileIndex()
				if selectedIndex < 0 || d.model.bundle == nil || selectedIndex >= len(d.model.bundle.fileEntries) {
					fmt.Printf("No file selected for extraction\n")
					return
				}

				// Check if we have a loaded DAT file path
				if d.model.datFilePath == "" {
					fmt.Printf("No DAT file loaded\n")
					return
				}

				// Get the original filename for the selected file
				entry := d.model.bundle.fileEntries[selectedIndex]

				// Sanitize the filename to remove directory separators
				sanitizedName := sanitizeFilename(entry.Name)

				// Open save dialog with the sanitized filename as default
				filename, err := dialog.File().Filter("All files", "*").SetStartFile(sanitizedName).Save()
				if err != nil {
					// User cancelled or error occurred
					if err.Error() != "Cancelled" {
						fmt.Printf("Error selecting save location: %v\n", err)
					}
					return
				}

				// Extract the file
				err = extractSingleFile(d.model.datFilePath, selectedIndex, filename)
				if err != nil {
					fmt.Printf("Error extracting file: %v\n", err)
				} else {
					fmt.Printf("File extracted successfully to: %s\n", filename)
				}
			})
			d.handlerSet = true
		}
	} else {
		d.nameText.SetValue("Name: -")
		d.sizeText.SetValue("Size: -")
		d.offsetText.SetValue("Offset: -")
		d.extractButton.SetText("Extract File") // Set handler once even when no file is selected
		if !d.handlerSet {
			d.extractButton.SetOnUp(func() {
				// Check if a file is selected
				selectedIndex := d.model.SelectedFileIndex()
				if selectedIndex < 0 || d.model.bundle == nil || selectedIndex >= len(d.model.bundle.fileEntries) {
					fmt.Printf("No file selected for extraction\n")
					return
				}

				// Check if we have a loaded DAT file path
				if d.model.datFilePath == "" {
					fmt.Printf("No DAT file loaded\n")
					return
				}

				// Get the original filename for the selected file
				entry := d.model.bundle.fileEntries[selectedIndex]

				// Sanitize the filename to remove directory separators
				sanitizedName := sanitizeFilename(entry.Name)

				// Open save dialog with the sanitized filename as default
				filename, err := dialog.File().Filter("All files", "*").SetStartFile(sanitizedName).Save()
				if err != nil {
					// User cancelled or error occurred
					if err.Error() != "Cancelled" {
						fmt.Printf("Error selecting save location: %v\n", err)
					}
					return
				}

				// Extract the file
				err = extractSingleFile(d.model.datFilePath, selectedIndex, filename)
				if err != nil {
					fmt.Printf("Error extracting file: %v\n", err)
				} else {
					fmt.Printf("File extracted successfully to: %s\n", filename)
				}
			})
			d.handlerSet = true
		}
	}

	d.form.SetItems([]basicwidget.FormItem{
		{
			PrimaryWidget: &d.nameText,
		},
		{
			PrimaryWidget: &d.sizeText,
		},
		{
			PrimaryWidget: &d.offsetText,
		},
		{
			PrimaryWidget: &d.extractButton,
		},
	})

	appender.AppendChildWidgetWithBounds(&d.form, context.Bounds(d))
	return nil
}
