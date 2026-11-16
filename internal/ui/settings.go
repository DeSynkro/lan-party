package ui

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/autostart"
)

const (
	prefStartMinimised = "startMinimised"
	prefStartOnBoot    = "startOnBoot"
)

// showSettings holds the two things that change how the app starts.
func (a *App) showSettings() {
	prefs := a.fyneApp.Preferences()

	minimised := widget.NewCheck("Start minimised", func(on bool) {
		prefs.SetBool(prefStartMinimised, on)
	})
	minimised.SetChecked(prefs.Bool(prefStartMinimised))

	boot := widget.NewCheck("Start when I log in", func(on bool) {
		a.setAutostart(on)
	})
	boot.SetChecked(prefs.Bool(prefStartOnBoot))

	body := container.NewVBox(
		minimised,
		boot,
		metaText("Closing the window keeps a live session running in the tray."),
	)

	d := dialog.NewCustom("Settings", "Close", body, a.win)
	d.Resize(fyne.NewSize(380, 200))
	d.Show()
}

func (a *App) setAutostart(on bool) {
	prefs := a.fyneApp.Preferences()

	if !on {
		if err := autostart.Disable(); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		prefs.SetBool(prefStartOnBoot, false)
		return
	}

	exec, err := os.Executable()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	if err := autostart.Enable(exec); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	prefs.SetBool(prefStartOnBoot, true)
}
