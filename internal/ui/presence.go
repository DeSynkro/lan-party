package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"time"
)

// memberView is the part of a roster row that changes while a session runs.
type memberView struct {
	dot    *canvas.Text
	status *canvas.Text
}

// refreshPresence redraws the roster from what WireGuard knows. It is called on
// a timer, so it has to tolerate there being no session.
func (a *App) refreshPresence() {
	presence := map[string]time.Time{}
	if a.live != nil {
		presence = a.live.Presence()
	}

	now := time.Now()
	for key, view := range a.memberViews {
		last := presence[key]

		switch {
		case last.IsZero():
			view.dot.Color = hex("#6f737d")
			view.status.Text = "waiting"
		case now.Sub(last) < 2*time.Minute:
			view.dot.Color = hex("#4ade80")
			view.status.Text = "live"
		default:
			view.dot.Color = hex("#6f737d")
			view.status.Text = "idle"
		}

		canvas.Refresh(view.dot)
		canvas.Refresh(view.status)
	}
}

// startPresenceTimer keeps the roster honest for as long as the window is open.
func (a *App) startPresenceTimer() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			fyne.Do(func() { a.refreshPresence() })
		}
	}()
}
