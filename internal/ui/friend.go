package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/session"
)

// showFriend is what a friend sees once they are connected.
func (a *App) showFriend(inv invite.Invite) {
	title := canvas.NewText(inv.GroupName, hex("#e9edf5"))
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}

	host := canvas.NewText("Hosted by "+inv.HostName, hex("#757c8c"))
	host.TextSize = 12

	status := container.NewHBox(
		canvas.NewText("●", hex("#22c55e")),
		widget.NewLabel("Connected"),
		layout.NewSpacer(),
		a.disconnectButton(),
	)

	header := container.NewVBox(
		title,
		host,
		status,
	)

	body := container.NewVBox(section("Ready to play"))

	bound := a.friend.Bound()
	if len(bound) == 0 {
		body.Add(muted("The host has not published anything yet."))
	}
	for _, b := range bound {
		body.Add(boundRow(a, b))
	}

	body.Add(widget.NewSeparator())

	forget := widget.NewButton("Forget this group", func() { a.forget(inv) })
	forget.Importance = widget.LowImportance
	body.Add(forget)

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(header),
		a.footer(),
		nil,
		nil,
		container.NewScroll(body),
	)))
	a.win.SetTitle("LAN Party · " + inv.GroupName)
}

func (a *App) disconnectButton() fyne.CanvasObject {
	button := widget.NewButton("Disconnect", func() {
		if a.friend != nil {
			_ = a.friend.Close()
			a.friend = nil
		}
		a.showHome()
	})
	button.Importance = widget.DangerImportance
	return button
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

func boundRow(a *App, b session.Bound) fyne.CanvasObject {
	name := canvas.NewText(b.Service, hex("#e9edf5"))
	name.TextSize = 13
	name.TextStyle = fyne.TextStyle{Bold: true}

	rows := container.NewVBox(name)

	if b.Proto != "" {
		rows.Add(muted(b.Proto))
	}

	address := canvas.NewText(b.Address, hex("#22d3ee"))
	address.TextSize = 13
	address.TextStyle = fyne.TextStyle{Monospace: true}
	rows.Add(address)

	copyButton := widget.NewButton("Copy", func() {
		a.fyneApp.Clipboard().SetContent(b.Address)
	})

	return container.NewHBox(
		rows,
		layout.NewSpacer(),
		container.NewVBox(layout.NewSpacer(), copyButton, layout.NewSpacer()),
	)
}
