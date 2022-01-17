package main

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
)

var xinitSession = &session{
	name: ".xinitrc",
	exec: "/bin/bash --login .xinitrc",
}

type session struct {
	name, exec string
}

func loadSessions() []*session {
	var list []*session
	for _, dir := range lookupXdgDataDirs() {
		sessionDir := filepath.Join(dir, "xsessions")
		files, err := ioutil.ReadDir(sessionDir)
		if err != nil {
			continue
		}
		for _, file := range files {
			list = append(list, loadSession(filepath.Join(sessionDir, file.Name())))
		}

	}
	return list
}

func loadSession(path string) *session {
	file, err := os.Open(path)
	if err != nil {
		fyne.LogError("Could not open session file", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	data := session{}
	var currentSection string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[") {
			currentSection = line
		}
		if currentSection != "[Desktop Entry]" {
			continue
		}
		if strings.HasPrefix(line, "Name=") {
			name := strings.SplitAfter(line, "=")
			data.name = name[1]
		} else if strings.HasPrefix(line, "Exec=") {
			exec := strings.SplitAfter(line, "=")
			data.exec = exec[1]
		}
	}
	if err := scanner.Err(); err != nil {
		fyne.LogError("Could not read file", err)
		return nil
	}
	return &data
}

// lookupXdgDataDirs returns a string slice of all XDG_DATA_DIRS
func lookupXdgDataDirs() []string {
	var dirs []string
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataHome = filepath.Join(home, ".local/share")
		}
	}
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share/:/usr/share/"
	}
	dirs = append(strings.Split(dataHome, ":"), strings.Split(dataDirs, ":")...)
	return dirs
}
