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
	top := container.NewBorder(nil, nil, newButton("Back", buttonQuiet, a.showHome), a.statusPill(), layout.NewSpacer())

	header := container.NewVBox(
		top,
		gap(8),
		titleText(g.Name),
		metaText("Host: "+g.HostName),
	)

	a.memberViews = map[string]*memberView{}

	body := container.NewVBox(a.sessionBanner(g))

	body.Add(gap(20))
	body.Add(sectionLabel("Published"))
	body.Add(gap(8))

	if len(g.Services) == 0 {
		body.Add(metaText("Nothing published yet."))
		body.Add(gap(8))
	}
	for i, s := range g.Services {
		if i > 0 {
			body.Add(gap(8))
		}
		body.Add(a.serviceCard(g, s))
	}

	add := newButton("Add service", buttonSecondary, func() { a.addService(g) })
	if a.live != nil {
		add.disabled = true
	}
	body.Add(add)

	body.Add(gap(20))
	body.Add(sectionLabel(fmt.Sprintf("Group · %d members", len(g.Members))))
	body.Add(gap(4))
	for _, m := range g.Members {
		row, view := memberRow(m)
		body.Add(row)
		if m.PublicKey != "" {
			a.memberViews[m.PublicKey] = view
		}
	}

	body.Add(gap(20))
	body.Add(a.inviteCard(g))
	body.Add(gap(8))

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(header),
		a.footer(),
		nil,
		nil,
		container.NewScroll(pad(0, 12, body)),
	)))
	a.win.Canvas().Unfocus()
	a.win.SetTitle("LAN Party · " + g.Name)
}

func (a *App) statusPill() fyne.CanvasObject {
	if a.live != nil {
		return pill("Live", "#4ade80")
	}
	return pill("Idle", "#6f737d")
}

// sessionBanner is the strip at the top: state on the left, the one action on
// the right.
func (a *App) sessionBanner(g group.Group) fyne.CanvasObject {
	live := a.live != nil

	fill := hex("#141b17")
	border := hex("#24402e")
	titleColour := "#8de3ac"
	title := "Session live"

	if !live {
		fill = hex("#17181c")
		border = hex("#2a2d34")
		titleColour = "#e8eaee"
		title = "No session running"
	}

	heading := canvas.NewText(title, hex(titleColour))
	heading.TextSize = sizeBody
	heading.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := canvas.NewText(`group "`+g.Name+`"`, hex("#6f737d"))
	subtitle.TextSize = sizeMeta
	if g.Endpoint != "" {
		subtitle.Text = subtitle.Text + " · " + g.Endpoint
	} else {
		subtitle.Text = "set a public address so friends can join"
		subtitle.Color = hex("#e0b341")
	}

	left := container.NewVBox(heading, gap(2), subtitle)
	button := a.sessionButton(g)

	content := container.NewBorder(nil, nil, nil,
		container.NewVBox(layout.NewSpacer(), button, layout.NewSpacer()), left)

	return container.NewStack(surface(fill, border, 12), pad(14, 16, content))
}

func (a *App) sessionButton(g group.Group) fyne.CanvasObject {
	if a.live != nil {
		return newButton("End session", buttonDanger, func() { a.endSession(g) })
	}
	return newButton("Start", buttonPrimary, func() { a.startSession(g) })
}

func (a *App) serviceCard(g group.Group, s group.Service) fyne.CanvasObject {
	letter := strings.ToUpper(s.Name[:1])

	name := canvas.NewText(s.Name, hex("#e8eaee"))
	name.TextSize = 13
	name.TextStyle = fyne.TextStyle{Bold: true}

	right := []fyne.CanvasObject{metaText("friends see")}
	if len(s.Ports) > 0 {
		address := canvas.NewText(fmt.Sprintf("127.0.0.1:%d", s.Ports[0]), hex("#5ad1e6"))
		address.TextSize = 13
		address.TextStyle = fyne.TextStyle{Monospace: true}
		right = append(right, address)
	}
	if peers := len(g.Members) - 1; peers > 0 {
		right = append(right, metaText(fmt.Sprintf("%d of %d can reach", peers, peers)))
	}

	left := container.NewHBox(
		tile(letter),
		gapW(10),
		container.NewVBox(name, gap(2), metaText(s.Target+" · "+strings.ToUpper(s.Proto))),
	)

	content := container.NewBorder(nil, nil, left,
		container.NewVBox(layout.NewSpacer(), container.New(rightLayout{}, right...), layout.NewSpacer()), nil)

	body := card(content)
	return container.NewStack(body, newTapArea(func() { a.serviceDialog(g, s) }))
}

