package main // import "fyshos.com/fin"

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/FyshOS/dryvers"
)

// xVTNR is the virtual terminal fin starts the X server on.
const xVTNR = 5

var askShutdown, doLogin func()

func init() {
	runtime.LockOSThread()
}

func main() {
	logger := openLogWriter()
	log.SetOutput(logger)
	log.Println("Fin started")

	var xPID int
	display := os.Getenv("DISPLAY")
	if display == "" {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM)
		go func() {
			for {
				<-sig
				stopX(xPID)
			}
		}()

		log.Println("Starting X")
		xPID = startX()
		_ = os.Setenv("DISPLAY", ":0")

		// We own the seat/VT, so tell the PAM login (via pam_systemd).
		_ = os.Setenv("XDG_SEAT", "seat0")
		_ = os.Setenv("XDG_VTNR", strconv.Itoa(xVTNR))
	}

	a := app.NewWithID("com.fyshos.fin")
	w := a.NewWindow("Fin")
	g := newGUI()
	g.win = w
	w.Resize(fyne.NewSize(771, 476))
	w.SetPadded(false)
	w.SetContent(g.makeUI())

	ui := newUI(g, a.Preferences(), getUsers)
	bright := dryvers.NewBrightness()
	w.Canvas().SetOnTypedKey(brightnessFunc(bright))
	askShutdown = func() {
		ui.askShutdown()
	}
	doLogin = func() {
		ui.doLogin()
	}

	if display == "" {
		w.SetFullScreen(true)
	} else {
		w.Resize(fyne.NewSize(1280, 720))
	}
	ui.loadUI(bright)
	w.ShowAndRun()

	if xPID != 0 {
		log.Println("Stopping X")
		stopX(xPID)
	}
}

func startX() int {
	cmd := fmt.Sprintf("X :0 vt%02d", xVTNR)
	exe := exec.Command("/bin/sh", "-c", cmd)
	err := exe.Start()
	if err != nil {
		fyne.LogError("Could not start X server", err)
		os.Exit(1)
	}

	time.Sleep(time.Second)
	return exe.Process.Pid
}

func stopX(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		fyne.LogError("Could not find X server pid", err)
	}

	_ = p.Kill()
}

func (g *gui) shutdownTapped() {
	askShutdown()
}

func (g *gui) loginTapped() {
	doLogin()
}
