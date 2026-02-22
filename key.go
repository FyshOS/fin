package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/FyshOS/dryvers"
)

const (
	scanCodeBrightnessDown = 232
	scanCodeBrightnessUp   = 233
)

func offsetValue(diff float64, b *dryvers.Brightness) error {
	floatVal, _ := b.Get()
	if floatVal <= 0.01 { // don't start doing 6, 11 etc just because we were on 1 (min)
		floatVal = 0
	}

	return b.Set(floatVal + diff)
}

func brightnessFunc(b *dryvers.Brightness) func(event *fyne.KeyEvent) {
	return func(event *fyne.KeyEvent) {
		switch event.Physical.ScanCode {
		case scanCodeBrightnessDown:
			_ = offsetValue(-0.05, b)
		case scanCodeBrightnessUp:
			_ = offsetValue(0.05, b)
		}
	}
}

type finPasswordEntry struct {
	widget.Entry

	bright func(event *fyne.KeyEvent)
}

func (e *finPasswordEntry) TypedKey(key *fyne.KeyEvent) {
	e.bright(key)

	e.Entry.TypedKey(key)
}

func newKeyEntry(b *dryvers.Brightness) *finPasswordEntry {
	e := &finPasswordEntry{bright: brightnessFunc(b)}
	e.Password = true
	e.ExtendBaseWidget(e)
	return e
}
