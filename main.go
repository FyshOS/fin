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

// xVTNR is the virtual terminal fin starts the X server on when there is no
// boot splash to take over from.
const xVTNR = 5

const splashHandoverTimeout = 10 * time.Second

var askShutdown, doLogin func()

func init() {
	runtime.LockOSThread()
}

func main() {
	logger := openLogWriter()
	log.SetOutput(logger)
	log.Println("Fin started")

	var xPID int
	splash := newPlymouth()
	display := os.Getenv("DISPLAY")
	if display == "" {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM)
		go func() {
			for {
				<-sig
				splash.finish(false)
				stopX(xPID)
			}
		}()

		// Take over the VT the splash is on, if plymouth running.
		vt := xVTNR
		if splashVT, ok := splash.vt(); ok {
			vt = splashVT
		}
		splash.deactivate()

		log.Println("Starting X on vt", vt)
		xPID = startX(vt, splash)
		_ = os.Setenv("DISPLAY", ":0")

		// We own the seat/VT, so tell the PAM login (via pam_systemd).
		_ = os.Setenv("XDG_SEAT", "seat0")
		_ = os.Setenv("XDG_VTNR", strconv.Itoa(vt))
	}

	a := app.NewWithID("com.fyshos.fin")

	// Hand over once the greeter is actually on screen.
	a.Lifecycle().SetOnEnteredForeground(func() {
		splash.finish(true)
	})
	time.AfterFunc(splashHandoverTimeout, func() {
		splash.finish(true)
	})

	g := newGUI()
	w := g.makeWindow(a)

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

	// fallback to clear the screen if we didn't succeeed in taking over screen.
	splash.finish(false)

	if xPID != 0 {
		log.Println("Stopping X")
		stopX(xPID)
	}
}

func startX(vt int, splash *plymouth) int {
	cmd := fmt.Sprintf("X :0 vt%02d", vt)
	exe := exec.Command("/bin/sh", "-c", cmd)
	err := exe.Start()
	if err != nil {
		// Drop the splash without retaining it, so the console comes back and
		// the failure is not hidden behind a frozen boot image.
		splash.finish(false)

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
