package main

import (
	"github.com/hajimehoshi/guigui"
	"github.com/hajimehoshi/guigui/basicwidget"
)

// Sidebar widget for navigation
type Sidebar struct {
	guigui.DefaultWidget

	panel        basicwidget.Panel
	panelContent sidebarContent
}

func (s *Sidebar) SetModel(model *Model) {
	s.panelContent.SetModel(model)
}

func (s *Sidebar) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	s.panel.SetStyle(basicwidget.PanelStyleSide)
	s.panel.SetBorder(basicwidget.PanelBorder{
		End: true,
	})
	context.SetSize(&s.panelContent, context.ActualSize(s))
	s.panel.SetContent(&s.panelContent)

	appender.AppendChildWidgetWithBounds(&s.panel, context.Bounds(s))
	return nil
}

type sidebarContent struct {
	guigui.DefaultWidget

	list  basicwidget.List[string]
	model *Model
}

func (s *sidebarContent) SetModel(model *Model) {
	s.model = model
}

func (s *sidebarContent) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	s.list.SetStyle(basicwidget.ListStyleSidebar)

	items := []basicwidget.ListItem[string]{
		{
			Text: "Bundle Viewer",
			ID:   "viewer",
		},
		{
			Text: "File Extractor",
			ID:   "extractor",
		},
	}

	s.list.SetItems(items)
	s.list.SelectItemByID(s.model.Mode())
	s.list.SetItemHeight(basicwidget.UnitSize(context))
	s.list.SetOnItemSelected(func(index int) {
		item, ok := s.list.ItemByIndex(index)
		if !ok {
			s.model.SetMode("viewer")
			return
		}
		s.model.SetMode(item.ID)
	})

	appender.AppendChildWidgetWithBounds(&s.list, context.Bounds(s))
	return nil
}
