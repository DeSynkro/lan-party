package gtkui

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"

	"github.com/desynkro/lan-party/internal/group"
)

func (u *probeUI) buildGroupContent(g group.Group) *gtk.Box {
	page := gtk.NewBox(gtk.OrientationVertical, 0)
	page.AddCSSClass("page")
	page.SetMarginTop(20)
	page.SetMarginBottom(20)
	page.SetMarginStart(20)
	page.SetMarginEnd(20)

	errorLabel := textLabel("", "error")
	errorLabel.SetVisible(false)
	page.Append(errorLabel)

	showError := func(err error) {
		errorLabel.SetText(err.Error())
		errorLabel.SetVisible(true)
	}

	if g.IsHost() {
		page.Append(u.hostSessionCard(g, showError))
		page.Append(sectionTitle("Published", strconv.Itoa(len(g.Services))))
		page.Append(u.hostServices(g))
		page.Append(sectionTitle("People", plural(len(g.Members), "member")))
		page.Append(u.hostPeople(g))
		page.Append(sectionTitle("Invites", ""))
		page.Append(u.hostInvites(g))
		page.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		page.Append(gapBox(10))
		leave := gtk.NewButtonWithLabel("Delete group")
		leave.AddCSSClass("secondary")
		leave.ConnectClicked(func() {
			log.Println("gtk probe: delete group dialog not ported yet")
		})
		page.Append(leave)
		page.Append(gapBox(4))
		page.Append(measuredText("This ends any running session and revokes every member.", "desc"))
	} else {
		page.Append(u.friendSessionCard(g, showError))
		page.Append(sectionTitle("Ready to play", ""))
		page.Append(u.friendServices(g))
		page.Append(sectionTitle("People", ""))
		page.Append(u.friendPeople(g))
		page.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		page.Append(gapBox(10))
		leave := gtk.NewButtonWithLabel("Leave group")
		leave.AddCSSClass("secondary")
		leave.ConnectClicked(func() {
			log.Println("gtk probe: leave group dialog not ported yet")
		})
		page.Append(leave)
		page.Append(gapBox(4))
		page.Append(measuredText("This removes the saved group and its device key from this PC.", "desc"))
	}

	return page
}

func (u *probeUI) hostSessionCard(g group.Group, showError func(error)) *gtk.Box {
	live := u.state.LiveGroupID() == g.ID
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginBottom(10)

	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	top.Append(statusDot(live))
	status := lineLabel("Session off", "card-title")
	if live {
		status.SetText("Session live")
	}
	top.Append(status)
	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	top.Append(spacer)

	action := gtk.NewButtonWithLabel("Start session")
	action.AddCSSClass("primary")
	if live {
		action.SetLabel("End session")
	}
	action.ConnectClicked(func() {
		if live {
			_ = u.state.StopSession()
			u.showGroup(g.ID)
			return
		}
		if err := u.state.StartSession(g.ID); err != nil {
			showError(err)
			return
		}
		u.showGroup(g.ID)
	})
	top.Append(action)
	card.Append(top)
	card.Append(gapBox(10))
	card.Append(measuredText("Hosting as "+g.HostName+" · "+plural(len(g.Members), "member"), "desc"))
	card.Append(gapBox(8))

	endpoint, err := u.state.CurrentEndpoint()
	if err != nil {
		card.Append(measuredText("No public address — friends can't join.", "warning"))
	} else {
		addr := lineLabel(endpoint, "mono")
		addr.SetHExpand(true)
		row := gtk.NewBox(gtk.OrientationHorizontal, 8)
		row.Append(addr)
		change := gtk.NewButtonWithLabel("Change")
		change.AddCSSClass("quiet")
		change.ConnectClicked(func() {
			u.showSettings()
		})
		row.Append(change)
		card.Append(row)
	}
	card.Append(measuredText("UDP "+strconv.Itoa(u.state.Settings().AppPort)+" on this PC", "desc"))
	card.Append(gapBox(6))
	if live {
		presence := u.state.HostPresence()
		connected := 0
		for _, p := range presence {
			if p.Live {
				connected++
			}
		}
		card.Append(measuredText(fmt.Sprintf("%d connected", connected), "desc"))
	} else {
		card.Append(measuredText("not running", "desc"))
	}
	return card
}

func (u *probeUI) hostServices(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	if len(g.Services) == 0 {
		box.Append(measuredText("Nothing published yet. Add the game server you want friends to reach.", "desc"))
		return box
	}
	for i, s := range g.Services {
		if i > 0 {
			box.Append(gapBox(8))
		}
		box.Append(u.hostServiceCard(s))
	}
	return box
}

