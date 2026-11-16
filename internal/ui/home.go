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
	title := canvas.NewText("LAN PARTY", hex("#e9edf5"))
	title.TextSize = 28
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("One port, not the network.", hex("#757c8c"))
	subtitle.TextSize = 12
	subtitle.Alignment = fyne.TextAlignCenter

	mark := canvas.NewImageFromResource(appIcon)
	mark.FillMode = canvas.ImageFillContain
	mark.SetMinSize(fyne.NewSize(84, 84))

	heading := container.NewVBox(
		container.NewCenter(mark),
		gap(10),
		container.NewCenter(title),
		container.NewCenter(subtitle),
	)

	list := container.NewVBox()

	if len(a.groups) > 0 {
		list.Add(sectionHeader("Your groups", nil))
		for _, g := range a.groups {
			g := g
			list.Add(widget.NewButton(fmt.Sprintf("%s · %d members", g.Name, len(g.Members)), func() {
				a.showGroup(g)
			}))
		}
	}

	if len(a.joined) > 0 {
		list.Add(sectionHeader("Groups you joined", nil))
		for _, inv := range a.joined {
			inv := inv
			label := inv.GroupName + " · " + inv.HostName
			list.Add(widget.NewButton(label, func() { a.openJoined(inv) }))
		}
	}

	if len(a.groups) > 0 || len(a.joined) > 0 {
		list.Add(widget.NewSeparator())
	}

	start := widget.NewButton("Start a group", a.newGroup)
	start.Importance = widget.HighImportance

	join := widget.NewButton("Join a group", a.joinGroup)

	list.Add(start)
	list.Add(join)

	hint := canvas.NewText("Paste an invite to join someone else's LAN.", hex("#5c6373"))
	hint.TextSize = 11
	hint.Alignment = fyne.TextAlignCenter
	list.Add(container.NewCenter(hint))

	var centre fyne.CanvasObject
	if len(a.groups) == 0 && len(a.joined) == 0 {
		centre = container.NewVBox(layout.NewSpacer(), list, layout.NewSpacer())
	} else {
		centre = container.NewVScroll(list)
	}

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(heading),
		a.footer(),
		nil,
		nil,
		container.NewPadded(centre),
	)))
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
	settings := quietButton("Settings", a.showSettings)

	return container.NewVBox(
		widget.NewSeparator(),
		container.NewBorder(
			nil,
			nil,
			nil,
			settings,
			container.NewCenter(muted(versionText())),
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
