package main

import (
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

const (
	prefSessionKey = "user.%s.session"
	prefUserKey    = "default.user"
)

type ui struct {
	win     fyne.Window
	pass    *widget.Entry
	session *widget.Select
	err     *canvas.Text

	hostname func() string
	user     string
	users    func() []string
	sessions []*session
	pref     fyne.Preferences
}

func newUI(w fyne.Window, p fyne.Preferences, host func() string, users func() []string) *ui {
	return &ui{win: w, hostname: host, pref: p, sessions: loadSessions(), users: users}
}

func (u *ui) askShutdown() {
	var pop *widget.PopUp
	message := widget.NewLabel("Are you sure you want to power off your computer?")
	message.Alignment = fyne.TextAlignCenter

	buttons := container.NewGridWithColumns(3,
		widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
			pop.Hide()
		}),
		widget.NewButtonWithIcon("Reboot", theme.ViewRefreshIcon(), func() {
			pop.Hide()
			_ = exec.Command("shutdown", "-r", "now").Start()
		}),
		container.NewMax(newButtonBackground(theme.ErrorColor()),
			widget.NewButtonWithIcon("Power off", theme.NewThemedResource(resourcePowerSvg), func() {
				pop.Hide()
				_ = exec.Command("shutdown", "-h", "now").Start()
			})))
	body := container.NewVBox(message, container.NewCenter(buttons))

	title := widget.NewLabelWithStyle("Shutdown", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	prop := canvas.NewRectangle(color.Transparent)
	prop.SetMinSize(body.MinSize().Add(fyne.NewSize(32, 16))) // pad to match dialog
	content := container.NewVBox(title, container.NewMax(prop, body))

	pop = widget.NewModalPopUp(content, u.win.Canvas())
	pop.Show()
}

func (u *ui) doLogin() {
	if u.user == "" || u.pass.Text == "" {
		u.setError("Missing username or password")
		return
	}
	u.pref.SetString(fmt.Sprintf(prefSessionKey, u.user), u.session.Selected)
	u.pref.SetString(prefUserKey, u.user)

	go func() {
		pid, err := login(u.user, u.pass.Text, u.sessionExec())
		if err != nil {
			u.setError(err.Error())
			return
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			u.setError(err.Error())
			u.win.Show()
			return
		}

		u.win.Hide()
		_, _ = proc.Wait()

		u.win.Show()
		_ = logout()
		u.pass.SetText("")
		u.win.Canvas().Focus(u.pass)
		u.setError("")
	}()
}

func (u *ui) setError(err string) {
	u.err.Text = err
	u.err.Refresh()
}

func (u *ui) loadUI() {
	u.pass = widget.NewPasswordEntry()
	u.pass.OnSubmitted = func(string) {
		u.win.Canvas().Focus(nil)
		u.doLogin()
	}
	u.session = widget.NewSelect(u.sessionNames(), func(string) {})
	u.err = canvas.NewText("", theme.ErrorColor())
	u.err.Alignment = fyne.TextAlignCenter

	users := u.users()
	var formItems []*widget.FormItem
	if len(users) == 0 {
		user := widget.NewEntry()
		user.OnChanged = func(user string) {
			u.user = user
		}

		formItems = append(formItems, widget.NewFormItem("Username", user))
	}

	formItems = append(formItems,
		widget.NewFormItem("Password", u.pass),
		widget.NewFormItem("Session", u.session))
	f := widget.NewForm(formItems...)
	login := widget.NewButtonWithIcon("Log In", theme.LoginIcon(), u.doLogin)
	login.Importance = widget.HighImportance
	buttons := container.NewGridWithColumns(2,
		widget.NewButtonWithIcon("Shutdown", theme.NewThemedResource(resourcePowerSvg), u.askShutdown),
		login)

	bg := canvas.NewImageFromResource(background)
	bgCol := fyne.CurrentApp().Settings().Theme().Color(
		"fynedeskPanelBackground",
		fyne.CurrentApp().Settings().ThemeVariant())
	if bgCol == nil || bgCol == color.Transparent {
		r, g, b, _ := theme.BackgroundColor().RGBA()
		bgCol = color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xdd}
	}
	box := canvas.NewRectangle(bgCol)

	var avatars []fyne.CanvasObject
	for _, name := range users {
		ava := newAvatar(name, func(user string) {
			for _, a := range avatars {
				border := a.(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*canvas.Rectangle)
				border.StrokeColor = theme.ShadowColor()
				border.Refresh()
			}
			u.user = user
			u.updateForUsername(user)
			u.win.Canvas().Focus(u.pass)
		})
		avatars = append(avatars, ava)
	}

	u.win.SetContent(container.NewMax(bg,
		container.NewCenter(container.NewMax(box, container.NewVBox(
			widget.NewLabelWithStyle("Log in to "+u.hostname(), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),

			container.NewMax(widget.NewLabel(""), u.err),
			container.NewCenter(container.NewHBox(avatars...)),
			container.NewBorder(nil, nil, widget.NewLabel("     "), widget.NewLabel("     "),
				container.NewVBox(f, buttons)),
			widget.NewLabel(""),
		))),
	))

	matched := false
	storedName := u.pref.String(prefUserKey)
	for i, name := range users {
		if name != storedName {
			continue
		}

		avatars[i].(*fyne.Container).Objects[0].(*fyne.Container).Objects[1].(*widget.Button).Tapped(&fyne.PointEvent{})
		matched = true
	}
	if matched {
		u.win.Canvas().Focus(u.pass)
	} else if len(users) == 0 {
		u.win.Canvas().Focus(formItems[0].Widget.(*widget.Entry))
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

func (u *ui) updateForUsername(user string) {
	home, _ := homedir(user)
	if _, err := os.Stat(filepath.Join(home, ".xinitrc")); err != nil {
		if u.sessions[len(u.sessions)-1] == xinitSession {
			u.sessions = u.sessions[:len(u.sessions)-1]
			u.session.Options = u.sessionNames()
			u.session.Refresh()
		}
	} else {
		if u.sessions[len(u.sessions)-1] != xinitSession {
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
	data, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		fyne.LogError("Failed to read password", err)
		return []string{""}
	}

	var ret []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "nologin") {
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
	ava.SetMinSize(fyne.NewSize(120, 120))
	border := canvas.NewRectangle(theme.InputBackgroundColor())
	border.StrokeWidth = theme.InputBorderSize()
	border.StrokeColor = theme.ShadowColor()

	tapper := widget.NewButton("", func() {
		f(user)
		border.StrokeColor = theme.PrimaryColor()
		border.Refresh()
	})
	tapper.Importance = widget.LowImportance

	img := container.NewMax(border, tapper, ava)
	return container.NewVBox(img,
		widget.NewLabelWithStyle(user, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)
}
