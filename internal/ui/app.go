package ui

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

// App holds the window, the stored groups and whatever the current screen is.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	store   *group.Store
	groups  []group.Group
	live    *session.Host
}

// Run starts the interface and blocks until the window closes.
func Run() {
	a := fyneapp.NewWithID("com.desynkro.lanparty")
	a.Settings().SetTheme(NewTheme())

	store := group.OpenStore(filepath.Join(config.DataDir(), "groups.json"))
	groups, err := store.Load()
	if err != nil {
		log.Printf("lan-party: %v", err)
	}

	app := &App{fyneApp: a, store: store, groups: groups}
	app.win = a.NewWindow("LAN Party")
	app.win.SetCloseIntercept(func() {
		if app.live != nil {
			_ = app.live.Close()
			app.live = nil
		}
		app.win.Close()
	})
	app.showHome()

	app.win.Resize(fyne.NewSize(420, 620))
	app.win.CenterOnScreen()
	app.win.ShowAndRun()
}

func defaultHostName() string {
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return "Host"
}
