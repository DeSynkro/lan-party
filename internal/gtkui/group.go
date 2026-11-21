package gtkui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
)

type livePerson struct {
	dot    *gtk.Label
	detail *gtk.Label
}

// groupLive keeps the widgets that change while a group screen stays open.
type groupLive struct {
	groupID   string
	signature string

	hostStatus    *gtk.Label
	hostDot       *gtk.Label
	hostConnected *gtk.Label
	hostPeople    map[string]*livePerson

	inviteExpiry map[string]*gtk.Label

	friendStatus     *gtk.Label
	friendDot        *gtk.Label
	friendDetail     *gtk.Label
	friendHostDot    *gtk.Label
	friendHostDetail *gtk.Label
	friendAction     *gtk.Button
}

func (u *probeUI) buildGroupContent(g group.Group) (*gtk.Box, *groupLive) {
	live := &groupLive{
		groupID:      g.ID,
		signature:    u.groupSignature(g),
		hostPeople:   map[string]*livePerson{},
		inviteExpiry: map[string]*gtk.Label{},
	}
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
		page.Append(u.hostSessionCard(g, showError, live))
		page.Append(sectionTitleAction("Published", strconv.Itoa(len(g.Services)), "Add service", func() { u.showAddServiceDialog(g) }))
		page.Append(u.hostServices(g))
		page.Append(sectionTitleAction("People", plural(len(g.Members), "member"), "Invite", func() { u.showInviteDialog(g) }))
		page.Append(u.hostPeople(g, live))
		page.Append(sectionTitle("Invites", ""))
		page.Append(u.hostInvites(g, live))
		page.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		page.Append(gapBox(10))
		leave := gtk.NewButtonWithLabel("Delete group")
		leave.AddCSSClass("secondary")
		leave.ConnectClicked(func() {
			u.confirmDeleteGroup(g)
		})
		page.Append(leave)
		page.Append(gapBox(4))
		page.Append(cardText("This ends any running session and revokes every member.", "desc"))
	} else {
		page.Append(u.friendSessionCard(g, showError, live))
		page.Append(sectionTitle("Ready to play", ""))
		page.Append(u.friendServices(g))
		page.Append(sectionTitle("People", ""))
		page.Append(u.friendPeople(g, live))
		page.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
		page.Append(gapBox(10))
		leave := gtk.NewButtonWithLabel("Leave group")
		leave.AddCSSClass("secondary")
		leave.ConnectClicked(func() {
			u.confirmLeaveGroup(g)
		})
		page.Append(leave)
		page.Append(gapBox(4))
		page.Append(cardText("This removes the saved group and its device key from this PC.", "desc"))
	}

	return page, live
}

