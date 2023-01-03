package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
)

func logPath() string {
	cacheDir := filepath.Join(systemLogDir(), "fyne", "com.fyshos.fin")
	err := os.MkdirAll(cacheDir, 0700)
	if err != nil {
		fyne.LogError("Could not create log directory", err)
	}

	return filepath.Join(cacheDir, "fin.log")
}

func openLogWriter() *os.File {
	path := logPath()
	rotateLog(path)

	f, err := os.Create(path)
	if err != nil {
		fyne.LogError("Unable to open log file", err)
		return os.Stderr
	}
	log.Println("Logging to", path)

	return f
}

func rotateLog(path string) {
	if _, err := os.Stat(path); err != nil && errors.Is(err, os.ErrNotExist) {
		return
	}

	crashPath := rotatedLogPath(path)
	err := os.Rename(path, crashPath)
	if err != nil {
		fyne.LogError("Could not save crash file: "+crashPath, err)
	}
}

func rotatedLogPath(path string) string {
	parent := filepath.Dir(path)
	now := time.Now().Format(time.RFC3339)
	return filepath.Join(parent, fmt.Sprintf("fin-%s.log", now))
}

func systemLogDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cache")
}
