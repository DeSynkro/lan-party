package ui

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/preset"
	"github.com/desynkro/lan-party/internal/session"
	"github.com/desynkro/lan-party/internal/tunnel"
)

// showGroup is the host's view of a group.
func (a *App) showGroup(g group.Group) {
	top := container.NewBorder(nil, nil, quietButton("Back", a.showHome), a.statusPill(), layout.NewSpacer())

	header := container.NewVBox(
		top,
		titleText(g.Name),
		muted("Host: "+g.HostName),
	)

	body := container.NewVBox(
		a.addressCard(g),
		gap(8),
		a.sessionCard(g),
	)

	a.memberViews = map[string]*memberView{}
	members := container.NewVBox(sectionHeader(fmt.Sprintf("Members · %d", len(g.Members)), nil))
	for _, m := range g.Members {
		row, view := memberRow(m)
		members.Add(row)
		if m.PublicKey != "" {
			a.memberViews[m.PublicKey] = view
		}
	}
	if g.Endpoint != "" {
		members.Add(widget.NewButton("Invite a friend", func() { a.newInvite(g) }))
	}

	services := container.NewVBox(sectionHeader("Services", nil))
	if len(g.Services) == 0 {
		services.Add(muted("Nothing published yet."))
	}
	for _, s := range g.Services {
		services.Add(a.serviceCard(g, s))
	}
	add := widget.NewButton("Add service", func() { a.addService(g) })
	if a.live != nil {
		add.Disable()
		services.Add(add)
		services.Add(muted("End the session to change services."))
	} else {
		services.Add(add)
	}

	body.Add(gap(12))
	body.Add(members)
	body.Add(gap(12))
	body.Add(services)

	if len(g.Invites) > 0 {
		body.Add(gap(12))
		body.Add(a.invitesSection(g))
	}

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(header),
		a.footer(),
		nil,
		nil,
		container.NewScroll(body),
	)))
	a.win.SetTitle("LAN Party · " + g.Name)
}

func (a *App) statusPill() fyne.CanvasObject {
	if a.live != nil {
		return pill("LIVE", "#22c55e")
	}
	return pill("IDLE", "#6b7280")
}

// addressCard shows the address friends dial, and says so when it is missing.
func (a *App) addressCard(g group.Group) fyne.CanvasObject {
	label := muted("Public address")

	var value fyne.CanvasObject
	if g.Endpoint == "" {
		value = canvas.NewText("Not set. Friends cannot join until you set one.", hex("#eab308"))
		value.(*canvas.Text).TextSize = 12
	} else {
		value = addressText(g.Endpoint)
	}

	action := widget.NewButton("Set", func() { a.setEndpoint(g) })
	if g.Endpoint != "" {
		action.SetText("Change")
	}

	row := container.NewBorder(nil, nil, nil, action, container.NewVBox(label, value))
	return card(row)
}

func (a *App) sessionCard(g group.Group) fyne.CanvasObject {
	state := "No session running"
	if a.live != nil {
		state = "Session live"
	}
	label := canvas.NewText(state, hex("#e9edf5"))
	label.TextSize = 13

	row := container.NewBorder(nil, nil, nil, a.sessionButton(g), label)
	return card(row)
}

func (a *App) sessionButton(g group.Group) fyne.CanvasObject {
	if a.live != nil {
		end := widget.NewButton("End", func() { a.endSession(g) })
		end.Importance = widget.DangerImportance
		return end
	}

	start := widget.NewButton("Start", func() { a.startSession(g) })
	start.Importance = widget.HighImportance
	return start
}

func (a *App) invitesSection(g group.Group) fyne.CanvasObject {
	rows := container.NewVBox(sectionHeader("Invites", nil))

	for _, code := range g.Invites {
		code := code
		detail := muted("expired")
		if time.Now().Before(code.Expires) {
			detail = muted("expires in " + timeLeft(code.Expires))
		}

		show := quietButton("Show", func() { a.showCode(code.PeerName, code.Value) })
		name := canvas.NewText(code.PeerName, hex("#e9edf5"))
		name.TextSize = 13

		action := container.NewVBox(layout.NewSpacer(), show, layout.NewSpacer())
		row := container.NewBorder(nil, nil, nil, action,
			container.NewVBox(name, detail))
		rows.Add(row)
		rows.Add(gap(6))
	}
	return rows
}

