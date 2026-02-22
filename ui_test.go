package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/stretchr/testify/assert"
)

func emptyUsers() []string {
	return nil
}

func oneUser() []string {
	return []string{"user1"}
}

func twoUsers() []string {
	return []string{"user1", "user2"}
}

func TestUI(t *testing.T) {
	a := test.NewApp()
	defer test.NewApp()

	g := newGUI()
	window := a.NewWindow("Fin")
	g.win = window
	window.SetContent(g.makeUI())
	ui := newUI(g, a.Preferences(), oneUser)
	ui.loadUI(nil)
	window.Resize(fyne.NewSize(370, 475))

	// TODO understand why only the unit test requires this to prop open
	panel := window.Content().(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[2].(*fyne.Container).Objects[0]
	scroll := panel.(*fyne.Container).Objects[0].(*container.Scroll)
	scroll.Content.Resize(scroll.Size())

	test.AssertImageMatches(t, "ui_initial.png", window.Canvas().Capture())
}

func TestUI_EnterLogin(t *testing.T) {
	g := newGUI()
	w := test.NewWindow(g.makeUI())
	g.win = w
	ui := newUI(g, test.NewApp().Preferences(), emptyUsers)
	ui.loadUI(nil)

	w.Canvas().Focus(ui.pass)
	ui.pass.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter})
	assert.NotEqual(t, ui.pass, w.Canvas().Focused())
}

func TestUI_Focus(t *testing.T) {
	g := newGUI()
	w := test.NewWindow(g.makeUI())
	g.win = w
	ui := newUI(g, test.NewApp().Preferences(), emptyUsers)
	ui.loadUI(nil)

	w.Canvas().FocusNext()
	assert.Equal(t, ui.pass, w.Canvas().Focused())

	w = test.NewWindow(g.makeUI())
	g.win = w
	ui = newUI(g, test.NewApp().Preferences(), oneUser)
	ui.loadUI(nil)

	assert.Equal(t, ui.pass, w.Canvas().Focused())

	w = test.NewWindow(g.makeUI())
	g.win = w
	ui = newUI(g, test.NewApp().Preferences(), twoUsers)
	ui.loadUI(nil)

	assert.Equal(t, nil, w.Canvas().Focused())
}

func TestUI_RequireFields(t *testing.T) {
	g := newGUI()
	w := test.NewWindow(g.makeUI())
	g.win = w
	ui := newUI(g, test.NewApp().Preferences(), emptyUsers)
	ui.loadUI(nil)

	assert.Zero(t, len(w.Canvas().Overlays().List()))
	ui.doLogin()
	assert.NotZero(t, len(w.Canvas().Overlays().List()))

	w.Canvas().Overlays().Remove(w.Canvas().Overlays().List()[0])
	ui.user = "user" // simulate tapping avatar
	assert.Zero(t, len(w.Canvas().Overlays().List()))
	ui.doLogin()
	assert.NotZero(t, len(w.Canvas().Overlays().List()))

	w.Canvas().Overlays().Remove(w.Canvas().Overlays().List()[0])
	ui.user = "" // avatar unset
	ui.pass.SetText("pass")
	assert.Zero(t, len(w.Canvas().Overlays().List()))
	ui.doLogin()
	assert.NotZero(t, len(w.Canvas().Overlays().List()))
}
