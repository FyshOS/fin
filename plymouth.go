package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// plymouth manages the handoff from the boot splash to the login greeter.
type plymouth struct {
	running bool
	once    sync.Once
}

// newPlymouth returns a handle to the running boot splash. Every method is a
// no-op when no splash is running or plymouth is missing.
func newPlymouth() *plymouth {
	return &plymouth{running: plymouthRun("--ping")}
}

// vt reports the virtual terminal the splash is drawn on.
func (p *plymouth) vt() (int, bool) {
	if !p.running || !plymouthRun("--has-active-vt") {
		return 0, false
	}

	active, err := os.ReadFile("/sys/class/tty/tty0/active")
	if err != nil {
		return 0, false
	}

	vt, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(string(active)), "tty"))
	if err != nil || vt <= 0 {
		return 0, false
	}
	return vt, true
}

// deactivate asks plymouth to drop DRM master and its console grab, leaving
// the splash on screen, so that the X server can claim the device.
func (p *plymouth) deactivate() {
	if !p.running {
		return
	}

	log.Println("Deactivating boot splash")
	plymouthRun("deactivate")
}

// finish stops the splash, and is safe to call more than once.
// This has to run on every exit path. Once fin declares itself the owner of the handoff,
// nothing else stops plymouthd.
func (p *plymouth) finish(retainSplash bool) {
	if !p.running {
		return
	}

	p.once.Do(func() {
		log.Println("Stopping boot splash, retaining image:", retainSplash)
		if retainSplash {
			plymouthRun("quit", "--retain-splash") // leave last frame visible
		} else {
			plymouthRun("quit")
		}
	})
}

// plymouthRun invokes the plymouth client, reporting whether it succeeded.
func plymouthRun(args ...string) bool {
	return exec.Command("plymouth", args...).Run() == nil
}
