package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"

	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/session"
)

// showFriend is what a friend sees once they are connected.
func (a *App) showFriend(inv invite.Invite) {
	header := container.NewVBox(
		container.NewBorder(nil, nil,
			pill("Connected", "#4ade80"),
			newButton("Disconnect", buttonQuiet, func() {
				if a.friend != nil {
					_ = a.friend.Close()
					a.friend = nil
				}
				a.showHome()
			}),
			layout.NewSpacer(),
		),
		gap(8),
		titleText(inv.GroupName),
		metaText("Hosted by "+inv.HostName),
	)

	body := container.NewVBox(sectionLabel("Ready to play"), gap(8))

	bound := a.friend.Bound()
	if len(bound) == 0 {
		body.Add(metaText("The host has not published anything yet."))
	}
	for i, b := range bound {
		if i > 0 {
			body.Add(gap(8))
		}
		body.Add(boundCard(a, b))
	}

	body.Add(gap(24))
	body.Add(container.NewHBox(newButton("Forget this group", buttonQuiet, func() { a.forget(inv) }), layout.NewSpacer()))

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(header),
		a.footer(),
		nil,
		nil,
		container.NewScroll(pad(0, 12, body)),
	)))
	a.win.Canvas().Unfocus()
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
	letter := strings.ToUpper(b.Service[:1])

	left := container.NewHBox(
		tile(letter),
		gapW(12),
		container.NewVBox(nameText(b.Service), gap(2), metaText(b.Proto)),
	)

	right := container.NewHBox(
		addressText(b.Address),
		gapW(10),
		newButton("Copy", buttonSecondary, func() {
			a.fyneApp.Clipboard().SetContent(b.Address)
		}),
	)

	return card(container.NewBorder(nil, nil, left,
		container.NewVBox(layout.NewSpacer(), right, layout.NewSpacer()), nil))
}
