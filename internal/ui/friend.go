package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/session"
)

// showFriend is what a friend sees once they are connected.
func (a *App) showFriend(inv invite.Invite) {
	disconnect := quietButton("Disconnect", func() {
		if a.friend != nil {
			_ = a.friend.Close()
			a.friend = nil
		}
		a.showHome()
	})

	header := container.NewVBox(
		container.NewBorder(nil, nil, pill("CONNECTED", "#22c55e"), disconnect, layout.NewSpacer()),
		titleText(inv.GroupName),
		muted("Hosted by "+inv.HostName),
	)

	body := container.NewVBox(sectionHeader("Ready to play", nil))

	bound := a.friend.Bound()
	if len(bound) == 0 {
		body.Add(muted("The host has not published anything yet."))
	}
	for _, b := range bound {
		body.Add(boundCard(a, b))
	}

	forget := quietButton("Forget this group", func() { a.forget(inv) })
	body.Add(container.NewHBox(forget, layout.NewSpacer()))

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(header),
		a.footer(),
		nil,
		nil,
		container.NewScroll(body),
	)))
	a.win.SetTitle("LAN Party · " + inv.GroupName)
}

func (a *App) forget(inv invite.Invite) {
	if a.friend != nil {
		_ = a.friend.Close()
		a.friend = nil
	}

	kept := make([]invite.Invite, 0, len(a.joined))
	for _, saved := range a.joined {
		if saved.GroupID != inv.GroupID {
			kept = append(kept, saved)
		}
	}
	a.joined = kept
	if err := a.inviteStore.Save(a.joined); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.showHome()
}

func boundCard(a *App, b session.Bound) fyne.CanvasObject {
	name := titleText(b.Service)
	name.TextSize = 15

	left := container.NewVBox(name)
	if b.Proto != "" {
		left.Add(muted(b.Proto))
	}
	left.Add(addressText(b.Address))

	copyButton := widget.NewButton("Copy", func() {
		a.fyneApp.Clipboard().SetContent(b.Address)
	})

	return card(container.NewBorder(
		nil, nil, nil,
		container.NewVBox(layout.NewSpacer(), copyButton, layout.NewSpacer()),
		left,
	))
}