// serviceDialog shows what a service publishes and lets the host remove it.
func (a *App) serviceDialog(g group.Group, s group.Service) {
	lines := container.NewVBox(
		metaText(s.Proto+" · "+group.FormatPorts(s.Ports)),
		metaText("on your LAN: "+s.Target),
	)
	if len(s.Ports) > 0 {
		lines.Add(gap(8))
		lines.Add(addressText(fmt.Sprintf("127.0.0.1:%d", s.Ports[0])))
		lines.Add(metaText("the address friends type"))
	}

	remove := newButton("Remove service", buttonDanger, func() { a.removeService(g, s.ID) })
	if a.live != nil {
		remove.disabled = true
	}

	body := container.NewVBox(lines, gap(16), container.NewHBox(layout.NewSpacer(), remove))

	d := dialog.NewCustom(s.Name, "Close", body, a.win)
	d.Resize(fyne.NewSize(400, 260))
	d.Show()
}

func (a *App) confirmRemove(g group.Group, s group.Service) {
	dialog.ShowConfirm("Remove service",
		fmt.Sprintf("Stop publishing %s?", s.Name),
		func(ok bool) {
			if ok {
				a.removeService(g, s.ID)
			}
		}, a.win)
}

func (a *App) inviteCard(g group.Group) fyne.CanvasObject {
	if len(g.Invites) == 0 {
		return card(container.NewBorder(nil, nil,
			container.NewVBox(nameText("No invites yet"), metaText("one person per invite")),
			newButton("Invite a friend", buttonSecondary, func() { a.newInvite(g) }),
			nil,
		))
	}

	latest := g.Invites[len(g.Invites)-1]

	detail := "expires in " + timeLeft(latest.Expires)
	if !time.Now().Before(latest.Expires) {
		detail = "expired"
	}

	left := container.NewVBox(nameText("Invite for "+latest.PeerName), metaText(detail))

	right := container.NewHBox(
		newButton("Copy", buttonSecondary, func() {
			a.fyneApp.Clipboard().SetContent(latest.Value)
		}),
		gapW(8),
		newButton("New invite", buttonPrimary, func() { a.newInvite(g) }),
	)

	return card(container.NewBorder(nil, nil, nil, right, left))
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
			"Friends need an address to connect to. Set a public address, then invite them.",
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

	copyButton := newButton("Copy", buttonPrimary, func() {
		a.fyneApp.Clipboard().SetContent(code)
	})

	body := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), copyButton),
		nil,
		nil,
		container.NewVBox(
			metaText(fmt.Sprintf("For %s. One use, and it expires in 24 hours.", name)),
			entry,
		),
	)

	d := dialog.NewCustom("Invite for "+name, "Close", body, a.win)
	d.Resize(fyne.NewSize(460, 300))
	d.Show()
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
	dot := canvas.NewText("●", hex("#6f737d"))
	dot.TextSize = 8

	status := canvas.NewText("waiting", hex("#6f737d"))
	status.TextSize = sizeMeta

	row := container.NewHBox(container.NewCenter(dot), gapW(10))

	name := canvas.NewText(m.Name, hex("#e8eaee"))
	name.TextSize = sizeBody
	row.Add(name)

	if m.IsHost {
		dot.Color = hex("#4ade80")
		status.Text = "—"
		row.Add(tag("host"))
	} else if m.BoxAddress != "" {
		row.Add(tag(m.BoxAddress))
	}

	row.Add(layout.NewSpacer())
	row.Add(status)

	return pad(7, 0, row), &memberView{dot: dot, status: status}
}
