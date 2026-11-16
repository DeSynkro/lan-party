package ui

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"github.com/desynkro/lan-party/internal/buildinfo"
)

// Run starts the interface. It blocks until the window closes.
func Run() {
	a := fyneapp.NewWithID("com.desynkro.lanparty")
	a.Settings().SetTheme(NewTheme())

	w := a.NewWindow("LAN Party")
	w.SetContent(newHome(w, buildinfo.Summary()))
	w.Resize(fyne.NewSize(420, 620))
	w.CenterOnScreen()
	w.ShowAndRun()
}
