//go:generate fyne bundle -o bundled.go assets

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/FyshOS/backgrounds"
	"github.com/FyshOS/dryvers"
)

const (
	prefSessionKey = "user.%s.session"
	prefUserKey    = "default.user"
)

type ui struct {
	gen     *gui
	pass    *finPasswordEntry
	session *widget.Select

	user     string
	users    func() []string
	sessions []*session
	pref     fyne.Preferences
}

func newUI(g *gui, p fyne.Preferences, users func() []string) *ui {
	return &ui{gen: g, pref: p, sessions: loadSessions(), users: users}
}

func (u *ui) askShutdown() {
	var d *dialog.CustomDialog
	message := widget.NewLabel("Are you sure you want to power off your computer?")
	message.Alignment = fyne.TextAlignCenter

	reboot := widget.NewButtonWithIcon("Reboot", theme.ViewRefreshIcon(), func() {
		d.Hide()
		_ = exec.Command("shutdown", "-r", "now").Start()
	})
	reboot.Importance = widget.WarningImportance
	shutdown := widget.NewButtonWithIcon("Power off", theme.NewThemedResource(resourcePowerSvg), func() {
		d.Hide()
		_ = exec.Command("shutdown", "-h", "now").Start()
	})
	shutdown.Importance = widget.DangerImportance

	buttons := []fyne.CanvasObject{
		widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
			d.Hide()
		}),
		reboot, shutdown,
	}

	d = dialog.NewCustom("Shutdown", "Cancel", message, u.gen.win)
	d.SetButtons(buttons)
	d.Show()
}

func (u *ui) doLogin() {
	if u.user == "" || u.pass.Text == "" {
		dialog.ShowError(errors.New("missing username or password"), u.gen.win)
		return
	}

	pass := u.pass.Text
	u.startSession(func() (int, error) {
		return login(u.user, pass, u.sessionExec())
	})
}

// doFingerprintLogin logs the selected user in with their fingerprint instead of
// a password. pam_fprintd (service fin-fingerprint) blocks until a finger is
// presented; it requires fingerprint login to be enabled in Account settings.
func (u *ui) doFingerprintLogin() {
	if u.user == "" {
		dialog.ShowError(errors.New("please choose a user first"), u.gen.win)
		return
	}

	u.startSession(func() (int, error) {
		return loginFingerprint(u.user, u.sessionExec())
	})
}

// startSession stores the session preference then authenticates and launches the
// user's session on a background goroutine, showing a spinner while it runs. The
// authenticate callback performs the credential check (password or fingerprint).
func (u *ui) startSession(authenticate func() (int, error)) {
	u.pref.SetString(fmt.Sprintf(prefSessionKey, u.user), u.session.Selected)
	u.pref.SetString(prefUserKey, u.user)

	a := widget.NewActivity()
	prop := canvas.NewRectangle(color.Transparent)
	prop.SetMinSize(fyne.NewSquareSize(a.MinSize().Width * 2.5))
	d := dialog.NewCustomWithoutButtons("Logging in...",
		container.NewStack(prop, a), u.gen.win)
	a.Start()
	d.Show()

	go func() {
		pid, err := authenticate()

		fyne.Do(func() {
			d.Hide()
			a.Stop()
		})

		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, u.gen.win)
			})
			return
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, u.gen.win)
				u.gen.win.Show()
			})
			return
		}

		// OpenBSD: give device ownership to logged-in user
		if runtime.GOOS == "openbsd" {
			usr, _ := user.Lookup(u.user)
			uid, _ := strconv.Atoi(usr.Uid)
			_ = os.Chown("/dev/console", uid, -1)
			_ = os.Chown("/dev/dri/card0", uid, -1)
			_ = os.Chown("/dev/dri/renderD128", uid, -1)
		}

		fyne.Do(func() {
			// Leave fullscreen before hiding. Without a window manager we need to release
			// the video mode to avoid possible graphical lock.
			u.gen.win.SetFullScreen(false)
			u.gen.win.Hide()
		})
		_, _ = proc.Wait()

		fyne.Do(func() {
			u.gen.win.SetFullScreen(true)
			u.gen.win.Show()
			u.pass.SetText("")
			u.gen.win.Canvas().Focus(u.pass)
		})
		_ = logout()

		// OpenBSD: give device ownership back to root
		if runtime.GOOS == "openbsd" {
			_ = os.Chown("/dev/console", 0, -1)
			_ = os.Chown("/dev/dri/card0", 0, -1)
			_ = os.Chown("/dev/dri/renderD128", 0, -1)
		}
	}()
}

