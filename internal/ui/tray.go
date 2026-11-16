package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// setupTray puts a menu in the system tray. It reports false when the platform
// has no tray, so the window close behaviour can fall back to quitting.
func (a *App) setupTray() bool {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return false
	}

	desk.SetSystemTrayIcon(appIcon)
	desk.SetSystemTrayMenu(fyne.NewMenu("LAN Party",
		fyne.NewMenuItem("Show", func() {
			a.win.Show()
			a.win.RequestFocus()
		}),
		fyne.NewMenuItem("Settings", a.showSettings),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			a.closeSessions()
			a.fyneApp.Quit()
		}),
	))

	return true
}
