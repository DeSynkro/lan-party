package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

type friendView struct {
	u         *UI
	id        string
	live      bool
	dot       *canvas.Text
	status    *canvas.Text
	link      *canvas.Text
	note      *widget.Label
	services  *fyne.Container
	signature string
}

func (u *UI) showFriend(g group.Group) {
	u.back = u.showHome
	u.current = func() { u.showGroup(g.ID) }
	u.screenName = "friend"

	live := u.app.LiveGroupID() == g.ID && u.app.FriendPresence().Live
	view := &friendView{
		u:      u,
		id:     g.ID,
		live:   live,
		dot:    canvas.NewText("●", colourMuted),
		status: canvas.NewText("", colourText),
		link:   canvas.NewText("", colourMuted),
		note:   mutedLabel(""),
	}
	view.dot.TextSize = 8
	view.status.TextSize = sizeName
	view.status.TextStyle = fyne.TextStyle{Bold: true}
	view.link.TextSize = sizeMeta
	view.services = container.NewVBox()
	view.updateStatus(g)
	view.fillServices(g)

	content := container.NewVBox(
		view.statusPanel(g),
		gap(12),
		sectionLabel("Ready to play"),
		gap(4),
		view.services,
		gap(12),
		sectionLabel("People"),
		gap(4),
		view.people(g),
		gap(12),
		divider(),
		newButton("Leave group", buttonDanger, func() { u.confirmLeaveGroup(g) }),
		gap(4),
		mutedLabel("This removes the saved group and its device key from this PC."),
	)

	u.screen(u.appBar(g.Name, false), content, func() { u.refreshFriendView(view) })
}

func (v *friendView) statusPanel(g group.Group) fyne.CanvasObject {
	action := smallButton("Reconnect", buttonPrimary, func() { v.u.reconnect(g) })
	if v.live {
		action = smallButton("Disconnect", buttonQuiet, func() { v.u.confirmDisconnect(g) })
	}

	top := container.NewBorder(nil, nil,
		container.NewHBox(container.NewCenter(v.dot), gapW(6), v.status),
		container.NewCenter(action),
		layout.NewSpacer(),
	)

	return card(container.NewVBox(
		top,
		gap(4),
		metaText("Hosted by "+g.HostName),
		gap(4),
		v.link,
		v.note,
	))
}

func (v *friendView) fillServices(g group.Group) {
	objs := []fyne.CanvasObject{}
	if v.live {
		bounds := v.u.app.FriendBounds()
		v.signature = boundsSignature(bounds)
		if len(bounds) == 0 {
			objs = append(objs, mutedLabel("No service open. The host has not published anything yet."))
		} else {
			order, grouped := groupBounds(bounds)
			for i, id := range order {
				if i > 0 {
					objs = append(objs, gap(4))
				}
				svc, ok := g.FindService(id)
				if !ok {
					svc = serviceFromBounds(id, grouped[id])
				}
				objs = append(objs, v.u.friendServiceCard(svc, grouped[id]))
			}
		}
	} else {
		v.signature = servicesSignature(g.Services)
		if len(g.Services) == 0 {
			objs = append(objs, mutedLabel("No service open. The host has not published anything yet."))
		} else {
			for i, svc := range g.Services {
				if i > 0 {
					objs = append(objs, gap(4))
				}
				objs = append(objs, v.u.offlineServiceCard(svc))
			}
		}
	}

	v.services.Objects = objs
	v.services.Refresh()
}

func (u *UI) friendServiceCard(s group.Service, bounds []session.Bound) fyne.CanvasObject {
	top := container.NewBorder(nil, nil, nil, container.NewCenter(tag(protoName(s.Proto))), nameText(s.Name))
	body := container.NewVBox(top, gap(4))

	seen := map[string]bool{}
	first := true
	for _, bound := range bounds {
		if bound.Error != "" {
			body.Add(warningLabel(bound.Error))
			continue
		}
		if seen[bound.Address] {
			continue
		}
		seen[bound.Address] = true

		var address fyne.CanvasObject
		if first {
			address = addressHero(bound.Address)
			first = false
		} else {
			address = addressText(bound.Address)
		}
		var copyButton *button
		copyButton = smallButton("Copy", buttonQuiet, func() {
			u.copyText(bound.Address, copyButton)
		})
		body.Add(container.NewBorder(nil, nil, nil, container.NewCenter(copyButton), address))
		if bound.Port != bound.PublishedPort {
			body.Add(metaText(fmt.Sprintf("Port %d was busy; this game uses %d.", bound.PublishedPort, bound.Port)))
		}
	}

	if s.Hint != "" {
		body.Add(gap(4))
		body.Add(mutedLabel(s.Hint))
	}
	return card(body)
}

func (u *UI) offlineServiceCard(s group.Service) fyne.CanvasObject {
	top := container.NewBorder(nil, nil, nil, container.NewCenter(tag(protoName(s.Proto))), nameText(s.Name))
	body := container.NewVBox(
		top,
		gap(4),
		metaText("Port "+group.FormatPorts(s.Ports)),
		mutedLabel("Host offline. Reconnect when the host is back."),
	)
	return card(body)
}

