package ui

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/session"
)

// App holds the window, the stored groups and whatever the current screen is.
type App struct {
	fyneApp     fyne.App
	win         fyne.Window
	store       *group.Store
	groups      []group.Group
	inviteStore *invite.Store
	joined      []invite.Invite

	live        *session.Host
	friend      *session.Friend
	memberViews map[string]*memberView
}

// Run starts the interface and blocks until the window closes.
func Run() {
	a := fyneapp.NewWithID("com.desynkro.lanparty")
	a.Settings().SetTheme(NewTheme())

	dataDir := config.DataDir()

	store := group.OpenStore(filepath.Join(dataDir, "groups.json"))
	groups, err := store.Load()
	if err != nil {
		log.Printf("lan-party: %v", err)
	}

	inviteStore := invite.OpenStore(filepath.Join(dataDir, "joined.json"))
	joined, err := inviteStore.Load()
	if err != nil {
		log.Printf("lan-party: %v", err)
	}

	app := &App{
		fyneApp:     a,
		store:       store,
		groups:      groups,
		inviteStore: inviteStore,
		joined:      joined,
	}
	app.win = a.NewWindow("LAN Party")
	app.win.SetCloseIntercept(func() {
		app.closeSessions()
		app.win.Close()
	})
	app.showHome()
	app.startPresenceTimer()

	app.win.Resize(fyne.NewSize(420, 620))
	app.win.CenterOnScreen()
	app.win.ShowAndRun()
}

func (a *App) closeSessions() {
	if a.live != nil {
		_ = a.live.Close()
		a.live = nil
	}
	if a.friend != nil {
		_ = a.friend.Close()
		a.friend = nil
	}
}

func defaultHostName() string {
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return "Host"
}
