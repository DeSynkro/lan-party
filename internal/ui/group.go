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
	"fyne.io/fyne/v2/theme"
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

	a.memberViews = map[string]*memberView{}

	body := container.NewVBox(a.sessionBanner(g))

	body.Add(gap(16))
	body.Add(caps("Published"))
	body.Add(gap(2))

	if len(g.Services) == 0 {
		body.Add(muted("Nothing published yet."))
	}
	for i, s := range g.Services {
		if i > 0 {
			body.Add(gap(6))
		}
		body.Add(a.serviceCard(g, s))
	}

	body.Add(gap(6))
	add := widget.NewButtonWithIcon("Add service", theme.ContentAddIcon(), func() { a.addService(g) })
	if a.live != nil {
		add.Disable()
	}
	body.Add(add)
	if a.live != nil {
		body.Add(muted("End the session to change services."))
	}

	body.Add(gap(16))
	body.Add(caps(fmt.Sprintf("Group · %d members", len(g.Members))))
	body.Add(gap(2))
	for _, m := range g.Members {
		row, view := memberRow(m)
		body.Add(row)
		if m.PublicKey != "" {
			a.memberViews[m.PublicKey] = view
		}
	}

	body.Add(gap(12))
	body.Add(a.inviteCard(g))

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

// sessionBanner is the green strip in the design: state on the left, the
// session button on the right.
func (a *App) sessionBanner(g group.Group) fyne.CanvasObject {
	live := a.live != nil

	fill := hex("#12251a")
	border := hex("#2c5a3c")
	titleColour := "#8ef0b4"
	title := "Session live · " + a.playingCount(g)

	if !live {
		fill = hex("#1b1d24")
		border = hex("#2b2d36")
		titleColour = "#e9edf5"
		title = "No session running"
	}

	heading := canvas.NewText(title, hex(titleColour))
	heading.TextSize = 13
	heading.TextStyle = fyne.TextStyle{Bold: true}

	detail := `group "` + g.Name + `"`
	if g.Endpoint != "" {
		detail += " · " + g.Endpoint
	}

	left := container.NewVBox(heading)
	if g.Endpoint == "" {
		warning := canvas.NewText("set a public address so friends can join", hex("#eab308"))
		warning.TextSize = 11
		left.Add(warning)
	} else {
		left.Add(muted(detail))
	}

	button := a.sessionButton(g)
	content := container.NewBorder(nil, nil, nil, container.NewVBox(layout.NewSpacer(), button, layout.NewSpacer()), left)

	return surface(fill, border, 10, container.NewPadded(content))
}

// playingCount is how many peers have completed a handshake recently.
func (a *App) playingCount(g group.Group) string {
	total := 0
	for _, m := range g.Members {
		if !m.IsHost {
			total++
		}
	}
	if a.live == nil || total == 0 {
		return fmt.Sprintf("0 of %d playing", total)
	}

	presence := a.live.Presence()
	playing := 0
	for _, m := range g.Members {
		if m.IsHost {
			continue
		}
		if last := presence[m.PublicKey]; !last.IsZero() && time.Since(last) < 2*time.Minute {
			playing++
		}
	}
	if playing == 1 {
		return fmt.Sprintf("1 of %d playing", total)
	}
	return fmt.Sprintf("%d of %d playing", playing, total)
}

func (a *App) sessionButton(g group.Group) fyne.CanvasObject {
	if a.live != nil {
		end := widget.NewButton("End session", func() { a.endSession(g) })
		end.Importance = widget.DangerImportance
		return end
	}

	start := widget.NewButton("Start", func() { a.startSession(g) })
	start.Importance = widget.HighImportance
	return start
}

func (a *App) serviceCard(g group.Group, s group.Service) fyne.CanvasObject {
	letter := strings.ToUpper(s.Name[:1])
	detail := s.Target + " · " + strings.ToUpper(s.Proto)

	name := canvas.NewText(s.Name, hex("#e9edf5"))
	name.TextSize = 13
	name.TextStyle = fyne.TextStyle{Bold: true}

	left := container.NewHBox(
		tile(letter),
		gapW(10),
		container.NewVBox(name, muted(detail)),
	)

	right := []fyne.CanvasObject{muted("friends see")}
	if len(s.Ports) > 0 {
		right = append(right, addressText(fmt.Sprintf("127.0.0.1:%d", s.Ports[0])))
	}
	right = append(right, muted(fmt.Sprintf("%d/%d can reach", len(g.Members)-1, len(g.Members)-1)))

	remove := widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() { a.confirmRemove(g, s) })
	remove.Importance = widget.LowImportance
	if a.live != nil {
		remove.Disable()
	}

	content := container.NewBorder(
		nil, nil,
		left,
		container.NewHBox(container.New(rightLayout{}, right...), gapW(8), remove),
		nil,
	)

	return card(container.NewPadded(content))
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
		create := widget.NewButton("Invite a friend", func() { a.newInvite(g) })
		return card(container.NewPadded(create))
	}

	latest := g.Invites[len(g.Invites)-1]

	name := canvas.NewText("Invite for "+latest.PeerName, hex("#e9edf5"))
	name.TextSize = 13

	detail := "expires in " + timeLeft(latest.Expires)
	if !time.Now().Before(latest.Expires) {
		detail = "expired"
	}

	left := container.NewVBox(name, muted(detail))

	copyButton := widget.NewButton("Copy", func() {
		a.fyneApp.Clipboard().SetContent(latest.Value)
	})
	newButton := widget.NewButton("New invite", func() { a.newInvite(g) })
	newButton.Importance = widget.HighImportance

	content := container.NewBorder(nil, nil, nil,
		container.NewHBox(copyButton, newButton), left)

	return card(container.NewPadded(content))
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

	row := container.NewHBox(dot, widget.NewLabel(m.Name))

	if m.IsHost {
		dot.Color = hex("#22c55e")
		status.SetText("—")
		row.Add(tag("host"))
	} else if m.BoxAddress != "" {
		row.Add(tag(m.BoxAddress))
	}

	row.Add(layout.NewSpacer())
	row.Add(status)

	return row, &memberView{dot: dot, status: status}
}
