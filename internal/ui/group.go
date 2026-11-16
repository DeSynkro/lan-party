package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/group"
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
		empty := canvas.NewText("Nothing published yet.", hex("#757c8c"))
		empty.TextSize = 12
		services.Add(empty)
	}

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

func section(title string) *canvas.Text {
	t := canvas.NewText(strings.ToUpper(title), hex("#666d7d"))
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Bold: true}
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
