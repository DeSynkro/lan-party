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
		titleText("One port, not the network."),
		gap(6),
		bodyLabel("Host a game on your LAN. Friends open it at 127.0.0.1; nothing else is reachable."),
	)

	actions := container.NewGridWithColumns(2,
		newButton("Host a group", buttonPrimary, u.newGroupDialog),
		newButton("Join with an invite", buttonSecondary, u.joinDialog),
	)

	content := container.NewVBox(
		hero,
		gap(16),
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
		content.Add(gap(20))
		content.Add(sectionLabel("Your groups"))
		content.Add(gap(8))
		for i, g := range hostGroups {
			if i > 0 {
				content.Add(gap(6))
			}
			content.Add(u.homeGroupRow(g))
		}
	}

	if len(joinedGroups) > 0 {
		content.Add(gap(20))
		content.Add(sectionLabel("Groups you joined"))
		content.Add(gap(8))
		for i, g := range joinedGroups {
			if i > 0 {
				content.Add(gap(6))
			}
			content.Add(u.homeGroupRow(g))
		}
	}

	if len(groups) == 0 {
		content.Add(gap(20))
		content.Add(sectionLabel("Your groups"))
		content.Add(gap(8))
		content.Add(card(mutedLabel("Nothing saved yet. Hosted and joined groups appear here.")))
	}

	content.Add(gap(20))
	content.Add(u.howItWorks())

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

func (u *UI) howItWorks() fyne.CanvasObject {
	left := container.NewBorder(nil, nil, nil, metaText("your server"), addressText("192.168.1.50:25565"))
	middle := container.NewCenter(metaText("encrypted"))
	right := container.NewBorder(nil, nil, nil, metaText("their game"), addressText("127.0.0.1:25565"))

	return card(container.NewVBox(
		sectionLabel("How it works"),
		gap(10),
		left,
		gap(4),
		middle,
		gap(4),
		right,
		gap(10),
		mutedLabel("The example addresses are a sketch. Only the ports you publish cross the tunnel."),
	))
}
