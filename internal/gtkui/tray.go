package gtkui

import (
	"log"
	"os"

	"fyne.io/systray"
	"github.com/diamondburned/gotk4/pkg/core/glib"
)

func startTray(u *probeUI) bool {
	if !u.trayAvailable {
		log.Println("lan-party: no StatusNotifier host; tray disabled")
		return false
	}

	icon, err := os.ReadFile(trayIconPath(u.root))
	if err != nil {
		log.Printf("lan-party: tray icon: %v", err)
		return false
	}

	// The Windows tray creates a hidden Win32 window and pumps its message
	// loop. Win32 windows are thread-affine, so keep this goroutine on one OS
	// thread; otherwise Go can move it and the tray stops receiving clicks.
	go func() {
		unlock := lockTrayThread()
		defer unlock()
		systray.Run(func() {
			log.Println("lan-party: tray ready")
			systray.SetIcon(icon)
			systray.SetTitle("LAN Party")
			systray.SetTooltip("LAN Party")
			systray.SetOnTapped(func() {
				log.Println("lan-party: tray tapped")
				glib.IdleAdd(u.reveal)
			})

			show := systray.AddMenuItem("Show LAN Party", "")
			settings := systray.AddMenuItem("Settings", "")
			systray.AddSeparator()
			quit := systray.AddMenuItem("Quit", "")

			go func() {
				for range show.ClickedCh {
					log.Println("lan-party: tray show clicked")
					glib.IdleAdd(u.reveal)
				}
			}()
			go func() {
				for range settings.ClickedCh {
					log.Println("lan-party: tray settings clicked")
					glib.IdleAdd(u.openSettings)
				}
			}()
			go func() {
				for range quit.ClickedCh {
					log.Println("lan-party: tray quit clicked")
					glib.IdleAdd(u.quit)
				}
			}()
		}, func() {
			log.Println("lan-party: tray stopped")
		})
	}()

	return true
}

func systrayQuit() {
	systray.Quit()
}