func (u *probeUI) hostSessionCard(g group.Group, showError func(error), state *groupLive) *gtk.Box {
	live := u.state.LiveGroupID() == g.ID
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginBottom(10)

	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	dot := statusDot(live)
	state.hostDot = dot
	top.Append(dot)
	status := lineLabel("Session off", "card-title")
	if live {
		status.SetText("Session live")
	}
	state.hostStatus = status
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
			u.confirmEndSession(g)
			return
		}
		action.SetSensitive(false)
		action.SetLabel("Starting…")
		go func() {
			err := u.state.StartSession(g.ID)
			glib.IdleAdd(func() {
				if err != nil {
					action.SetLabel("Start session")
					action.SetSensitive(true)
					showError(err)
					return
				}
				u.showGroup(g.ID)
			})
		}()
	})
	top.Append(action)
	card.Append(top)
	card.Append(gapBox(10))
	card.Append(cardText("Hosting as "+g.HostName+" · "+plural(len(g.Members), "member"), "desc"))
	card.Append(gapBox(8))

	endpoint, endpointErr := u.state.GroupEndpoint(g.ID)
	connection, connectionErr := u.state.EffectiveConnection(g.ID)
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	if endpointErr != nil {
		addr := lineLabel("not set", "mono")
		addr.SetHExpand(true)
		row.Append(addr)
	} else {
		addr := lineLabel(endpoint, "mono")
		addr.SetHExpand(true)
		row.Append(addr)
	}
	if connectionErr == nil {
		if connection.Custom {
			row.Append(tagLabel("Custom"))
		} else {
			row.Append(tagLabel("Default"))
		}
		if connection.UseRelay {
			row.Append(tagLabel("Relay"))
		}
	}
	change := gtk.NewButtonWithLabel("Change")
	change.AddCSSClass("quiet")
	change.ConnectClicked(func() {
		u.showGroupConnectionDialog(g, showError)
	})
	row.Append(change)
	card.Append(row)
	if endpointErr != nil {
		card.Append(gapBox(4))
		if connectionErr == nil && connection.UseRelay {
			card.Append(cardText("The relay is not configured. Set it in Settings or use a custom relay for this group.", "warning"))
		} else {
			card.Append(cardText("No public address — friends can't join.", "warning"))
		}
	}
	if connectionErr == nil && connection.UseRelay {
		card.Append(cardText("Through the relay — no port forward needed.", "desc"))
	} else {
		appPort, portErr := u.state.GroupAppPort(g.ID)
		if portErr != nil {
			appPort = u.state.Settings().AppPort
		}
		card.Append(cardText("UDP "+strconv.Itoa(appPort)+" on this PC", "desc"))
	}
	card.Append(gapBox(6))
	connected := cardText("not running", "desc")
	if live {
		presence := u.state.HostPresence()
		count := 0
		for _, p := range presence {
			if p.Live {
				count++
			}
		}
		connected.SetText(fmt.Sprintf("%d connected", count))
	}
	state.hostConnected = connected
	card.Append(connected)
	if note := u.state.HostNote(); note != "" {
		card.Append(gapBox(6))
		card.Append(cardText(note, "warning"))
	}
	return card
}

func (u *probeUI) hostServices(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	if len(g.Services) == 0 {
		box.Append(cardText("Nothing published yet. Add the game server you want friends to reach.", "desc"))
		return box
	}
	for i, s := range g.Services {
		if i > 0 {
			box.Append(gapBox(8))
		}
		card := u.hostServiceCard(s)
		addClick(card, func() { u.showServiceDialog(g, s) })
		box.Append(card)
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
	card.Append(cardText("Friends open 127.0.0.1:"+group.FormatPorts(s.Ports), "desc"))
	game := s.Target
	if address, err := s.GameAddress(); err == nil {
		game = address
	}
	card.Append(cardText("Game server "+game, "desc"))
	if s.Hint != "" {
		card.Append(gapBox(6))
		card.Append(cardText(s.Hint, "desc"))
	}
	return card
}

func (u *probeUI) hostPeople(g group.Group, state *groupLive) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	hostCard, _, _ := personCardParts(g.HostName+" (you)", "this device", true, "host")
	box.Append(hostCard)

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
		card, dot, detailLabel := personCardPartsWithDevice(m.Name, detail, m.DeviceName, live, "")
		state.hostPeople[m.ID] = &livePerson{dot: dot, detail: detailLabel}
		addClick(card, func() { u.showMemberDialog(g, m) })
		box.Append(card)
	}
	if len(g.Members) == 0 {
		box.Append(gapBox(8))
		box.Append(cardText("No friends yet. Create an invite when you're ready.", "desc"))
	}
	return box
}

