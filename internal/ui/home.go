package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/desynkro/lan-party/internal/buildinfo"
	"github.com/desynkro/lan-party/internal/group"
)

// showHome lists saved groups, or introduces the app when there are none.
func (a *App) showHome() {
	title := canvas.NewText("LAN PARTY", hex("#e9edf5"))
	title.TextSize = 30
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("One port, not the network.", hex("#757c8c"))
	subtitle.TextSize = 13
	subtitle.Alignment = fyne.TextAlignCenter

	heading := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(subtitle),
	)

	actions := container.NewVBox(layout.NewSpacer())
	for _, g := range a.groups {
		g := g
		actions.Add(widget.NewButton("Open "+g.Name, func() { a.showGroup(g) }))
	}

	start := widget.NewButton("Start a group", a.newGroup)
	start.Importance = widget.HighImportance
	actions.Add(start)

	join := widget.NewButton("Join a group", nil)
	join.Disable()
	actions.Add(join)

	hint := canvas.NewText("Joining someone else's group comes next.", hex("#5c6373"))
	hint.TextSize = 11
	hint.Alignment = fyne.TextAlignCenter
	actions.Add(container.NewCenter(hint))
	actions.Add(layout.NewSpacer())

	a.win.SetContent(container.NewPadded(container.NewBorder(
		container.NewPadded(heading),
		a.footer(),
		nil,
		nil,
		container.NewPadded(actions),
	)))
	a.win.SetTitle("LAN Party")
}

// newGroup asks for the two names a group needs.
func (a *App) newGroup() {
	nameEntry := widget.NewEntry()
	nameEntry.SetText("Friday Night")
	hostEntry := widget.NewEntry()
	hostEntry.SetText(defaultHostName())

	form := dialog.NewForm("Start a group", "Create", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Group name", nameEntry),
			widget.NewFormItem("Your name", hostEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			host := strings.TrimSpace(hostEntry.Text)
			if name == "" || host == "" {
				dialog.ShowInformation("Missing something", "A group needs a name, and so do you.", a.win)
				return
			}
			g := group.Create(name, host)
			a.groups = append(a.groups, g)
			if err := a.store.Save(a.groups); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.showGroup(g)
		}, a.win)
	form.Resize(fyne.NewSize(380, 250))
	form.Show()
}

func (a *App) footer() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewLabelWithStyle(buildinfo.Summary(), fyne.TextAlignCenter, fyne.TextStyle{}),
			layout.NewSpacer(),
		),
	)
}
