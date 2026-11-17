package ui

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	windowWidth  = 400
	windowHeight = 600
)

// Run starts the application window. The skeleton is replaced screen by screen
// as the app grows.
func Run() {
	a := fyneapp.NewWithID("com.desynkro.lanparty")
	w := a.NewWindow("LAN Party")
	w.SetContent(container.NewCenter(widget.NewLabel("LAN Party")))
	w.Resize(fyne.NewSize(windowWidth, windowHeight))
	w.SetFixedSize(true)
	w.CenterOnScreen()
	w.ShowAndRun()
}