func (u *probeUI) hostInvites(g group.Group, state *groupLive) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	open := g.OpenInvites(time.Now())
	if len(open) == 0 {
		box.Append(cardText("No open invites.", "desc"))
		return box
	}
	for i, inv := range open {
		if i > 0 {
			box.Append(gapBox(8))
		}
		row := gtk.NewBox(gtk.OrientationHorizontal, 8)
		row.AddCSSClass("invite-row")
		row.SetHExpand(true)
		name := lineLabel(inv.Name, "row-title")
		name.SetHExpand(true)
		row.Append(name)
		expiry := lineLabel(expiresIn(inv.ExpiresAt), "row-meta")
		state.inviteExpiry[inv.ID] = expiry
		row.Append(expiry)
		if code := u.state.InviteCode(inv.ID); code != "" {
			copy := gtk.NewButtonWithLabel("Copy")
			copy.AddCSSClass("quiet")
			copy.ConnectClicked(func() {
				u.win.Widget.Clipboard().SetText(code)
				copy.SetLabel("Copied")
			})
			row.Append(copy)
		}
		cancel := gtk.NewButtonWithLabel("Cancel")
		cancel.AddCSSClass("quiet")
		cancel.ConnectClicked(func() { u.confirmCancelInvite(g, inv) })
		row.Append(cancel)
		box.Append(row)
	}
	return box
}

