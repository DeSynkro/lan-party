package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/desynkro/lan-party/internal/group"
)

func (u *UI) showHome() {
	u.back = nil
	u.current = u.showHome
	u.screenName = "home"

	hero := container.NewVBox(
		titleText("A LAN cable across the internet."),
		gap(rowGap),
		bodyLabel("Host a game on your LAN. Friends open it at 127.0.0.1; nothing else is reachable."),
	)

	actions := container.NewGridWithColumns(2,
		newButton("Host a group", buttonPrimary, u.newGroupDialog),
		newButton("Join with an invite", buttonSecondary, u.joinDialog),
	)

	content := container.NewVBox(
		hero,
		gap(sectionGap),
		actions,
		container.NewCenter(smallButton("First time? Read the setup guide", buttonQuiet, func() {
			u.showGuide(u.showHome)
		})),
	)

	groups := u.app.Groups()
	hostGroups := make([]group.Group, 0, len(groups))
	joinedGroups := make([]group.Group, 0, len(groups))
	for _, g := range groups {
		if g.IsHost() {
			hostGroups = append(hostGroups, g)
		} else {
			joinedGroups = append(joinedGroups, g)
		}
	}

	if len(hostGroups) > 0 {
		content.Add(gap(sectionGap))
		content.Add(sectionLabel("Your groups"))
		content.Add(gap(rowGap))
		for i, g := range hostGroups {
			if i > 0 {
				content.Add(gap(rowGap))
			}
			content.Add(u.homeGroupRow(g))
		}
	}

	if len(joinedGroups) > 0 {
		content.Add(gap(sectionGap))
		content.Add(sectionLabel("Groups you joined"))
		content.Add(gap(rowGap))
		for i, g := range joinedGroups {
			if i > 0 {
				content.Add(gap(rowGap))
			}
			content.Add(u.homeGroupRow(g))
		}
	}

	if len(groups) == 0 {
		content.Add(gap(sectionGap))
		content.Add(sectionLabel("Your groups"))
		content.Add(gap(rowGap))
		content.Add(card(mutedLabel("Nothing saved yet. Hosted and joined groups appear here.")))
	}

	u.screen(u.appBar("", true), content, nil)
}

func (u *UI) homeGroupRow(g group.Group) fyne.CanvasObject {
	meta := "host · " + plural(len(g.Members), "member")
	if !g.IsHost() {
		meta = "joined · hosted by " + g.HostName
	}
	row := newRowButton(g.Name, meta, func() { u.showGroup(g.ID) })
	if u.app.LiveGroupID() == g.ID {
		row.setLive(true)
	}

	if !g.IsHost() {
		return row
	}

	deleteButton := smallButton("Delete", buttonQuiet, func() { u.confirmDeleteGroup(g) })
	return container.NewBorder(nil, nil, nil, container.NewCenter(deleteButton), row)
}
