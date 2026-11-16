package ui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/preset"
)

// showGroup is the host's view of a group that exists but is not connected.
func (a *App) showGroup(g group.Group) {
	back := widget.NewButton("Back", a.showHome)
	back.Importance = widget.LowImportance

	name := canvas.NewText(g.Name, hex("#e9edf5"))
	name.TextSize = 24
	name.TextStyle = fyne.TextStyle{Bold: true}

	host := canvas.NewText("Host: "+g.HostName, hex("#757c8c"))
	host.TextSize = 12

	header := container.NewVBox(
		container.NewHBox(back, layout.NewSpacer()),
		name,
		host,
	)

	status := container.NewHBox(
		canvas.NewText("●", hex("#6b7280")),
		widget.NewLabel("No session running"),
	)

	members := container.NewVBox(section("Members"))
	for _, m := range g.Members {
		members.Add(memberRow(m))
	}

	services := container.NewVBox(section("Services"))
	if len(g.Services) == 0 {
		services.Add(muted("Nothing published yet."))
	}
	for _, s := range g.Services {
		services.Add(a.serviceRow(g, s))
	}
	add := widget.NewButton("Add service", func() { a.addService(g) })
	services.Add(add)

	body := container.NewVBox(
		status,
		widget.NewSeparator(),
		members,
		widget.NewSeparator(),
		services,
	)

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(header),
		a.footer(),
		nil,
		nil,
		container.NewScroll(body),
	)))
	a.win.SetTitle("LAN Party · " + g.Name)
}

func (a *App) serviceRow(g group.Group, s group.Service) fyne.CanvasObject {
	title := canvas.NewText(s.Name, hex("#e9edf5"))
	title.TextSize = 13
	title.TextStyle = fyne.TextStyle{Bold: true}

	detail := muted(fmt.Sprintf("%s · %s → %s", s.Proto, group.FormatPorts(s.Ports), s.Target))

	rows := container.NewVBox(title, detail)
	if len(s.Ports) > 0 {
		address := canvas.NewText(fmt.Sprintf("friends see 127.0.0.1:%d", s.Ports[0]), hex("#22d3ee"))
		address.TextSize = 11
		address.TextStyle = fyne.TextStyle{Monospace: true}
		rows.Add(address)
	}

	remove := widget.NewButton("Remove", func() { a.removeService(g, s.ID) })
	remove.Importance = widget.LowImportance

	return container.NewHBox(rows, layout.NewSpacer(), remove)
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

func section(title string) *canvas.Text {
	t := canvas.NewText(strings.ToUpper(title), hex("#666d7d"))
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func muted(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#757c8c"))
	t.TextSize = 12
	return t
}

func memberRow(m group.Member) fyne.CanvasObject {
	dot := canvas.NewText("●", hex("#22c55e"))
	dot.TextSize = 10

	row := container.NewHBox(dot, widget.NewLabel(m.Name))
	if m.IsHost {
		tag := canvas.NewText("host", hex("#9aa2b2"))
		tag.TextSize = 10
		row.Add(tag)
	}
	return row
}