func (u *probeUI) friendSessionCard(g group.Group, showError func(error), state *groupLive) *gtk.Box {
	presence := u.state.FriendPresence()
	reconnect := u.state.ReconnectStatus(g.ID)
	live := u.state.LiveGroupID() == g.ID && presence.Live
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginBottom(10)

	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	dot := statusDot(live)
	state.friendDot = dot
	top.Append(dot)
	status := lineLabel("Host offline", "card-title")
	if live {
		status.SetText("Host live")
	} else if reconnect.Active {
		status.SetText("Reconnecting")
	}
	state.friendStatus = status
	top.Append(status)
	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	top.Append(spacer)

	actionLabel := "Reconnect"
	if live {
		actionLabel = "Disconnect"
	} else if g.UseRelay {
		actionLabel = "Join new invite"
	} else if reconnect.Active {
		actionLabel = "Reconnect now"
	}
	action := gtk.NewButtonWithLabel(actionLabel)
	action.AddCSSClass("primary")
	state.friendAction = action
	action.ConnectClicked(func() {
		presence := u.state.FriendPresence()
		if u.state.LiveGroupID() == g.ID && presence.Live {
			u.confirmDisconnect(g)
			return
		}
		if g.UseRelay {
			u.showJoinDialog()
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
	card.Append(cardText("Hosted by "+g.HostName, "desc"))
	detail := "Start the session on the host, then reconnect."
	if g.UseRelay {
		detail = "Ask the host for a new invite, then join with it."
	}
	switch {
	case live:
		detail = presence.RTT.Round(time.Millisecond).String()
	case reconnect.Active:
		detail = reconnectWaitText(reconnect)
	case u.state.FriendNote() != "":
		detail = u.state.FriendNote()
	case reconnect.Stopped:
		detail = reconnectStoppedText(reconnect)
	}
	detailLabel := cardText(detail, "desc")
	state.friendDetail = detailLabel
	card.Append(detailLabel)
	return card
}

func reconnectWaitText(status apppkg.ReconnectStatus) string {
	if status.Next.IsZero() {
		return "Trying now…"
	}
	remaining := time.Until(status.Next)
	if remaining <= 0 {
		return "Trying now…"
	}
	seconds := int(remaining.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return "Next try in " + strconv.Itoa(seconds) + "s."
}

func reconnectStoppedText(status apppkg.ReconnectStatus) string {
	if strings.TrimSpace(status.Reason) != "" {
		return status.Reason
	}
	return "Auto-reconnect stopped. Reconnect when the host is back."
}

func (u *probeUI) friendServices(g group.Group) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	live := u.state.LiveGroupID() == g.ID && u.state.FriendPresence().Live
	if !live {
		if len(g.Services) == 0 {
			box.Append(cardText("No service open. The host has not published anything yet.", "desc"))
			return box
		}
		for i, s := range g.Services {
			if i > 0 {
				box.Append(gapBox(8))
			}
			box.Append(u.offlineFriendServiceCard(s, g.UseRelay))
		}
		return box
	}

	bounds := u.state.FriendBounds()
	if len(bounds) == 0 {
		box.Append(cardText("No service open. The host has not published anything yet.", "desc"))
		return box
	}
	views := buildFriendServiceViews(bounds)
	for i, view := range views {
		if i > 0 {
			box.Append(gapBox(8))
		}
		card := gtk.NewBox(gtk.OrientationVertical, 0)
		card.AddCSSClass("card")
		top := gtk.NewBox(gtk.OrientationHorizontal, 8)
		name := view.name
		if name == "" {
			name = "Service"
		}
		title := lineLabel(name, "row-title")
		title.SetHExpand(true)
		top.Append(title)
		if proto := protocolSetName(view.protocols); proto != "" {
			top.Append(tagLabel(proto))
		}
		card.Append(top)
		card.Append(gapBox(8))
		for _, address := range view.addresses {
			card.Append(cardText(address.address, "mono"))
		}
		if len(view.addresses) == 0 && len(view.errors) == 0 {
			card.Append(cardText("The local port could not be opened.", "warning"))
		}
		for _, message := range view.errors {
			card.Append(cardText(message, "warning"))
		}
		for _, note := range view.notes {
			card.Append(cardText(note, "desc"))
		}
		if view.hint != "" {
			card.Append(gapBox(6))
			card.Append(cardText(view.hint, "desc"))
		}
		box.Append(card)
	}
	return box
}

func (u *probeUI) offlineFriendServiceCard(s group.Service, relay bool) *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	top := gtk.NewBox(gtk.OrientationHorizontal, 8)
	name := lineLabel(s.Name, "row-title")
	name.SetHExpand(true)
	top.Append(name)
	top.Append(tagLabel(protoName(s.Proto)))
	card.Append(top)
	card.Append(gapBox(8))
	card.Append(cardText("Port "+group.FormatPorts(s.Ports), "desc"))
	if relay {
		card.Append(cardText("Host offline. Ask the host for a new invite.", "desc"))
	} else {
		card.Append(cardText("Host offline. Reconnect when the host is back.", "desc"))
	}
	if s.Hint != "" {
		card.Append(gapBox(6))
		card.Append(cardText(s.Hint, "desc"))
	}
	return card
}

func (u *probeUI) friendPeople(g group.Group, state *groupLive) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	presence := u.state.FriendPresence()
	live := u.state.LiveGroupID() == g.ID && presence.Live
	detail := "host"
	if live {
		detail = "host · " + presence.RTT.Round(time.Millisecond).String()
	}
	hostCard, dot, detailLabel := personCardParts(g.HostName, detail, live, "host")
	state.friendHostDot = dot
	state.friendHostDetail = detailLabel
	box.Append(hostCard)
	box.Append(gapBox(8))
	self := g.SelfName
	if strings.TrimSpace(self) == "" {
		self = "you"
	}
	selfCard, _, _ := personCardParts("You ("+self+")", u.state.DeviceName(), true, "you")
	box.Append(selfCard)
	return box
}

func personCardParts(name, detail string, live bool, role string) (*gtk.Box, *gtk.Label, *gtk.Label) {
	return personCardPartsWithDevice(name, detail, "", live, role)
}

func personCardPartsWithDevice(name, detail, device string, live bool, role string) (*gtk.Box, *gtk.Label, *gtk.Label) {
	card := gtk.NewBox(gtk.OrientationHorizontal, 8)
	card.AddCSSClass("card")
	dot := statusDot(live)
	card.Append(dot)
	text := gtk.NewBox(gtk.OrientationVertical, 0)
	title := gtk.NewBox(gtk.OrientationHorizontal, 6)
	title.Append(lineLabel(name, "row-title"))
	if role != "" {
		title.Append(tagLabel(role))
	}
	if device = strings.TrimSpace(device); device != "" {
		tag := tagLabel(truncateText(device, 18))
		tag.SetTooltipText(device)
		title.Append(tag)
	}
	text.Append(title)
	detailLabel := cardText(detail, "desc")
	text.Append(detailLabel)
	card.Append(text)
	return card, dot, detailLabel
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
	return sectionTitleAction(title, count, "", nil)
}

func sectionTitleAction(title, count, action string, onAction func()) *gtk.Box {
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
	if action != "" && onAction != nil {
		button := gtk.NewButtonWithLabel(action)
		button.AddCSSClass("quiet")
		button.ConnectClicked(onAction)
		row.Append(button)
	}
	return row
}

func (u *probeUI) groupSignature(g group.Group) string {
	var b strings.Builder
	fmt.Fprintf(&b, "group|%s|%s|%s|%s|%s|%s|%s", g.ID, g.Name, g.Role, g.HostName, g.Endpoint, g.HostPin, g.SelfName)
	if g.IsHost() {
		live := u.state.LiveGroupID() == g.ID
		fmt.Fprintf(&b, "|live=%t|host-note=%s", live, u.state.HostNote())
		fmt.Fprintf(&b, "|connection=%s,%d", g.PublicAddress, g.AppPort)
		if connection, err := u.state.EffectiveConnection(g.ID); err == nil {
			fmt.Fprintf(&b, "|use-relay=%t,relay-custom=%t,relay=%s,%d,%s,override=%t",
				connection.UseRelay, connection.RelayCustom, connection.RelayHost,
				connection.RelayPort, connection.RelayID, connection.Custom)
		} else {
			fmt.Fprintf(&b, "|connection-error=%s", err)
		}
		if endpoint, err := u.state.GroupEndpoint(g.ID); err == nil {
			fmt.Fprintf(&b, "|endpoint=%s", endpoint)
		} else {
			fmt.Fprintf(&b, "|endpoint-error=%s", err)
		}
		if appPort, err := u.state.GroupAppPort(g.ID); err == nil {
			fmt.Fprintf(&b, "|app-port=%d", appPort)
		} else {
			fmt.Fprintf(&b, "|app-port-error=%s", err)
		}
		for _, s := range g.Services {
			fmt.Fprintf(&b, "|service=%s,%s,%s,%s,%s,%s", s.ID, s.Name, s.Proto, s.Target, group.FormatPorts(s.Ports), s.Hint)
		}
		for _, m := range g.Members {
			fmt.Fprintf(&b, "|member=%s,%s,%s", m.ID, m.Name, m.State)
		}
		for _, inv := range g.OpenInvites(time.Now()) {
			fmt.Fprintf(&b, "|invite=%s,%s,%t", inv.ID, inv.Name, u.state.InviteCode(inv.ID) != "")
		}
		return b.String()
	}

	presence := u.state.FriendPresence()
	live := u.state.LiveGroupID() == g.ID && presence.Live
	fmt.Fprintf(&b, "|live=%t|note=%s", live, u.state.FriendNote())
	if live {
		for _, bound := range u.state.FriendBounds() {
			fmt.Fprintf(&b, "|bound=%s,%s,%s,%d,%d,%s,%s,%s", bound.ServiceID, bound.Service, bound.Proto, bound.PublishedPort, bound.Port, bound.Address, bound.Error, bound.Hint)
		}
		return b.String()
	}
	for _, s := range g.Services {
		fmt.Fprintf(&b, "|service=%s,%s,%s,%s,%s", s.ID, s.Name, s.Proto, group.FormatPorts(s.Ports), s.Hint)
	}
	return b.String()
}

// refreshGroup updates the visible group screen once a second. Presence and
// invite countdowns change in place; structural changes rebuild the page.
func (u *probeUI) refreshGroup() {
	state := u.groupState
	if state == nil || u.stack.VisibleChildName() != "group" {
		return
	}
	g, ok := u.state.Group(state.groupID)
	if !ok {
		u.showHome()
		return
	}
	if u.groupSignature(g) != state.signature {
		u.showGroup(g.ID)
		return
	}
	state.refresh(u, g)
}

func (state *groupLive) refresh(u *probeUI, g group.Group) {
	now := time.Now()
	if g.IsHost() {
		live := u.state.LiveGroupID() == g.ID
		setStatusDot(state.hostDot, live)
		if state.hostStatus != nil {
			if live {
				state.hostStatus.SetText("Session live")
			} else {
				state.hostStatus.SetText("Session off")
			}
		}

		presence := u.state.HostPresence()
		connected := 0
		for _, p := range presence {
			if p.Live {
				connected++
			}
		}
		if state.hostConnected != nil {
			if live {
				state.hostConnected.SetText(fmt.Sprintf("%d connected", connected))
			} else {
				state.hostConnected.SetText("not running")
			}
		}

		for _, m := range g.Members {
			person := state.hostPeople[m.ID]
			if person == nil {
				continue
			}
			detail := "idle"
			memberLive := false
			if !m.IsApproved() {
				detail = "waiting for approval"
			} else if p, ok := presence[m.ID]; ok && p.Live {
				memberLive = true
				detail = "live · " + p.RTT.Round(time.Millisecond).String()
			}
			person.detail.SetText(detail)
			setStatusDot(person.dot, memberLive)
		}

		for _, inv := range g.OpenInvites(now) {
			if label := state.inviteExpiry[inv.ID]; label != nil {
				label.SetText(expiresIn(inv.ExpiresAt))
			}
		}
		return
	}

	presence := u.state.FriendPresence()
	reconnect := u.state.ReconnectStatus(g.ID)
	live := u.state.LiveGroupID() == g.ID && presence.Live
	setStatusDot(state.friendDot, live)
	if state.friendStatus != nil {
		switch {
		case live:
			state.friendStatus.SetText("Host live")
		case reconnect.Active:
			state.friendStatus.SetText("Reconnecting")
		default:
			state.friendStatus.SetText("Host offline")
		}
	}
	if state.friendAction != nil {
		switch {
		case live:
			state.friendAction.SetLabel("Disconnect")
		case g.UseRelay:
			state.friendAction.SetLabel("Join new invite")
		case reconnect.Active:
			state.friendAction.SetLabel("Reconnect now")
		default:
			state.friendAction.SetLabel("Reconnect")
		}
	}
	if state.friendDetail != nil {
		switch {
		case live:
			state.friendDetail.SetText(presence.RTT.Round(time.Millisecond).String())
		case reconnect.Active:
			state.friendDetail.SetText(reconnectWaitText(reconnect))
		case u.state.FriendNote() != "":
			state.friendDetail.SetText(u.state.FriendNote())
		case reconnect.Stopped:
			state.friendDetail.SetText(reconnectStoppedText(reconnect))
		case g.UseRelay:
			state.friendDetail.SetText("Ask the host for a new invite, then join with it.")
		default:
			state.friendDetail.SetText("Start the session on the host, then reconnect.")
		}
	}
	if state.friendHostDetail != nil {
		detail := "host"
		if live {
			detail = "host · " + presence.RTT.Round(time.Millisecond).String()
		}
		state.friendHostDetail.SetText(detail)
	}
	setStatusDot(state.friendHostDot, live)
}

func setStatusDot(dot *gtk.Label, live bool) {
	if dot == nil {
		return
	}
	if live {
		dot.SetCSSClasses([]string{"dot"})
	} else {
		dot.SetCSSClasses([]string{"dot-off"})
	}
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

func protoValue(index int) string {
	switch index {
	case 0:
		return "tcp"
	case 1:
		return "udp"
	default:
		return "both"
	}
}

func protoIndex(proto string) int {
	switch proto {
	case "udp":
		return 1
	case "both":
		return 2
	default:
		return 0
	}
}
