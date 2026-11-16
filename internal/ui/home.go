package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/buildinfo"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/session"
)

// showHome lists saved groups, or introduces the app when there are none.
func (a *App) showHome() {
	mark := canvas.NewImageFromResource(appIcon)
	mark.FillMode = canvas.ImageFillContain
	mark.SetMinSize(fyne.NewSize(72, 72))

	brand := canvas.NewText("LAN Party", hex("#e8eaee"))
	brand.TextSize = 22
	brand.TextStyle = fyne.TextStyle{Bold: true}
	brand.Alignment = fyne.TextAlignCenter

	tagline := canvas.NewText("One port, not the network.", hex("#6f737d"))
	tagline.TextSize = sizeMeta
	tagline.Alignment = fyne.TextAlignCenter

	heading := container.NewVBox(
		container.NewCenter(mark),
		gap(12),
		container.NewCenter(brand),
		container.NewCenter(tagline),
	)

	list := container.NewVBox()

	if len(a.groups) > 0 {
		list.Add(sectionLabel("Your groups"))
		list.Add(gap(4))
		for _, g := range a.groups {
			g := g
			label := fmt.Sprintf("%s · %d members", g.Name, len(g.Members))
			list.Add(newButton(label, buttonSecondary, func() { a.showGroup(g) }))
			list.Add(gap(4))
		}
		list.Add(gap(16))
	}

	if len(a.joined) > 0 {
		list.Add(sectionLabel("Groups you joined"))
		list.Add(gap(4))
		for _, inv := range a.joined {
			inv := inv
			label := inv.GroupName + " · " + inv.HostName
			list.Add(newButton(label, buttonSecondary, func() { a.openJoined(inv) }))
			list.Add(gap(4))
		}
		list.Add(gap(16))
	}

	list.Add(newButton("Start a group", buttonPrimary, a.newGroup))
	list.Add(gap(8))
	list.Add(newButton("Join a group", buttonSecondary, a.joinGroup))
	list.Add(gap(12))

	hint := canvas.NewText("Paste an invite to join someone else's LAN.", hex("#4d515a"))
	hint.TextSize = sizeMeta
	hint.Alignment = fyne.TextAlignCenter
	list.Add(container.NewCenter(hint))

	block := container.NewVBox(heading, gap(32), list)

	var centre fyne.CanvasObject
	if len(a.groups) == 0 && len(a.joined) == 0 {
		centre = container.NewVBox(layout.NewSpacer(), block, layout.NewSpacer())
	} else {
		centre = container.NewVScroll(block)
	}

	page := container.NewPadded(container.NewBorder(
		nil,
		a.footer(),
		nil,
		nil,
		container.NewPadded(centre),
	))
	a.win.SetContent(container.New(column{max: 460}, page))
	a.win.Canvas().Unfocus()
	a.win.SetTitle("LAN Party")
}

// newGroup asks for the two names a group needs.
func (a *App) newGroup() {
	nameEntry := widget.NewEntry()
	nameEntry.SetText("Friday Night")
	hostEntry := widget.NewEntry()
	hostEntry.SetText(defaultHostName())

	form := dialog.NewForm("Start a group", "Create", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Group name", nameEntry),
			widget.NewFormItem("Your name", hostEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			host := strings.TrimSpace(hostEntry.Text)
			if name == "" || host == "" {
				dialog.ShowInformation("Missing something", "A group needs a name, and so do you.", a.win)
				return
			}
			g := group.Create(name, host)
			a.groups = append(a.groups, g)
			if err := a.store.Save(a.groups); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.showGroup(g)
		}, a.win)
	form.Resize(fyne.NewSize(380, 250))
	form.Show()
}

// joinGroup takes an invite and connects to the host.
func (a *App) joinGroup() {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("lp1_…")
	entry.Wrapping = fyne.TextWrapBreak

	body := container.NewVBox(
		widget.NewLabel("Paste the invite you were sent."),
		entry,
	)

	d := dialog.NewCustomConfirm("Join a group", "Join", "Cancel", body, func(ok bool) {
		if !ok {
			return
		}
		inv, err := invite.Decode(entry.Text)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.openJoined(inv)
	}, a.win)
	d.Resize(fyne.NewSize(440, 260))
	d.Show()
}

// openJoined connects to a group and remembers it.
func (a *App) openJoined(inv invite.Invite) {
	if a.friend != nil || a.live != nil {
		dialog.ShowInformation("One session at a time", "End the session you are in first.", a.win)
		return
	}

	f, err := session.StartFriend(session.FriendConfig{Invite: inv})
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}

	a.friend = f
	a.remember(inv)
	a.showFriend(inv)
}

func (a *App) remember(inv invite.Invite) {
	for i := range a.joined {
		if a.joined[i].GroupID == inv.GroupID {
			a.joined[i] = inv
			if err := a.inviteStore.Save(a.joined); err != nil {
				dialog.ShowError(err, a.win)
			}
			return
		}
	}
	a.joined = append(a.joined, inv)
	if err := a.inviteStore.Save(a.joined); err != nil {
		dialog.ShowError(err, a.win)
	}
}

func (a *App) footer() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewSeparator(),
		container.NewBorder(
			nil,
			nil,
			nil,
			newButton("Settings", buttonQuiet, a.showSettings),
			container.NewCenter(metaText(versionText())),
		),
	)
}

func versionText() string {
	summary := buildinfo.Summary()
	if summary == "dev" || summary == "" {
		return "dev build"
	}
	return "v" + summary
}