func timeLeft(t time.Time) string {
	d := time.Until(t)
	switch {
	case d <= 0:
		return "expired"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func (a *App) startSession(g group.Group) {
	if g.PrivateKey == "" {
		dialog.ShowError(errors.New("this group has no host key"), a.win)
		return
	}
	key, err := wgtypes.ParseKey(g.PrivateKey)
	if err != nil {
		dialog.ShowError(errors.New("this group has a damaged host key"), a.win)
		return
	}

	peers, err := groupPeers(g)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}

	live, err := session.StartHost(session.HostConfig{
		PrivateKey: key,
		ListenPort: uint16(g.ListenPort),
		Peers:      peers,
		Services:   g.Services,
	})
	if err != nil {
		dialog.ShowError(fmt.Errorf("could not start the session: %w", err), a.win)
		return
	}

	a.live = live
	a.showGroup(g)
}

func (a *App) endSession(g group.Group) {
	if a.live != nil {
		_ = a.live.Close()
		a.live = nil
	}
	a.showGroup(g)
}

func (a *App) setEndpoint(g group.Group) {
	entry := widget.NewEntry()
	entry.SetText(g.Endpoint)
	entry.SetPlaceHolder("lanparty.example.com:51820")

	form := dialog.NewForm("Public address", "Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Address friends connect to", entry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			value := strings.TrimSpace(entry.Text)
			if value != "" {
				if err := validHostPort(value); err != nil {
					dialog.ShowError(err, a.win)
					return
				}
			}
			g.Endpoint = value
			if err := a.save(g); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.showGroup(g)
		}, a.win)
	form.Resize(fyne.NewSize(420, 190))
	form.Show()
}

func (a *App) newInvite(g group.Group) {
	if g.Endpoint == "" {
		dialog.ShowInformation("Set the address first",
			"Friends need an address to connect to. Press Set next to the public address, then invite them.",
			a.win)
		return
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Ash")

	form := dialog.NewForm("Invite a friend", "Create invite", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Their name", nameEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				dialog.ShowError(errors.New("give them a name so you can tell them apart"), a.win)
				return
			}

			peerKey, err := wgtypes.GeneratePrivateKey()
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			box := g.NextBoxAddress()
			if box == "" {
				dialog.ShowError(errors.New("this group is full"), a.win)
				return
			}

			g.Members = append(g.Members, group.Member{
				Name:       name,
				PublicKey:  peerKey.PublicKey().String(),
				BoxAddress: box,
			})

			expires := time.Now().Add(24 * time.Hour)
			inv := invite.Invite{
				Version:   1,
				GroupID:   g.ID,
				GroupName: g.Name,
				HostName:  g.HostName,
				Endpoint:  g.Endpoint,
				HostKey:   hostPublicKey(g),
				PeerName:  name,
				PeerKey:   peerKey.String(),
				PeerBox:   box,
				Expires:   expires,
				Services:  inviteServices(g),
			}

			code, err := invite.Encode(inv)
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}

			g.Invites = append(g.Invites, group.Code{
				Value:    code,
				PeerName: name,
				Expires:  expires,
			})
			if err := a.save(g); err != nil {
				dialog.ShowError(err, a.win)
				return
			}

			if a.live != nil {
				_ = a.live.AddPeer(tunnel.Peer{
					PublicKey:  peerKey.PublicKey(),
					AllowedIPs: allowedFor(box),
				})
			}

			a.showGroup(g)
			a.showCode(name, code)
		}, a.win)
	form.Resize(fyne.NewSize(380, 200))
	form.Show()
}

func (a *App) showCode(name, code string) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(code)
	entry.Wrapping = fyne.TextWrapBreak

	copyButton := widget.NewButton("Copy", func() {
		a.fyneApp.Clipboard().SetContent(code)
	})
	copyButton.Importance = widget.HighImportance

	body := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), copyButton),
		nil,
		nil,
		container.NewVBox(
			muted(fmt.Sprintf("For %s. One use, and it expires in 24 hours.", name)),
			entry,
		),
	)

	d := dialog.NewCustom("Invite for "+name, "Close", body, a.win)
	d.Resize(fyne.NewSize(460, 300))
	d.Show()
}

func (a *App) serviceCard(g group.Group, s group.Service) fyne.CanvasObject {
	name := canvas.NewText(s.Name, hex("#e9edf5"))
	name.TextSize = 13
	name.TextStyle = fyne.TextStyle{Bold: true}

	detail := muted(fmt.Sprintf("%s · %s → %s", s.Proto, group.FormatPorts(s.Ports), s.Target))

	left := container.NewVBox(name, detail)
	if len(s.Ports) > 0 {
		left.Add(addressText(fmt.Sprintf("127.0.0.1:%d", s.Ports[0])))
	}

	remove := quietButton("Remove", func() { a.removeService(g, s.ID) })
	if a.live != nil {
		remove.Disable()
	}

	return card(container.NewBorder(nil, nil, nil, remove, left))
}

