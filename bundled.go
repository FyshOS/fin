package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)


//go:embed assets/power.svg
var powerSVG []byte
//go:embed assets/fysh.png
var fyshPNG []byte

var resourcePowerSvg = &fyne.StaticResource{
	StaticName: "power.svg",
	StaticContent: powerSVG,
}
var resourceFyshPng = &fyne.StaticResource{
	StaticName: "fysh.png",
	StaticContent: fyshPNG,
}