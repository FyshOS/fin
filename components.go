package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

type halfPad struct {
}

func (h *halfPad) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Move(fyne.NewPos(theme.Padding()/2, theme.Padding()/2))
	objs[0].Resize(s.Subtract(fyne.NewSize(theme.Padding(), theme.Padding())))
}

func (h *halfPad) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return objs[0].MinSize().Add(fyne.NewSize(theme.Padding(), theme.Padding()))
}

func newButtonBackground(c color.Color) fyne.CanvasObject {
	return container.New(&halfPad{}, canvas.NewRectangle(c))
}
