package gtkui

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/systray"
	"github.com/diamondburned/gotk4/pkg/core/glib"
)

func startTray(u *probeUI) bool {
	if !u.trayAvailable {
		log.Println("lan-party: no StatusNotifier host; tray disabled")
		return false
	}

	icon, err := os.ReadFile(filepath.Join(u.root, "brand", "png", "tray-icon-color-24.png"))
	if err != nil {
		log.Printf("lan-party: tray icon: %v", err)
		return false
	}

	go systray.Run(func() {
		log.Println("lan-party: tray ready")
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
		log.Println("lan-party: tray stopped")
	})

	return true
}

func systrayQuit() {
	systray.Quit()
}
