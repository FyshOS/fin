package main

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
)

func logPath() string {
	cacheDir := filepath.Join(systemLogDir(), "fyne", "io.fyne.fin")
	err := os.MkdirAll(cacheDir, 0700)
	if err != nil {
		fyne.LogError("Could not create log directory", err)
	}

	return filepath.Join(cacheDir, "fin.log")
}

func openLogWriter() *os.File {
	path := logPath()
	f, err := os.Create(path)
	if err != nil {
		fyne.LogError("Unable to open log file", err)
		return os.Stderr
	}
	log.Println("Logging to", path)

	return f
}

func systemLogDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cache")
}
