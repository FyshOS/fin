package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/stretchr/testify/assert"
)

func testHostname() string {
	return "test"
}

func emptyUsers() []string {
	return nil
}

func TestUI(t *testing.T) {
	a := test.NewApp()
	defer test.NewApp()
	window := test.NewWindow(nil)
	defer window.Close()

	ui := newUI(window, a.Preferences(), testHostname, emptyUsers)
	ui.loadUI()
	window.Resize(window.Content().MinSize().Add(fyne.NewSize(100, 100)))

	test.AssertImageMatches(t, "ui_initial.png", window.Canvas().Capture())
}

func TestUI_EnterLogin(t *testing.T) {
	w := test.NewWindow(nil)
	ui := newUI(w, test.NewApp().Preferences(), testHostname, emptyUsers)
	ui.loadUI()

	w.Canvas().Focus(ui.pass)
	ui.pass.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter})
	assert.NotEqual(t, ui.pass, w.Canvas().Focused())
}

func TestUI_Focus(t *testing.T) {
	w := test.NewWindow(nil)
	ui := newUI(w, test.NewApp().Preferences(), testHostname, emptyUsers)
	ui.loadUI()

	w.Canvas().FocusNext()
	assert.Equal(t, ui.pass, w.Canvas().Focused())
}

func TestUI_RequireFields(t *testing.T) {
	w := test.NewWindow(nil)
	ui := newUI(w, test.NewApp().Preferences(), testHostname, emptyUsers)
	ui.loadUI()

	assert.Zero(t, ui.err.Text)
	ui.doLogin()
	assert.NotZero(t, ui.err.Text)

	ui.setError("")
	ui.user = "user" // simulate tapping avatar
	assert.Zero(t, ui.err.Text)
	ui.doLogin()
	assert.NotZero(t, ui.err.Text)

	ui.setError("")
	ui.user = "" // avatar unset
	ui.pass.SetText("pass")
	assert.Zero(t, ui.err.Text)
	ui.doLogin()
	assert.NotZero(t, ui.err.Text)
}