func (v *friendView) people(g group.Group) fyne.CanvasObject {
	presence := v.u.app.FriendPresence()
	hostDetail := "host"
	if presence.Live {
		hostDetail = "host · " + presence.RTT.Round(time.Millisecond).String()
	}

	hostName := container.NewHBox(nameText(g.HostName), gapW(6), tag("host"))
	host := card(container.NewBorder(nil, nil, statusDot(presence.Live), nil,
		container.NewVBox(hostName, metaText(hostDetail))))

	selfName := g.SelfName
	if strings.TrimSpace(selfName) == "" {
		selfName = "you"
	}
	youName := container.NewHBox(nameText("You ("+selfName+")"), gapW(6), tag("you"))
	you := card(container.NewBorder(nil, nil, statusDot(true), nil,
		container.NewVBox(youName, metaText(v.u.app.DeviceName()))))

	return container.NewVBox(host, gap(4), you)
}

func (v *friendView) updateStatus(g group.Group) {
	presence := v.u.app.FriendPresence()
	note := v.u.app.FriendNote()

	v.live = v.u.app.LiveGroupID() == g.ID && presence.Live
	if v.live {
		v.dot.Color = colourLive
		v.status.Text = "Host live"
		v.status.Color = colourLive
		v.link.Text = presence.RTT.Round(time.Millisecond).String()
		v.note.SetText("")
	} else {
		v.dot.Color = colourMuted
		v.status.Text = "Host offline"
		v.status.Color = colourText
		v.link.Text = "not connected"
		if note != "" {
			v.note.SetText(note)
		} else {
			v.note.SetText("Start the session on the host, then reconnect.")
		}
	}
	v.dot.Refresh()
	v.status.Refresh()
	v.link.Refresh()
	v.note.Refresh()
}

func (u *UI) refreshFriendView(v *friendView) {
	g, ok := u.app.Group(v.id)
	if !ok {
		u.showHome()
		return
	}
	live := u.app.LiveGroupID() == v.id && u.app.FriendPresence().Live
	if live != v.live {
		u.showFriend(g)
		return
	}
	v.updateStatus(g)

	var signature string
	if live {
		signature = boundsSignature(u.app.FriendBounds())
	} else {
		signature = servicesSignature(g.Services)
	}
	if signature != v.signature {
		v.fillServices(g)
	}
}

func (u *UI) reconnect(g group.Group) {
	u.runWaiting("Connecting", "Waiting for the host to answer.",
		func(ctx context.Context) error {
			timed, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			return u.app.Reconnect(timed, g.ID)
		},
		func(err error) {
			if ignoreCancel(err) {
				return
			}
			if err != nil {
				u.showError(err)
				return
			}
			u.showGroup(g.ID)
		})
}

func (u *UI) confirmDisconnect(g group.Group) {
	dialog.NewConfirm("Disconnect?",
		"The group stays saved. You can reconnect when the host is online.",
		func(ok bool) {
			if !ok {
				return
			}
			_ = u.app.StopSession()
			u.showGroup(g.ID)
		}, u.win).Show()
}

func (u *UI) confirmLeaveGroup(g group.Group) {
	dialog.NewConfirm("Leave "+g.Name+"?",
		"This removes the saved group and its device key from this PC. The host keeps their copy until they remove you.",
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

func groupBounds(bounds []session.Bound) ([]string, map[string][]session.Bound) {
	order := []string{}
	grouped := map[string][]session.Bound{}
	for _, bound := range bounds {
		if _, ok := grouped[bound.ServiceID]; !ok {
			order = append(order, bound.ServiceID)
		}
		grouped[bound.ServiceID] = append(grouped[bound.ServiceID], bound)
	}
	return order, grouped
}

func serviceFromBounds(id string, bounds []session.Bound) group.Service {
	svc := group.Service{ID: id}
	seenPorts := map[int]bool{}
	hasTCP, hasUDP := false, false
	for _, bound := range bounds {
		if svc.Name == "" {
			svc.Name = bound.Service
			svc.Hint = bound.Hint
		}
		if bound.Proto == "tcp" {
			hasTCP = true
		}
		if bound.Proto == "udp" {
			hasUDP = true
		}
		if !seenPorts[bound.PublishedPort] {
			seenPorts[bound.PublishedPort] = true
			svc.Ports = append(svc.Ports, bound.PublishedPort)
		}
	}
	switch {
	case hasTCP && hasUDP:
		svc.Proto = "both"
	case hasUDP:
		svc.Proto = "udp"
	default:
		svc.Proto = "tcp"
	}
	return svc
}

func boundsSignature(bounds []session.Bound) string {
	var b strings.Builder
	for _, bound := range bounds {
		fmt.Fprintf(&b, "%s|%s|%s|%s;", bound.ServiceID, bound.Proto, bound.Address, bound.Error)
	}
	return b.String()
}

func servicesSignature(services []group.Service) string {
	var b strings.Builder
	for _, svc := range services {
		fmt.Fprintf(&b, "%s|%s|%v|%s;", svc.ID, svc.Proto, svc.Ports, svc.Hint)
	}
	return b.String()
}
