package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/icon.png
var iconPNG []byte

// appIcon is the mark used for the window and the tray.
var appIcon = &fyne.StaticResource{
	StaticName:    "lan-party.png",
	StaticContent: iconPNG,
}