func (u *ui) loadUI(b *dryvers.Brightness) {
	keyEntry := newKeyEntry(b)
	items := u.gen.form.Items
	u.gen.form.Items = nil
	u.gen.form.Refresh()

	items[0].Widget = keyEntry
	u.gen.form.Items = items
	u.gen.form.Refresh()
	u.pass = keyEntry
	u.pass.OnSubmitted = func(string) {
		u.gen.win.Canvas().Focus(nil)
		u.doLogin()
	}
	u.session = u.gen.form.Items[1].Widget.(*widget.Select)
	u.session.Options = u.sessionNames()

	// Offer fingerprint login when it has been enabled in Account settings
	// (the dedicated PAM service is present). Swiping is an explicit action so
	// it never competes with a password login for the shared PAM handle.
	if _, err := os.Stat("/etc/pam.d/fin-fingerprint"); err == nil {
		fpButton := widget.NewButtonWithIcon("Log in with fingerprint",
			theme.LoginIcon(), func() {
				u.gen.win.Canvas().Focus(nil)
				u.doFingerprintLogin()
			})
		u.gen.form.Items = append(u.gen.form.Items,
			widget.NewFormItem("", fpButton))
		u.gen.form.Refresh()

		lay := u.gen.logoHolder.Layout.(layout.CustomPaddedLayout)
		lay.TopPadding -= 48
		lay.LeftPadding += 24
		lay.RightPadding += 24
		u.gen.logoHolder.Layout = lay
		u.gen.logoHolder.Refresh()
	}

	users := u.users()
	u.gen.shutdown.SetIcon(theme.NewThemedResource(resourcePowerSvg))
	u.gen.logo.Resource = resourceFyshPng

	var avatars []fyne.CanvasObject
	for _, name := range users {
		ava := newAvatar(name, func(user string) {
			for _, a := range avatars {
				border := a.(*fyne.Container).Objects[0].(*fyne.Container).Objects[4].(*canvas.Rectangle)
				border.StrokeColor = theme.Color(theme.ColorNameShadow)
				border.Refresh()
			}
			u.user = user
			u.updateForUsername(user)
			u.gen.win.Canvas().Focus(u.pass)
		})
		avatars = append(avatars, ava)
	}
	u.gen.avatars.Objects = avatars

	matched := false
	storedName := u.pref.String(prefUserKey)
	if storedName == "" && len(users) == 1 {
		storedName = users[0]
	}
	for i, name := range users {
		if name != storedName {
			continue
		}

		avatars[i].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*widget.Button).Tapped(&fyne.PointEvent{})
		matched = true
	}

	fyne.CurrentApp().Settings().AddListener(func(s fyne.Settings) {
		settingsListener(s, u.gen.bg, u.gen.box, u.currentHome())
	})
	settingsListener(fyne.CurrentApp().Settings(), u.gen.bg, u.gen.box, u.currentHome())
	if matched {
		u.gen.win.Canvas().Focus(u.pass)
		u.updateForUsername(u.user)
	}
}

func (u *ui) sessionNames() []string {
	names := make([]string, len(u.sessions))
	for i, sess := range u.sessions {
		names[i] = sess.name
	}
	return names
}

func (u *ui) sessionExec() string {
	for _, sess := range u.sessions {
		if sess.name == u.session.Selected {
			return sess.exec
		}
	}
	return u.sessions[0].exec
}

// currentHome returns the home directory of the currently selected user, or ""
// if no user has been chosen yet.
func (u *ui) currentHome() string {
	if u.user == "" {
		return ""
	}
	home, _ := homedir(u.user)
	return home
}

func (u *ui) updateForUsername(user string) {
	home, _ := homedir(user)
	if _, err := os.Stat(filepath.Join(home, ".xinitrc")); err != nil {
		if len(u.sessions) > 0 && u.sessions[len(u.sessions)-1] == xinitSession {
			u.sessions = u.sessions[:len(u.sessions)-1]
			u.session.Options = u.sessionNames()
			u.session.Refresh()
		}
	} else {
		if len(u.sessions) > 0 && u.sessions[len(u.sessions)-1] != xinitSession {
			u.sessions = append(u.sessions, xinitSession)
			u.session.Options = u.sessionNames()
			u.session.Refresh()
		}
	}

	last := u.pref.String(fmt.Sprintf(prefSessionKey, user))
	if last != "" {
		u.session.SetSelected(last)
	}
	updateBackground(u.gen.bg, fyne.CurrentApp().Settings(), home)
}

func boxBackgroundColor(s fyne.Settings) color.Color {
	bgCol := s.Theme().Color("fynedeskPanelBackground", s.ThemeVariant())
	if bgCol == nil || bgCol == color.Transparent {
		r, g, b, _ := theme.Color(theme.ColorNameBackground).RGBA()
		bgCol = color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xdd}
	}
	return bgCol
}

