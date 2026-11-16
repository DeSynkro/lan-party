package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// card puts content on a raised surface with a border.
func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(hex("#20242c"))
	bg.CornerRadius = 12

	return container.NewStack(bg, container.NewPadded(content))
}

// gap is fixed empty space, unlike a spacer which grows.
func gap(height float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, height))
	return r
}

// sectionHeader is a small caps title with an optional action on the right.
func sectionHeader(title string, action fyne.CanvasObject) fyne.CanvasObject {
	label := canvas.NewText(strings.ToUpper(title), hex("#666d7d"))
	label.TextSize = 10
	label.TextStyle = fyne.TextStyle{Bold: true}

	if action == nil {
		return label
	}

	return container.NewBorder(
		nil, nil, nil, action,
		container.NewVBox(layout.NewSpacer(), label, layout.NewSpacer()),
	)
}

// titleText is the big heading used at the top of a screen.
func titleText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#e9edf5"))
	t.TextSize = 22
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// pill is a short status label with a coloured dot.
func pill(text, colour string) fyne.CanvasObject {
	dot := canvas.NewText("●", hex(colour))
	dot.TextSize = 10

	label := canvas.NewText(text, hex(colour))
	label.TextSize = 11
	label.TextStyle = fyne.TextStyle{Bold: true}

	return container.NewHBox(dot, label)
}

// muted is secondary text.
func muted(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#757c8c"))
	t.TextSize = 12
	return t
}

// addressText is an address a person will read or copy.
func addressText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#22d3ee"))
	t.TextSize = 13
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

// quietButton is for actions that should not compete for attention.
func quietButton(label string, tapped func()) *widget.Button {
	b := widget.NewButton(label, tapped)
	b.Importance = widget.LowImportance
	return b
}
