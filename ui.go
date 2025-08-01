//go:generate fyne bundle -o bundled.go assets

package main

import (
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

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

const (
	prefSessionKey = "user.%s.session"
	prefUserKey    = "default.user"
)

type ui struct {
	gen     *gui
	pass    *widget.Entry
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
	u.pref.SetString(fmt.Sprintf(prefSessionKey, u.user), u.session.Selected)
	u.pref.SetString(prefUserKey, u.user)

	go func() {
		pid, err := login(u.user, u.pass.Text, u.sessionExec())
		if err != nil {
			dialog.ShowError(err, u.gen.win)
			return
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			dialog.ShowError(err, u.gen.win)
			u.gen.win.Show()
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

		u.gen.win.Hide()
		_, _ = proc.Wait()

		u.gen.win.Show()
		_ = logout()
		u.pass.SetText("")
		u.gen.win.Canvas().Focus(u.pass)

		// OpenBSD: give device ownership back to root
		if runtime.GOOS == "openbsd" {
			_ = os.Chown("/dev/console", 0, -1)
			_ = os.Chown("/dev/dri/card0", 0, -1)
			_ = os.Chown("/dev/dri/renderD128", 0, -1)
		}
	}()
}

func (u *ui) loadUI() {
	u.pass = u.gen.form.Items[0].Widget.(*widget.Entry)
	u.pass.OnSubmitted = func(string) {
		u.gen.win.Canvas().Focus(nil)
		u.doLogin()
	}
	u.session = u.gen.form.Items[1].Widget.(*widget.Select)
	u.session.Options = u.sessionNames()

	users := u.users()
	u.gen.shutdown.SetIcon(theme.NewThemedResource(resourcePowerSvg))
	u.gen.logo.Resource = resourceFyshPng

	var avatars []fyne.CanvasObject
	for _, name := range users {
		ava := newAvatar(name, func(user string) {
			for _, a := range avatars {
				border := a.(*fyne.Container).Objects[0].(*fyne.Container).Objects[4].(*canvas.Rectangle)
				border.StrokeColor = theme.ShadowColor()
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
	if matched {
		u.gen.win.Canvas().Focus(u.pass)
	}

	fyne.CurrentApp().Settings().AddListener(func(s fyne.Settings) {
		settingsListener(s, u.gen.bg, u.gen.box)
	})
	settingsListener(fyne.CurrentApp().Settings(), u.gen.bg, u.gen.box)
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
}

func boxBackgroundColor(s fyne.Settings) color.Color {
	bgCol := s.Theme().Color("fynedeskPanelBackground", s.ThemeVariant())
	if bgCol == nil || bgCol == color.Transparent {
		r, g, b, _ := theme.BackgroundColor().RGBA()
		bgCol = color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xdd}
	}
	return bgCol
}

func getScreenSize() (uint16, uint16) {
	conn, err := xgb.NewConn()
	if err != nil {
		log.Println("ScreenSize X connect error", err)
		return 1280, 720
	}
	err = randr.Init(conn)
	if err != nil {
		log.Println("ScreenSize X RandR error", err)
		return 1280, 720
	}

	root := xproto.Setup(conn).DefaultScreen(conn).Root
	resources, _ := randr.GetScreenResources(conn, root).Reply()

	// Get first connected output
	// TODO: Consider multiple connected outputs in multihead mode
	var crtcInfo *randr.GetCrtcInfoReply
	for _, v := range resources.Outputs {
		output, _ := randr.GetOutputInfo(conn, v, 0).Reply()
		// 0 = "connected", 1 = "disconnected, 2 = "unknown"
		if output.Connection == 0 {
			crtcInfo, _ = randr.GetCrtcInfo(conn, output.Crtc, 0).Reply()
			break
		}
	}

	return crtcInfo.Width, crtcInfo.Height
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
	border.StrokeColor = theme.ShadowColor()

	tapper := widget.NewButton("", func() {
		f(user)
		border.StrokeColor = theme.PrimaryColor()
		border.Refresh()
	})
	tapper.Importance = widget.LowImportance

	bg := canvas.NewRectangle(theme.ButtonColor())
	bg.CornerRadius = theme.InputRadiusSize()
	clipper := canvas.NewRectangle(color.Transparent)
	clipper.StrokeWidth = theme.InputRadiusSize() * 1.25
	clipper.StrokeColor = theme.OverlayBackgroundColor()
	clipper.CornerRadius = theme.InputRadiusSize() * 2
	negativePad := theme.InputRadiusSize() * -.75
	img := container.NewStack(bg, tapper, ava, container.New(layout.NewCustomPaddedLayout(
		negativePad, negativePad, negativePad, negativePad), clipper), border)
	return container.NewVBox(img,
		widget.NewLabelWithStyle(user, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)
}

func settingsListener(s fyne.Settings, c *fyne.Container, box *canvas.Rectangle) {
	b := backgrounds.Default()
	bg := b.Load(s.Theme(), s.ThemeVariant())
	c.Objects[0] = bg
	c.Refresh()

	box.FillColor = boxBackgroundColor(s)
	box.Refresh()
}
