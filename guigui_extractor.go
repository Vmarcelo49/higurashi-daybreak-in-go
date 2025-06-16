package main

import (
	"fmt"

	"github.com/hajimehoshi/guigui"
	"github.com/hajimehoshi/guigui/basicwidget"
	"github.com/hajimehoshi/guigui/layout"
	"github.com/sqweek/dialog"
)

// FileExtractor widget for extracting files
type FileExtractor struct {
	guigui.DefaultWidget

	form             basicwidget.Form
	extractAllButton basicwidget.Button
	outputDirText    basicwidget.Text
	selectDirButton  basicwidget.Button
	patternText      basicwidget.Text
	patternEntry     basicwidget.TextInput
	progressText     basicwidget.Text
	statusText       basicwidget.Text

	// Settings
	settingsForm         basicwidget.Form
	autoExtractText      basicwidget.Text
	autoExtractToggle    basicwidget.Toggle
	preserveStructText   basicwidget.Text
	preserveStructToggle basicwidget.Toggle
	showHiddenText       basicwidget.Text
	showHiddenToggle     basicwidget.Toggle

	model *Model
}

func (e *FileExtractor) SetModel(model *Model) {
	e.model = model
}

func (e *FileExtractor) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	u := basicwidget.UnitSize(context)
	// Output directory selection
	if e.model.outputDir != "" {
		e.outputDirText.SetValue("Output: " + e.model.outputDir)
	} else {
		e.outputDirText.SetValue("Output: Not selected")
	}

	e.selectDirButton.SetText("Select Output Directory")
	e.selectDirButton.SetOnUp(func() {
		directory, err := dialog.Directory().Title("Select Output Directory").Browse()
		if err != nil {
			// User cancelled or error occurred
			if err.Error() != "Cancelled" {
				// TODO: Show error dialog
				fmt.Printf("Error selecting directory: %v\n", err)
			}
			return
		}

		// Set the selected directory
		e.model.SetOutputDir(directory)
	})

	// Pattern entry (no placeholder, just a label)
	e.patternText.SetValue("File Pattern (optional):")
	e.patternEntry.SetOnValueChanged(func(text string, committed bool) {
		if committed {
			e.model.SetExtractPattern(text)
		}
	})

	// Extract all button
	e.extractAllButton.SetText("Extract All Files")
	enabled := e.model.bundle != nil && e.model.outputDir != ""
	context.SetEnabled(&e.extractAllButton, enabled)
	e.extractAllButton.SetOnUp(func() {
		// TODO: Implement extraction
		e.model.SetStatus("Extracting files...")
	})
	// Progress and status
	e.progressText.SetValue("Progress: Ready")
	e.statusText.SetValue("Status: " + e.model.Status())

	// Settings
	e.autoExtractText.SetValue("Auto-extract BMP files:")
	e.autoExtractToggle.SetValue(e.model.AutoExtractBmp())
	e.autoExtractToggle.SetOnValueChanged(func(value bool) {
		e.model.SetAutoExtractBmp(value)
	})

	e.preserveStructText.SetValue("Preserve folder structure:")
	e.preserveStructToggle.SetValue(e.model.PreserveStructure())
	e.preserveStructToggle.SetOnValueChanged(func(value bool) {
		e.model.SetPreserveStructure(value)
	})

	e.showHiddenText.SetValue("Show hidden files:")
	e.showHiddenToggle.SetValue(e.model.ShowHiddenFiles())
	e.showHiddenToggle.SetOnValueChanged(func(value bool) {
		e.model.SetShowHiddenFiles(value)
	})

	e.form.SetItems([]basicwidget.FormItem{
		{
			PrimaryWidget:   &e.outputDirText,
			SecondaryWidget: &e.selectDirButton,
		},
		{
			PrimaryWidget:   &e.patternText,
			SecondaryWidget: &e.patternEntry,
		},
		{
			PrimaryWidget: &e.extractAllButton,
		},
		{
			PrimaryWidget: &e.progressText,
		},
		{
			PrimaryWidget: &e.statusText,
		},
	})

	e.settingsForm.SetItems([]basicwidget.FormItem{
		{
			PrimaryWidget:   &e.autoExtractText,
			SecondaryWidget: &e.autoExtractToggle,
		},
		{
			PrimaryWidget:   &e.preserveStructText,
			SecondaryWidget: &e.preserveStructToggle,
		},
		{
			PrimaryWidget:   &e.showHiddenText,
			SecondaryWidget: &e.showHiddenToggle,
		},
	})
	// Layout with settings panel
	gl := layout.GridLayout{
		Bounds: context.Bounds(e).Inset(u),
		Heights: []layout.Size{
			layout.FlexibleSize(2), // Main form
			layout.FlexibleSize(1), // Settings
		},
		RowGap: u,
	}

	mainFormBounds := gl.CellBounds(0, 0)
	settingsFormBounds := gl.CellBounds(0, 1)

	appender.AppendChildWidgetWithBounds(&e.form, mainFormBounds)
	appender.AppendChildWidgetWithBounds(&e.settingsForm, settingsFormBounds)
	return nil
}
