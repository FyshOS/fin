package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/background-light.png
var backgroundLight []byte
//go:embed assets/background-dark.png
var backgroundDark []byte
//go:embed assets/power.svg
var powerSVG []byte

var resourceBackgroundDarkPng = &fyne.StaticResource{
	StaticName: "background-dark.png",
	StaticContent: backgroundDark,
}
var resourceBackgroundLightPng = &fyne.StaticResource{
	StaticName: "background-light.png",
	StaticContent: backgroundLight,
}
var resourcePowerSvg = &fyne.StaticResource{
	StaticName: "power.svg",
	StaticContent: powerSVG,
}
