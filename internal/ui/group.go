package ui

import (
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

type hostView struct {
	u      *UI
	id     string
	live   bool
	dot    *canvas.Text
	status *canvas.Text
	link   *canvas.Text
	people *fyne.Container
	invite *fyne.Container
}

func (u *UI) showHostGroup(g group.Group) {
	u.back = u.showHome
	u.current = func() { u.showGroup(g.ID) }
	u.screenName = "host"

	view := &hostView{
		u:      u,
		id:     g.ID,
		live:   u.app.LiveGroupID() == g.ID,
		dot:    canvas.NewText("●", colourMuted),
		status: canvas.NewText("", colourText),
		link:   canvas.NewText("", colourMuted),
	}
	view.dot.TextSize = 8
	view.status.TextSize = sizeName
	view.status.TextStyle = fyne.TextStyle{Bold: true}
	view.link.TextSize = sizeMeta

	view.people = container.NewVBox()
	view.invite = container.NewVBox()
	view.updateStatus(g)
	view.fillPeople(g)
	view.fillInvites(g)

	content := container.NewVBox(
		view.sessionPanel(g),
		gap(16),
		u.publishedHeader(g),
		gap(8),
		u.servicesBox(g),
		gap(16),
		u.peopleHeader(g),
		gap(8),
		view.people,
		gap(16),
		sectionLabel("Invites"),
		gap(8),
		view.invite,
		gap(16),
		divider(),
		newButton("Delete group", buttonDanger, func() { u.confirmDeleteGroup(g) }),
		gap(6),
		mutedLabel("This ends any running session and revokes every member. Friends keep a saved entry until they remove it."),
	)

	u.screen(u.appBar(g.Name, false), content, func() {
		u.refreshHostView(view)
	})
}

func (v *hostView) sessionPanel(g group.Group) fyne.CanvasObject {
	button := newButton("Start session", buttonPrimary, func() { v.u.startSession(g) })
	if v.live {
		button = newButton("End session", buttonDanger, func() { v.u.confirmEndSession(g) })
	}

	top := container.NewBorder(
		nil, nil,
		container.NewHBox(container.NewCenter(v.dot), gapW(6), v.status),
		button,
		layout.NewSpacer(),
	)
	hostLine := metaText("Hosting as " + g.HostName + " · " + plural(len(g.Members), "member"))

	panel := container.NewVBox(
		top,
		gap(4),
		hostLine,
		gap(10),
		v.addressRow(g),
		gap(6),
		v.link,
	)
	return card(panel)
}

func (v *hostView) addressRow(g group.Group) fyne.CanvasObject {
	settings := v.u.app.Settings()
	public := strings.TrimSpace(settings.PublicAddress)
	port := strconv.Itoa(settings.AppPort)
	change := smallButton("Change", buttonQuiet, func() {
		v.u.showSettings(func() { v.u.showGroup(g.ID) })
	})

	if public == "" {
		change.setLabel("Set address")
		text := container.NewVBox(
			warningLabel("No public address — friends can't join"),
			metaText("UDP "+port),
		)
		return container.NewBorder(nil, nil, nil, container.NewCenter(change), text)
	}

	endpoint, err := v.u.app.CurrentEndpoint()
	if err != nil {
		endpoint = public
	}
	text := container.NewVBox(
		addressText(endpoint),
		metaText("UDP "+port+" on this PC"),
	)
	return container.NewBorder(nil, nil, nil, container.NewCenter(change), text)
}

func (u *UI) publishedHeader(g group.Group) fyne.CanvasObject {
	add := smallButton("Add service", buttonQuiet, func() { u.addServiceDialog(g) })
	return container.NewBorder(nil, nil, nil, container.NewCenter(add),
		sectionLabel("Published · "+strconv.Itoa(len(g.Services))))
}

func (u *UI) servicesBox(g group.Group) fyne.CanvasObject {
	box := container.NewVBox()
	if len(g.Services) == 0 {
		box.Add(mutedLabel("Nothing published yet. Add the game server you want friends to reach."))
		return box
	}
	for i, s := range g.Services {
		if i > 0 {
			box.Add(gap(8))
		}
		box.Add(u.serviceCard(g, s))
	}
	return box
}

func (u *UI) serviceCard(g group.Group, s group.Service) fyne.CanvasObject {
	top := container.NewBorder(nil, nil, nil, container.NewCenter(tag(protoName(s.Proto))), nameText(s.Name))

	friendLine := container.NewHBox(
		metaText("Friends open"),
		gapW(6),
		addressText("127.0.0.1:"+group.FormatPorts(s.Ports)),
	)
	targetLine := container.NewHBox(
		metaText("On your LAN"),
		gapW(6),
		addressMuted(s.Target),
	)

	body := container.NewVBox(top, gap(8), friendLine, targetLine)
	if s.Hint != "" {
		body.Add(gap(6))
		body.Add(mutedLabel(s.Hint))
	}
	for _, warning := range group.ServiceWarnings(s) {
		body.Add(warningLabel(warning))
	}

	return container.NewStack(
		card(body),
		newTapArea(func() { u.showServiceDetails(g, s) }),
	)
}

func (u *UI) peopleHeader(g group.Group) fyne.CanvasObject {
	invite := smallButton("Invite", buttonQuiet, func() { u.inviteFor(g) })
	return container.NewBorder(nil, nil, nil, container.NewCenter(invite),
		sectionLabel("People · "+plural(len(g.Members), "member")))
}

func (v *hostView) fillPeople(g group.Group) {
	objs := []fyne.CanvasObject{v.hostRow(g)}
	presence := v.u.app.HostPresence()
	for _, member := range g.Members {
		objs = append(objs, gap(6), v.memberRow(g, member, presence))
	}
	if len(g.Members) == 0 {
		objs = append(objs, gap(8), mutedLabel("No friends yet. Create an invite when you're ready."))
	}
	v.people.Objects = objs
	v.people.Refresh()
}

func (v *hostView) hostRow(g group.Group) fyne.CanvasObject {
	name := container.NewHBox(
		nameText("You ("+g.HostName+")"),
		gapW(6),
		tag("host"),
	)
	text := container.NewVBox(name, metaText("this device"))
	return card(container.NewBorder(nil, nil, statusDot(true), nil, text))
}

func (v *hostView) memberRow(g group.Group, member group.Member, presence map[string]session.Presence) fyne.CanvasObject {
	detail := "idle"
	live := false
	if member.State != group.MemberApproved {
		detail = "waiting for approval"
	} else if p, ok := presence[member.ID]; ok && p.Live {
		live = true
		detail = "live · " + p.RTT.Round(time.Millisecond).String()
	}

	row := newRowButton(member.Name, detail, func() {
		v.u.showMemberDetails(g, member)
	})
	row.setLive(live)
	return row
}

func (v *hostView) fillInvites(g group.Group) {
	open := g.OpenInvites(time.Now())
	objs := make([]fyne.CanvasObject, 0, len(open)*2)
	if len(open) == 0 {
		objs = append(objs, mutedLabel("No open invites."))
	}
	for _, inv := range open {
		code := v.u.app.InviteCode(inv.ID)
		row := newRowButton(inv.Name, expiresIn(inv.ExpiresAt), func() {
			v.u.showInviteDetails(g, inv, code)
		})
		row.setChevron(false)
		if code == "" {
			objs = append(objs, row, gap(6))
			continue
		}
		var copyButton *button
		copyButton = smallButton("Copy", buttonQuiet, func() {
			v.u.copyText(code, copyButton)
		})
		objs = append(objs,
			container.NewBorder(nil, nil, nil, container.NewCenter(copyButton), row),
			gap(6),
		)
	}
	v.invite.Objects = objs
	v.invite.Refresh()
}

func (v *hostView) updateStatus(g group.Group) {
	presence := v.u.app.HostPresence()
	connected := 0
	for _, p := range presence {
		if p.Live {
			connected++
		}
	}
	approved := 0
	for _, m := range g.Members {
		if m.State == group.MemberApproved {
			approved++
		}
	}

	if v.live {
		v.dot.Color = colourLive
		v.status.Text = "Session live"
		v.status.Color = colourLive
		if approved > 0 {
			v.link.Text = strconv.Itoa(connected) + " of " + strconv.Itoa(approved) + " connected"
		} else {
			v.link.Text = "no friends in the group yet"
		}
	} else {
		v.dot.Color = colourMuted
		v.status.Text = "Session off"
		v.status.Color = colourText
		v.link.Text = "not running"
	}
	v.dot.Refresh()
	v.status.Refresh()
	v.link.Refresh()
}

func (u *UI) refreshHostView(v *hostView) {
	g, ok := u.app.Group(v.id)
	if !ok {
		u.showHome()
		return
	}
	live := u.app.LiveGroupID() == v.id
	if live != v.live {
		u.showHostGroup(g)
		return
	}
	v.updateStatus(g)
	v.fillPeople(g)
	v.fillInvites(g)
}

func (u *UI) startSession(g group.Group) {
	if err := u.app.StartSession(g.ID); err != nil {
		u.showError(err)
		return
	}
	u.showGroup(g.ID)
}

func (u *UI) confirmEndSession(g group.Group) {
	dialog.NewConfirm("End the session?",
		"Connected friends will be disconnected. The group and its invites stay saved.",
		func(ok bool) {
			if !ok {
				return
			}
			_ = u.app.StopSession()
			u.showGroup(g.ID)
		}, u.win).Show()
}

func (u *UI) confirmDeleteGroup(g group.Group) {
	dialog.NewConfirm("Delete "+g.Name+"?",
		"This ends the session and revokes every member. They will not be able to reconnect until you invite them again.",
		func(ok bool) {
			if !ok {
				return
			}
			if err := u.app.DeleteGroup(g.ID); err != nil {
				u.showError(err)
				return
			}
			u.showHome()
		}, u.win).Show()
}

func (u *UI) inviteFor(g group.Group) {
	if strings.TrimSpace(u.app.Settings().PublicAddress) == "" {
		u.showInfo("Public address needed", "Set your public address in Settings before creating an invite.")
		return
	}
	u.inviteDialog(g)
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

func protoLabel(proto string) string {
	switch proto {
	case "tcp":
		return "TCP"
	case "udp":
		return "UDP"
	default:
		return "TCP and UDP"
	}
}

func protoValue(label string) string {
	switch label {
	case "TCP":
		return "tcp"
	case "UDP":
		return "udp"
	default:
		return "both"
	}
}
