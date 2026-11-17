package gtkui

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/systray"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/godbus/dbus/v5"
)

// statusNotifierAvailable checks for the host that owns tray icons on Linux.
// Without it, hiding the window on close would strand the app.
func statusNotifierAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var owner bool
	err = conn.BusObject().Call(
		"org.freedesktop.DBus.NameHasOwner",
		0,
		"org.kde.StatusNotifierWatcher",
	).Store(&owner)
	return err == nil && owner
}

func startTray(u *probeUI) bool {
	if !u.trayAvailable {
		log.Println("gtk probe: no StatusNotifier host; tray disabled")
		return false
	}

	icon, err := os.ReadFile(filepath.Join(u.root, "brand", "png", "tray-icon-color-24.png"))
	if err != nil {
		log.Printf("gtk probe: tray icon: %v", err)
		return false
	}

	go systray.Run(func() {
		log.Println("gtk probe: tray ready")
		systray.SetIcon(icon)
		systray.SetTitle("LAN Party")
		systray.SetTooltip("LAN Party")
		systray.SetOnTapped(func() {
			glib.IdleAdd(u.reveal)
		})

		show := systray.AddMenuItem("Show LAN Party", "")
		settings := systray.AddMenuItem("Settings", "")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "")

		go func() {
			for range show.ClickedCh {
				glib.IdleAdd(u.reveal)
			}
		}()
		go func() {
			for range settings.ClickedCh {
				glib.IdleAdd(u.openSettings)
			}
		}()
		go func() {
			for range quit.ClickedCh {
				glib.IdleAdd(u.quit)
			}
		}()
	}, func() {
		log.Println("gtk probe: tray stopped")
	})

	return true
}

func systrayQuit() {
	systray.Quit()
}
