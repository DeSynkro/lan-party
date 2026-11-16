package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// memberView is the bit of a roster row that changes while a session runs.
type memberView struct {
	dot    *canvas.Text
	status *widget.Label
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
		if last.IsZero() {
			view.dot.Color = hex("#6b7280")
			view.status.SetText("waiting")
		} else {
			view.dot.Color = hex("#22c55e")
			view.status.SetText("joined " + ago(now.Sub(last)))
		}
		canvas.Refresh(view.dot)
	}
}

func ago(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
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