func (u *probeUI) hostServiceCard(s group.Service) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	name := lineLabel(s.Name, "row-title")
	name.SetHExpand(true)
	top.Append(name)
	top.Append(tagLabel(protoName(s.Proto)))
	card.Append(top)
	card.Append(gapBox(8))
	card.Append(measuredText("Friends open 127.0.0.1:"+group.FormatPorts(s.Ports), "desc"))
	card.Append(measuredText("On your LAN "+s.Target, "desc"))
	if s.Hint != "" {
		card.Append(gapBox(6))
		card.Append(measuredText(s.Hint, "desc"))
	}
	return card
}

func (u *probeUI) hostPeople(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.Append(personCard(g.HostName+" (you)", "this device", true, "host"))

	presence := u.state.HostPresence()
	for _, m := range g.Members {
		detail := "waiting for approval"
		live := false
		if m.State == group.MemberApproved {
			detail = "idle"
			if p, ok := presence[m.ID]; ok && p.Live {
				live = true
				detail = "live · " + p.RTT.Round(time.Millisecond).String()
			}
		}
		box.Append(gapBox(8))
		box.Append(personCard(m.Name, detail, live, ""))
	}
	if len(g.Members) == 0 {
		box.Append(gapBox(8))
		box.Append(measuredText("No friends yet. Create an invite when you're ready.", "desc"))
	}
	return box
}

func (u *probeUI) hostInvites(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	open := g.OpenInvites(time.Now())
	if len(open) == 0 {
		box.Append(measuredText("No open invites.", "desc"))
		return box
	}
	for i, inv := range open {
		if i > 0 {
			box.Append(gapBox(8))
		}
		row := gtk.NewButton()
		row.AddCSSClass("row")
		content := gtk.NewBox(gtk.OrientationHorizontal, 8)
		content.AddCSSClass("row-content")
		name := lineLabel(inv.Name, "row-title")
		name.SetHExpand(true)
		content.Append(name)
		content.Append(lineLabel(expiresIn(inv.ExpiresAt), "row-meta"))
		row.SetChild(content)
		box.Append(row)
	}
	return box
}

func (u *probeUI) friendSessionCard(g group.Group, showError func(error)) *gtk.Box {
	presence := u.state.FriendPresence()
	live := u.state.LiveGroupID() == g.ID && presence.Live
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginBottom(10)

	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	top.Append(statusDot(live))
	status := lineLabel("Host offline", "card-title")
	if live {
		status.SetText("Host live")
	}
	top.Append(status)
	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	top.Append(spacer)

	action := gtk.NewButtonWithLabel("Reconnect")
	action.AddCSSClass("primary")
	if live {
		action.SetLabel("Disconnect")
	}
	action.ConnectClicked(func() {
		if live {
			_ = u.state.StopSession()
			u.showGroup(g.ID)
			return
		}
		if err := u.state.Reconnect(context.Background(), g.ID); err != nil {
			showError(err)
			return
		}
		u.showGroup(g.ID)
	})
	top.Append(action)
	card.Append(top)
	card.Append(gapBox(10))
	card.Append(measuredText("Hosted by "+g.HostName, "desc"))
	if live {
		card.Append(measuredText(presence.RTT.Round(time.Millisecond).String(), "desc"))
	} else if note := u.state.FriendNote(); note != "" {
		card.Append(measuredText(note, "desc"))
	} else {
		card.Append(measuredText("Start the session on the host, then reconnect.", "desc"))
	}
	return card
}

func (u *probeUI) friendServices(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	live := u.state.LiveGroupID() == g.ID && u.state.FriendPresence().Live
	if !live {
		if len(g.Services) == 0 {
			box.Append(measuredText("No service open. The host has not published anything yet.", "desc"))
			return box
		}
		for i, s := range g.Services {
			if i > 0 {
				box.Append(gapBox(8))
			}
			box.Append(u.offlineFriendServiceCard(s))
		}
		return box
	}

	bounds := u.state.FriendBounds()
	if len(bounds) == 0 {
		box.Append(measuredText("No service open. The host has not published anything yet.", "desc"))
		return box
	}
	for i, b := range bounds {
		if i > 0 {
			box.Append(gapBox(8))
		}
		card := gtk.NewBox(gtk.OrientationVertical, 0)
		card.AddCSSClass("card")
		top := gtk.NewBox(gtk.OrientationHorizontal, 8)
		name := lineLabel(b.Service, "row-title")
		name.SetHExpand(true)
		top.Append(name)
		top.Append(tagLabel(protoName(b.Proto)))
		card.Append(top)
		card.Append(gapBox(8))
		card.Append(measuredText(b.Address, "mono"))
		if b.Error != "" {
			card.Append(measuredText(b.Error, "warning"))
		} else if b.Port != b.PublishedPort {
			card.Append(measuredText(fmt.Sprintf("Port %d was busy; this game uses %d.", b.PublishedPort, b.Port), "desc"))
		}
		if b.Hint != "" {
			card.Append(gapBox(6))
			card.Append(measuredText(b.Hint, "desc"))
		}
		box.Append(card)
	}
	return box
}

