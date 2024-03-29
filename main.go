package main // import "fyshos.com/fin"

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

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
	}

	a := app.NewWithID("com.fyshos.fin")
	w := a.Driver().CreateWindow("Fin")
	ui := newUI(w, a.Preferences(), getUsers)
	w.SetPadded(false)

	if display == "" {
		screenW, screenH := getScreenSize()
		go func() {
			tries := 0
			scale := float32(1.0)
			for scale == float32(1.0) && tries < 50 {
				time.Sleep(time.Millisecond * 100) // TODO use lifecycle to resize this at the correct time
				scale = w.Canvas().Scale()
				tries++
			}
			w.Resize(fyne.NewSize(float32(screenW)/scale, float32(screenH)/scale))
		}()
		w.Resize(fyne.NewSize(float32(screenW), float32(screenH)))
		ui.loadUI()
	} else {
		ui.loadUI()
		w.Resize(fyne.NewSize(1280, 720))
	}
	w.ShowAndRun()

	if xPID != 0 {
		log.Println("Stopping X")
		stopX(xPID)
	}
}

func startX() int {
	cmd := "X :0 vt05"
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
