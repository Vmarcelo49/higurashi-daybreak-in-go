package main

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/guigui"
	"github.com/hajimehoshi/guigui/basicwidget"
	"github.com/hajimehoshi/guigui/layout"
)

// Main GUI root widget
type DaybreakGUI struct {
	guigui.DefaultWidget

	background    basicwidget.Background
	sidebar       Sidebar
	bundleViewer  BundleViewer
	fileExtractor FileExtractor

	// Dialogs
	errorDialog   ErrorDialog
	successDialog SuccessDialog

	model Model
}

func (g *DaybreakGUI) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	appender.AppendChildWidgetWithBounds(&g.background, context.Bounds(g))

	g.sidebar.SetModel(&g.model)
	g.bundleViewer.SetModel(&g.model)
	g.fileExtractor.SetModel(&g.model)

	gl := layout.GridLayout{
		Bounds: context.Bounds(g),
		Widths: []layout.Size{
			layout.FixedSize(8 * basicwidget.UnitSize(context)),
			layout.FlexibleSize(1),
		},
	}
	appender.AppendChildWidgetWithBounds(&g.sidebar, gl.CellBounds(0, 0))
	bounds := gl.CellBounds(1, 0)

	switch g.model.Mode() {
	case "viewer":
		appender.AppendChildWidgetWithBounds(&g.bundleViewer, bounds)
	case "extractor":
		appender.AppendChildWidgetWithBounds(&g.fileExtractor, bounds)
	default:
		appender.AppendChildWidgetWithBounds(&g.bundleViewer, bounds)
	}

	// Add dialogs (they manage their own positioning)
	appender.AppendChildWidget(&g.errorDialog)
	appender.AppendChildWidget(&g.successDialog)

	return nil
}

// NewGuiguiGUI creates a new GUI using guigui
func NewGuiguiGUI(initialFile string) *DaybreakGUI {
	gui := &DaybreakGUI{
		model: *NewModel(), // Initialize with default settings
	}
	if initialFile != "" {
		gui.model.LoadDatFile(initialFile)
	}
	return gui
}

// RunGuiguiGUI starts the guigui-based GUI
func RunGuiguiGUI(initialFile string) error {
	gui := NewGuiguiGUI(initialFile)

	op := &guigui.RunOptions{
		Title:      "Higurashi Daybreak Bundle Tools",
		WindowSize: image.Pt(900, 600),
		RunGameOptions: &ebiten.RunGameOptions{
			ApplePressAndHoldEnabled: true,
		},
	}

	return guigui.Run(gui, op)
}
