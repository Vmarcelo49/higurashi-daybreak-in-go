package main

import (
	"image"

	"github.com/hajimehoshi/guigui"
	"github.com/hajimehoshi/guigui/basicwidget"
	"github.com/hajimehoshi/guigui/layout"
)

// ErrorDialog shows error messages in a popup
type ErrorDialog struct {
	guigui.DefaultWidget

	popup   basicwidget.Popup
	content errorDialogContent
}

func (e *ErrorDialog) Show(context *guigui.Context, message string) {
	e.content.SetMessage(message)
	e.content.SetPopup(&e.popup)
	e.popup.SetContent(&e.content)
	e.popup.SetBackgroundBlurred(true)
	e.popup.SetCloseByClickingOutside(true)
	e.popup.SetAnimationDuringFade(true)

	// Center the popup
	u := basicwidget.UnitSize(context)
	appBounds := context.AppBounds()
	contentSize := image.Pt(int(16*u), int(8*u))

	popupPosition := image.Point{
		X: appBounds.Min.X + (appBounds.Dx()-contentSize.X)/2,
		Y: appBounds.Min.Y + (appBounds.Dy()-contentSize.Y)/2,
	}
	popupBounds := image.Rectangle{
		Min: popupPosition,
		Max: popupPosition.Add(contentSize),
	}

	context.SetSize(&e.content, popupBounds.Size())
	e.popup.Open(context)
}

func (e *ErrorDialog) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	// The popup will be positioned when Show() is called
	appender.AppendChildWidget(&e.popup)
	return nil
}

type errorDialogContent struct {
	guigui.DefaultWidget

	popup       *basicwidget.Popup
	titleText   basicwidget.Text
	messageText basicwidget.Text
	okButton    basicwidget.Button
	message     string
}

func (e *errorDialogContent) SetMessage(message string) {
	e.message = message
}

func (e *errorDialogContent) SetPopup(popup *basicwidget.Popup) {
	e.popup = popup
}

func (e *errorDialogContent) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	u := basicwidget.UnitSize(context)

	e.titleText.SetValue("Error")
	e.titleText.SetBold(true)
	e.messageText.SetValue(e.message)
	// Text wrapping may not be available, remove if not supported

	e.okButton.SetText("OK")
	e.okButton.SetOnUp(func() {
		if e.popup != nil {
			e.popup.Close()
		}
	})

	gl := layout.GridLayout{
		Bounds: context.Bounds(e).Inset(u),
		Heights: []layout.Size{
			layout.FixedSize(2 * u), // Title
			layout.FlexibleSize(1),  // Message
			layout.FixedSize(3 * u), // Button
		},
		RowGap: u / 2,
	}

	appender.AppendChildWidgetWithBounds(&e.titleText, gl.CellBounds(0, 0))
	appender.AppendChildWidgetWithBounds(&e.messageText, gl.CellBounds(0, 1))

	// Center the OK button
	buttonBounds := gl.CellBounds(0, 2)
	buttonSize := e.okButton.DefaultSize(context)
	centeredButtonBounds := image.Rect(
		buttonBounds.Min.X+(buttonBounds.Dx()-buttonSize.X)/2,
		buttonBounds.Min.Y,
		buttonBounds.Min.X+(buttonBounds.Dx()-buttonSize.X)/2+buttonSize.X,
		buttonBounds.Max.Y,
	)
	appender.AppendChildWidgetWithBounds(&e.okButton, centeredButtonBounds)

	return nil
}

// SuccessDialog shows success messages in a popup
type SuccessDialog struct {
	guigui.DefaultWidget

	popup   basicwidget.Popup
	content successDialogContent
}

func (s *SuccessDialog) Show(context *guigui.Context, message string) {
	s.content.SetMessage(message)
	s.content.SetPopup(&s.popup)
	s.popup.SetContent(&s.content)
	s.popup.SetBackgroundBlurred(true)
	s.popup.SetCloseByClickingOutside(true)
	s.popup.SetAnimationDuringFade(true)

	// Center the popup
	u := basicwidget.UnitSize(context)
	appBounds := context.AppBounds()
	contentSize := image.Pt(int(16*u), int(8*u))

	popupPosition := image.Point{
		X: appBounds.Min.X + (appBounds.Dx()-contentSize.X)/2,
		Y: appBounds.Min.Y + (appBounds.Dy()-contentSize.Y)/2,
	}
	popupBounds := image.Rectangle{
		Min: popupPosition,
		Max: popupPosition.Add(contentSize),
	}

	context.SetSize(&s.content, popupBounds.Size())
	s.popup.Open(context)
}

func (s *SuccessDialog) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	appender.AppendChildWidget(&s.popup)
	return nil
}

type successDialogContent struct {
	guigui.DefaultWidget

	popup       *basicwidget.Popup
	titleText   basicwidget.Text
	messageText basicwidget.Text
	okButton    basicwidget.Button
	message     string
}

func (s *successDialogContent) SetMessage(message string) {
	s.message = message
}

func (s *successDialogContent) SetPopup(popup *basicwidget.Popup) {
	s.popup = popup
}

func (s *successDialogContent) Build(context *guigui.Context, appender *guigui.ChildWidgetAppender) error {
	u := basicwidget.UnitSize(context)

	s.titleText.SetValue("Success")
	s.titleText.SetBold(true)
	s.messageText.SetValue(s.message)
	// Text wrapping may not be available, remove if not supported

	s.okButton.SetText("OK")
	s.okButton.SetOnUp(func() {
		if s.popup != nil {
			s.popup.Close()
		}
	})

	gl := layout.GridLayout{
		Bounds: context.Bounds(s).Inset(u),
		Heights: []layout.Size{
			layout.FixedSize(2 * u), // Title
			layout.FlexibleSize(1),  // Message
			layout.FixedSize(3 * u), // Button
		},
		RowGap: u / 2,
	}

	appender.AppendChildWidgetWithBounds(&s.titleText, gl.CellBounds(0, 0))
	appender.AppendChildWidgetWithBounds(&s.messageText, gl.CellBounds(0, 1))

	// Center the OK button
	buttonBounds := gl.CellBounds(0, 2)
	buttonSize := s.okButton.DefaultSize(context)
	centeredButtonBounds := image.Rect(
		buttonBounds.Min.X+(buttonBounds.Dx()-buttonSize.X)/2,
		buttonBounds.Min.Y,
		buttonBounds.Min.X+(buttonBounds.Dx()-buttonSize.X)/2+buttonSize.X,
		buttonBounds.Max.Y,
	)
	appender.AppendChildWidgetWithBounds(&s.okButton, centeredButtonBounds)

	return nil
}