func (a *App) addService(g group.Group) {
	choices := []string{"Custom"}
	byName := map[string]preset.Preset{}
	for _, p := range preset.All() {
		choices = append(choices, p.Name)
		byName[p.Name] = p
	}

	pick := widget.NewSelect(choices, nil)
	name := widget.NewEntry()
	target := widget.NewEntry()
	target.SetPlaceHolder("192.168.1.50:25565")
	ports := widget.NewEntry()
	proto := widget.NewSelect([]string{"tcp", "udp", "both"}, nil)
	proto.SetSelected("tcp")

	pick.OnChanged = func(choice string) {
		p, ok := byName[choice]
		if !ok {
			return
		}
		name.SetText(p.Name)
		ports.SetText(group.FormatPorts(p.Ports))
		proto.SetSelected(p.Proto)
	}

	form := dialog.NewForm("Add a service", "Publish", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Preset", pick),
			widget.NewFormItem("Name", name),
			widget.NewFormItem("Address on your LAN", target),
			widget.NewFormItem("Ports", ports),
			widget.NewFormItem("Protocol", proto),
		},
		func(ok bool) {
			if !ok {
				return
			}

			parsedPorts, err := group.ParsePorts(ports.Text)
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			addr, loopback, err := group.ParseTarget(target.Text)
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			if !group.ValidProto(proto.Selected) {
				dialog.ShowError(errors.New("pick a protocol"), a.win)
				return
			}

			s := group.Service{
				ID:     group.NewID(),
				Name:   strings.TrimSpace(name.Text),
				Proto:  proto.Selected,
				Ports:  parsedPorts,
				Target: addr,
			}
			if s.Name == "" {
				s.Name = s.Target
			}

			g.Services = append(g.Services, s)
			if err := a.save(g); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.showGroup(g)

			if loopback {
				dialog.ShowInformation("Loopback target",
					"The server is only listening on this machine. Friends can reach it while it runs, but nothing else on your LAN can.",
					a.win)
			}
		}, a.win)
	form.Resize(fyne.NewSize(420, 340))
	form.Show()
}

func (a *App) removeService(g group.Group, id string) {
	kept := make([]group.Service, 0, len(g.Services))
	for _, s := range g.Services {
		if s.ID != id {
			kept = append(kept, s)
		}
	}
	g.Services = kept
	if err := a.save(g); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.showGroup(g)
}

// save writes the group back into the list the store knows about.
func (a *App) save(g group.Group) error {
	for i := range a.groups {
		if a.groups[i].ID == g.ID {
			a.groups[i] = g
			return a.store.Save(a.groups)
		}
	}
	return errors.New("group is no longer in the list")
}

func groupPeers(g group.Group) ([]tunnel.Peer, error) {
	var peers []tunnel.Peer

	for _, m := range g.Members {
		if m.IsHost || m.PublicKey == "" || m.BoxAddress == "" {
			continue
		}
		key, err := wgtypes.ParseKey(m.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("member %s has a damaged key", m.Name)
		}
		peers = append(peers, tunnel.Peer{
			PublicKey:  key,
			AllowedIPs: allowedFor(m.BoxAddress),
		})
	}
	return peers, nil
}

func allowedFor(box string) []net.IPNet {
	ip, network, err := net.ParseCIDR(box + "/32")
	if err != nil {
		return nil
	}
	return []net.IPNet{{IP: ip, Mask: network.Mask}}
}

func hostPublicKey(g group.Group) string {
	key, err := wgtypes.ParseKey(g.PrivateKey)
	if err != nil {
		return ""
	}
	return key.PublicKey().String()
}

func inviteServices(g group.Group) []invite.Service {
	out := make([]invite.Service, 0, len(g.Services))
	for _, s := range g.Services {
		hint := ""
		if p, ok := preset.ByID(s.ID); ok {
			hint = p.Hint
		}
		out = append(out, invite.Service{
			Name:  s.Name,
			Proto: s.Proto,
			Ports: append([]int(nil), s.Ports...),
			Hint:  hint,
		})
	}
	return out
}

func validHostPort(value string) error {
	host, portText, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		return errors.New("use address:port, like lanparty.example.com:51820")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("the port must be between 1 and 65535")
	}
	return nil
}

func memberRow(m group.Member) (fyne.CanvasObject, *memberView) {
	dot := canvas.NewText("●", hex("#6b7280"))
	dot.TextSize = 10

	status := widget.NewLabel("waiting")
	status.TextStyle = fyne.TextStyle{}

	row := container.NewHBox(dot, widget.NewLabel(m.Name), layout.NewSpacer(), status)
	if m.IsHost {
		dot.Color = hex("#22c55e")
		status.SetText("host")
	}

	return row, &memberView{dot: dot, status: status}
}