func (u *probeUI) offlineFriendServiceCard(s group.Service) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	name := lineLabel(s.Name, "row-title")
	name.SetHExpand(true)
	top.Append(name)
	top.Append(tagLabel(protoName(s.Proto)))
	card.Append(top)
	card.Append(gapBox(8))
	card.Append(measuredText("Port "+group.FormatPorts(s.Ports), "desc"))
	card.Append(measuredText("Host offline. Reconnect when the host is back.", "desc"))
	if s.Hint != "" {
		card.Append(gapBox(6))
		card.Append(measuredText(s.Hint, "desc"))
	}
	return card
}

func (u *probeUI) friendPeople(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	presence := u.state.FriendPresence()
	detail := "host"
	if presence.Live {
		detail = "host · " + presence.RTT.Round(time.Millisecond).String()
	}
	box.Append(personCard(g.HostName, detail, presence.Live, "host"))
	box.Append(gapBox(8))
	self := g.SelfName
	if strings.TrimSpace(self) == "" {
		self = "you"
	}
	box.Append(personCard("You ("+self+")", u.state.DeviceName(), true, "you"))
	return box
}

func personCard(name, detail string, live bool, role string) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationHorizontal, 8)
	card.AddCSSClass("card")
	card.Append(statusDot(live))
	text := gtk.NewBox(gtk.OrientationVertical, 0)
	title := gtk.NewBox(gtk.OrientationHorizontal, 6)
	title.Append(lineLabel(name, "row-title"))
	if role != "" {
		title.Append(tagLabel(role))
	}
	text.Append(title)
	text.Append(measuredText(detail, "desc"))
	card.Append(text)
	return card
}

func navRow(title, meta string, live bool, onTap func()) *gtk.Button {
	row := gtk.NewButton()
	row.AddCSSClass("row")
	row.SetHExpand(true)
	content := gtk.NewBox(gtk.OrientationHorizontal, 8)
	content.AddCSSClass("row-content")
	content.Append(statusDot(live))
	name := lineLabel(title, "row-title")
	name.SetHExpand(true)
	content.Append(name)
	content.Append(lineLabel(meta, "row-meta"))
	content.Append(lineLabel("›", "chevron"))
	row.SetChild(content)
	row.ConnectClicked(onTap)
	return row
}

func statusDot(live bool) *gtk.Label {
	dot := gtk.NewLabel("●")
	if live {
		dot.AddCSSClass("dot")
	} else {
		dot.AddCSSClass("dot-off")
	}
	return dot
}

func tagLabel(text string) *gtk.Label {
	return lineLabel(text, "tag")
}

func sectionTitle(title, count string) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	row.SetMarginTop(14)
	row.SetMarginBottom(8)
	text := title
	if count != "" {
		text += " · " + count
	}
	label := lineLabel(text, "section")
	label.SetHExpand(true)
	row.Append(label)
	return row
}

func expiresIn(at time.Time) string {
	left := time.Until(at)
	switch {
	case left <= 0:
		return "expired"
	case left < time.Minute:
		return "expires in under a minute"
	case left < time.Hour:
		return fmt.Sprintf("expires in %dm", int(left.Minutes()))
	default:
		return fmt.Sprintf("expires in %dh", int(left.Hours()))
	}
}

func plural(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", count, word)
}

func protoName(proto string) string {
	switch proto {
	case "tcp":
		return "TCP"
	case "udp":
		return "UDP"
	default:
		return "TCP + UDP"
	}
}

func lineLabel(text, class string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.AddCSSClass(class)
	label.SetWrap(false)
	label.SetXAlign(0)
	label.SetHAlign(gtk.AlignStart)
	label.SetEllipsize(pango.EllipsizeEnd)
	return label
}
