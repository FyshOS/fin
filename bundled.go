package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/bg-dark.svg
var bgDarkSVG []byte

//go:embed assets/bg-light.svg
var bgLightSVG []byte

//go:embed assets/power.svg
var powerSVG []byte

//go:embed assets/fysh.png
var fyshPNG []byte

var resourcePowerSvg = &fyne.StaticResource{
	StaticName:    "power.svg",
	StaticContent: powerSVG,
}

var resourceFyshPng = &fyne.StaticResource{
	StaticName:    "fysh.png",
	StaticContent: fyshPNG,
}

var resourceBgDark = &fyne.StaticResource{
	StaticName:    "bg-dark.svg",
	StaticContent: bgDarkSVG,
}

var resourceBgLight = &fyne.StaticResource{
	StaticName:    "bg-light.svg",
	StaticContent: bgLightSVG,
}
