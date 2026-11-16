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

// card puts content on a raised surface.
func card(content fyne.CanvasObject) fyne.CanvasObject {
	return surface(hex("#191b21"), hex("#2b2d36"), 10, content)
}

// surface is a rounded rectangle with content on top.
func surface(fill, border color.Color, radius float32, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.CornerRadius = radius
	if border != nil {
		bg.StrokeColor = border
		bg.StrokeWidth = 1
	}
	return container.NewStack(bg, content)
}

// gap is fixed vertical space, unlike a spacer which grows.
func gap(height float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, height))
	return r
}

// gapW is fixed horizontal space.
func gapW(width float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(width, 0))
	return r
}

// sectionHeader is a small caps title with an optional action on the right.
func sectionHeader(title string, action fyne.CanvasObject) fyne.CanvasObject {
	label := caps(title)

	if action == nil {
		return label
	}

	return container.NewBorder(
		nil, nil, nil, action,
		container.NewVBox(layout.NewSpacer(), label, layout.NewSpacer()),
	)
}

func caps(text string) *canvas.Text {
	t := canvas.NewText(strings.ToUpper(text), hex("#666d7d"))
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// titleText is the big heading used at the top of a screen.
func titleText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#e9edf5"))
	t.TextSize = 20
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

// tag is a small outlined label.
func tag(text string) fyne.CanvasObject {
	label := canvas.NewText(text, hex("#9aa2b2"))
	label.TextSize = 10

	return surface(color.Transparent, hex("#33363f"), 999,
		container.New(layout.NewCustomPaddedLayout(1, 1, 7, 7), label))
}

// tile is the rounded square that stands in for a service icon.
func tile(letter string) fyne.CanvasObject {
	bg := canvas.NewRectangle(hex("#23252e"))
	bg.CornerRadius = 8
	bg.SetMinSize(fyne.NewSize(34, 34))

	text := canvas.NewText(letter, hex("#a5f0ff"))
	text.TextSize = 15
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.Alignment = fyne.TextAlignCenter

	return container.NewStack(bg, container.NewCenter(text))
}

// muted is secondary text.
func muted(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#757c8c"))
	t.TextSize = 11
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

// rightLayout stacks children against the right edge, which Fyne's stock
// layouts will not do for a block of text with different widths.
type rightLayout struct{}

func (rightLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	width, height := float32(0), float32(0)
	for _, o := range objs {
		size := o.MinSize()
		if size.Width > width {
			width = size.Width
		}
		height += size.Height
	}
	return fyne.NewSize(width, height)
}

func (rightLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objs {
		child := o.MinSize()
		o.Resize(child)
		o.Move(fyne.NewPos(size.Width-child.Width, y))
		y += child.Height
	}
}
