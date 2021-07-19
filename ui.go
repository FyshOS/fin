package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
)

const (
	prefSessionKey = "user.%s.session"
	prefUserKey    = "default.user"
)

type ui struct {
	win        fyne.Window
	user, pass *widget.Entry
	session    *widget.Select
	err        *canvas.Text

	hostname func() string
	sessions []*session
	pref     fyne.Preferences
}

func newUI(w fyne.Window, p fyne.Preferences, host func() string) *ui {
	return &ui{win: w, hostname: host, pref: p, sessions: loadSessions()}
}

func (u *ui) askShutdown() {
	dialog.ShowConfirm("Shutdown", "Are you sure you want to shut down?",
		func(ok bool) {
			if !ok {
				return
			}

			cmd := exec.Command("shutdown", "-h", "now")
			_ = cmd.Start()
		}, u.win)
}

func (u *ui) doLogin() {
	if u.user.Text == "" || u.pass.Text == "" {
		u.setError("Missing username or password")
		return
	}
	u.pref.SetString(fmt.Sprintf(prefSessionKey, u.user.Text), u.session.Selected)
	u.pref.SetString(prefUserKey, u.user.Text)

	go func() {
		pid, err := login(u.user.Text, u.pass.Text, u.sessionExec())
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
	u.user = widget.NewEntry()
	u.user.OnChanged = u.updateForUsername
	u.pass = widget.NewPasswordEntry()
	u.pass.OnSubmitted = func(string) {
		u.win.Canvas().Focus(nil)
		u.doLogin()
	}
	u.session = widget.NewSelect(u.sessionNames(), func(string) {})
	u.err = canvas.NewText("", theme.ErrorColor())
	u.err.Alignment = fyne.TextAlignCenter

	f := widget.NewForm(
		widget.NewFormItem("Username", u.user),
		widget.NewFormItem("Password", u.pass),
		widget.NewFormItem("Session", u.session))
	f.SubmitText = "Log In"
	f.CancelText = "Shutdown"
	f.OnCancel = u.askShutdown
	f.OnSubmit = u.doLogin

	bg := canvas.NewImageFromResource(background)
	r, g, b, _ := theme.BackgroundColor().RGBA()
	box := canvas.NewRectangle(color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xdd})

	u.win.SetContent(container.NewMax(bg,
		container.NewCenter(container.NewMax(box, container.NewVBox(
			widget.NewLabelWithStyle(fmt.Sprintf("Log in to %s", u.hostname()), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),

			container.NewMax(widget.NewLabel(""), u.err),
			container.NewBorder(nil, nil, widget.NewLabel("     "), widget.NewLabel("     "), f),
			widget.NewLabel(""),
		))),
	))

	u.user.SetText(u.pref.String(prefUserKey))
	if len(u.user.Text) == 0 {
		u.win.Canvas().Focus(u.user)
	} else {
		u.win.Canvas().Focus(u.pass)
	}
}

func (u *ui) sessionNames() []string {
	var ret []string
	for _, sess := range u.sessions {
		ret = append(ret, sess.name)
	}
	return ret
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
	conn, err := xgbutil.NewConn()
	if err != nil {
		log.Println("ScreenSize X connect error", err)
		return 1280, 720
	}
	err = randr.Init(conn.Conn())
	if err != nil {
		log.Println("ScreenSize X RandR error", err)
		return 1280, 720
	}

	root := xproto.Setup(conn.Conn()).DefaultScreen(conn.Conn()).Root
	resources, _ := randr.GetScreenResources(conn.Conn(), root).Reply()
	output := resources.Outputs[0]
	outputInfo, _ := randr.GetOutputInfo(conn.Conn(), output, 0).Reply()

	crtcInfo, _ := randr.GetCrtcInfo(conn.Conn(), outputInfo.Crtc, 0).Reply()
	return crtcInfo.Width, crtcInfo.Height
}