func getUsers() []string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		fyne.LogError("Failed to read password", err)
		return []string{""}
	}

	var ret []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "nologin") || strings.Contains(line, "/var/empty") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 7 || fields[0] == "root" || fields[6][len(fields[6])-2:] != "sh" {
			continue
		}
		ret = append(ret, fields[0])
	}
	return ret
}

func newAvatar(user string, f func(string)) fyne.CanvasObject {
	ava := canvas.NewImageFromResource(theme.AccountIcon())
	home, _ := homedir(user)
	facePath := filepath.Join(home, ".face")
	if _, err := os.Stat(facePath); err == nil {
		ava = canvas.NewImageFromFile(facePath)
	}
	ava.SetMinSize(fyne.NewSize(112, 112))
	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = theme.InputRadiusSize()
	border.StrokeWidth = theme.InputBorderSize()
	border.StrokeColor = theme.Color(theme.ColorNameShadow)

	tapper := widget.NewButton("", func() {
		f(user)
		border.StrokeColor = theme.Color(theme.ColorNamePrimary)
		border.Refresh()
	})
	tapper.Importance = widget.LowImportance

	bg := canvas.NewRectangle(theme.Color(theme.ColorNameButton))
	bg.CornerRadius = theme.InputRadiusSize()
	clipper := canvas.NewRectangle(color.Transparent)
	clipper.StrokeWidth = theme.InputRadiusSize() * 1.25
	clipper.StrokeColor = theme.Color(theme.ColorNameOverlayBackground)
	clipper.CornerRadius = theme.InputRadiusSize() * 2
	negativePad := theme.InputRadiusSize() * -.75
	img := container.NewStack(bg, tapper, ava, container.New(layout.NewCustomPaddedLayout(
		negativePad, negativePad, negativePad, negativePad,
	), clipper), border)
	return container.NewVBox(
		img,
		widget.NewLabelWithStyle(user, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)
}

func settingsListener(s fyne.Settings, c *fyne.Container, box *canvas.Rectangle, home string) {
	updateBackground(c, s, home)

	box.FillColor = boxBackgroundColor(s)
	box.Refresh()
}

func updateBackground(c *fyne.Container, s fyne.Settings, home string) {
	pref := fyne.CurrentApp().Preferences()
	configured := pref.String("background")
	fill := pref.StringWithFallback("backgroundfill", "Stretch")
	colorHex := pref.StringWithFallback("backgroundcolor", "#000000")

	if home != "" { // try and get user BG
		userPath := home + "/.config/fyne/com.fyshos.tyde/preferences.json"
		log.Println(userPath)
		if f, err := os.Open(userPath); err == nil {
			var data map[string]interface{}
			err := json.NewDecoder(f).Decode(&data)
			if err == nil {
				if bg, ok := data["background"].(string); ok {
					configured = bg
				}
				if v, ok := data["backgroundfill"].(string); ok {
					fill = v
				}
				if v, ok := data["backgroundcolor"].(string); ok {
					colorHex = v
				}
			}

			f.Close()
		}
	}

	var bg fyne.CanvasObject
	if configured != "" {
		if stat, err := os.Stat(configured); err == nil && stat.Mode().IsRegular() {
			img := canvas.NewImageFromFile(configured)
			img.ScaleMode = canvas.ImageScaleFastest
			img.FillMode = backgroundFillMode(fill)

			rect := canvas.NewRectangle(parseHexColor(colorHex))
			bg = container.NewStack(rect, img)
		}
	}
	if bg == nil {
		b := backgrounds.Default()
		bg = b.Load(s.Theme(), s.ThemeVariant())
	}

	c.Objects[0] = bg
	c.Refresh()
}

// backgroundFillMode maps a Tyde fill name to a canvas fill mode, matching the
// "Stretch"/"Fit"/"Fill" options exposed by the Tyde settings screen.
func backgroundFillMode(name string) canvas.ImageFill {
	switch name {
	case "Fit":
		return canvas.ImageFillContain
	case "Fill":
		return canvas.ImageFillCover
	default: // "Stretch"
		return canvas.ImageFillStretch
	}
}

// parseHexColor turns a "#rrggbb" or "#rrggbbaa" string into a colour,
// falling back to opaque black for empty or malformed input. This mirrors the
// background colour stored by Tyde.
func parseHexColor(hex string) color.NRGBA {
	c := color.NRGBA{A: 0xff}
	if len(hex) == 0 || hex[0] != '#' {
		return c
	}

	switch len(hex) {
	case 7: // #rrggbb
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &c.R, &c.G, &c.B); err != nil {
			return color.NRGBA{A: 0xff}
		}
	case 9: // #rrggbbaa
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A); err != nil {
			return color.NRGBA{A: 0xff}
		}
	}
	return c
}
