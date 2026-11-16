package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// newHome is the screen you see before there is a group.
func newHome(_ fyne.Window, version string) fyne.CanvasObject {
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

	start := widget.NewButton("Start a group", func() {})
	start.Importance = widget.HighImportance
	join := widget.NewButton("Join a group", func() {})

	actions := container.NewVBox(
		layout.NewSpacer(),
		start,
		join,
		layout.NewSpacer(),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewLabelWithStyle(version, fyne.TextAlignCenter, fyne.TextStyle{}),
			layout.NewSpacer(),
		),
	)

	return container.NewPadded(container.NewBorder(
		container.NewPadded(heading),
		footer,
		nil,
		nil,
		container.NewPadded(actions),
	))
}
