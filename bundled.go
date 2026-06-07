package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/power.svg
var powerSVG []byte

//go:embed assets/fysh.png
var fyshPNG []byte

//go:embed assets/bg-dark.png
var bgDarkPNG []byte

var resourcePowerSvg = &fyne.StaticResource{
	StaticName:    "power.svg",
	StaticContent: powerSVG,
}

var resourceFyshPng = &fyne.StaticResource{
	StaticName:    "fysh.png",
	StaticContent: fyshPNG,
}

var resourceBgDarkPng = &fyne.StaticResource{
	StaticName:    "bg-dark.png",
	StaticContent: bgDarkPNG,
}
