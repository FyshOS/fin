package main

//go:generate fyne bundle -o bundled.go assets

var (
	// backgroundDark is the default background image for dark mode
	backgroundDark = resourceBackgroundDarkPng
	// backgroundLight is the default background image for light mode
	backgroundLight = resourceBackgroundLightPng
)
