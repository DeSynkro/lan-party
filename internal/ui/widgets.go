package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Type scale. Inter at these sizes holds up in a 420px window.
const (
	sizeTitle   = 18
	sizeName    = 14
	sizeBody    = 13
	sizeMeta    = 11
	sizeAddress = 14
)

func titleText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#e8eaee"))
	t.TextSize = sizeTitle
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func nameText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#e8eaee"))
	t.TextSize = sizeName
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func bodyText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#a3a7b0"))
	t.TextSize = sizeBody
	return t
}

func metaText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#6f737d"))
	t.TextSize = sizeMeta
	return t
}

func addressText(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#5ad1e6"))
	t.TextSize = sizeAddress
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func sectionLabel(text string) *canvas.Text {
	t := canvas.NewText(text, hex("#6f737d"))
	t.TextSize = sizeMeta
	return t
}

// pad is fixed padding, unlike container.NewPadded's 4px.
func pad(v, h float32, content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(v, v, h, h), content)
}

// gap is fixed vertical space, unlike a spacer which grows.
func gap(height float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, height))
	return r
}

func gapW(width float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(width, 0))
	return r
}

// surface is a rounded rectangle. Depth comes from the surface step and a
// hairline, never from a shadow on a dark theme.
func surface(fill, border color.Color, radius float32) *canvas.Rectangle {
	rect := canvas.NewRectangle(fill)
	rect.CornerRadius = radius
	if border != nil {
		rect.StrokeColor = border
		rect.StrokeWidth = 1
	}
	return rect
}

// card puts content on a surface with room to breathe.
func card(content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(
		surface(hex("#17181c"), hex("#2a2d34"), 12),
		pad(16, 16, content),
	)
}

// tile is the rounded square that stands in for a service icon.
func tile(letter string) fyne.CanvasObject {
	bg := canvas.NewRectangle(hex("#22252b"))
	bg.CornerRadius = 10
	bg.SetMinSize(fyne.NewSize(32, 32))

	text := canvas.NewText(letter, hex("#e8eaee"))
	text.TextSize = 14
	text.TextStyle = fyne.TextStyle{Bold: true}
	text.Alignment = fyne.TextAlignCenter

	return container.NewStack(bg, container.NewCenter(text))
}

// tag is a small outlined label.
func tag(text string) fyne.CanvasObject {
	label := canvas.NewText(text, hex("#a3a7b0"))
	label.TextSize = sizeMeta

	return container.NewStack(
		surface(color.Transparent, hex("#2a2d34"), 6),
		container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), label),
	)
}

// pill is a status label with a coloured dot.
func pill(text, colour string) fyne.CanvasObject {
	dot := canvas.NewText("●", hex(colour))
	dot.TextSize = 8

	label := canvas.NewText(text, hex(colour))
	label.TextSize = sizeMeta
	label.TextStyle = fyne.TextStyle{Bold: true}

	return container.NewHBox(
		container.NewCenter(dot),
		gapW(6),
		container.NewCenter(label),
	)
}

// tapArea is an invisible tappable layer, so a card can open a detail dialog
// without a visible button on it.
type tapArea struct {
	widget.BaseWidget
	onTap func()
}

func newTapArea(onTap func()) *tapArea {
	t := &tapArea{onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapArea) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tapArea) CreateRenderer() fyne.WidgetRenderer {
	return &tapRenderer{rect: canvas.NewRectangle(color.Transparent)}
}

type tapRenderer struct {
	rect *canvas.Rectangle
}

func (r *tapRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *tapRenderer) Layout(size fyne.Size)        { r.rect.Resize(size) }
func (r *tapRenderer) Refresh()                     { r.rect.Refresh() }
func (r *tapRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *tapRenderer) Destroy()                     {}

// column keeps content at a readable width and centres it, no matter how wide
// the window gets. A utility should not stretch its buttons across a desktop.
type column struct{ max float32 }

func (c column) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return objs[0].MinSize()
}

func (c column) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	width := size.Width
	if width > c.max {
		width = c.max
	}
	x := (size.Width - width) / 2
	objs[0].Resize(fyne.NewSize(width, size.Height))
	objs[0].Move(fyne.NewPos(x, 0))
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

// button kinds. Primary is used once per screen at most, quiet for anything
// that should not compete.
type buttonKind int

const (
	buttonPrimary buttonKind = iota
	buttonSecondary
	buttonQuiet
	buttonDanger
)

type button struct {
	widget.BaseWidget

	label    string
	kind     buttonKind
	compact  bool
	disabled bool
	onTap    func()

	hover bool
}

func newButton(label string, kind buttonKind, onTap func()) *button {
	b := &button{label: label, kind: kind, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func newCompactButton(label string, kind buttonKind, onTap func()) *button {
	b := newButton(label, kind, onTap)
	b.compact = true
	return b
}

func (b *button) Tapped(*fyne.PointEvent) {
	if b.disabled || b.onTap == nil {
		return
	}
	b.onTap()
}

func (b *button) MouseIn(*desktop.MouseEvent) { b.hover = true; b.Refresh() }
func (b *button) MouseOut()                   { b.hover = false; b.Refresh() }
func (b *button) Cursor() desktop.Cursor      { return desktop.PointerCursor }

func (b *button) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText(b.label, b.textColour())
	text.TextSize = sizeBody
	text.Alignment = fyne.TextAlignCenter
	if b.kind == buttonPrimary {
		text.TextStyle = fyne.TextStyle{Bold: true}
	}

	bg := surface(b.fillColour(), b.borderColour(), 8)

	return &buttonRenderer{button: b, bg: bg, text: text}
}

type buttonRenderer struct {
	button *button
	bg     *canvas.Rectangle
	text   *canvas.Text
}

func (r *buttonRenderer) MinSize() fyne.Size {
	width := fyne.MeasureText(r.button.label, sizeBody, r.text.TextStyle).Width + 28
	height := float32(32)
	if r.button.compact {
		width = 26
		height = 26
	}
	return fyne.NewSize(width, height)
}

func (r *buttonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	r.text.Resize(size)
	textHeight := r.text.MinSize().Height
	r.text.Move(fyne.NewPos(0, (size.Height-textHeight)/2))
}

func (r *buttonRenderer) Refresh() {
	r.bg.FillColor = r.button.fillColour()
	r.bg.StrokeColor = r.button.borderColour()
	r.text.Text = r.button.label
	r.text.Color = r.button.textColour()
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *buttonRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.text} }
func (r *buttonRenderer) Destroy()                     {}

func (b *button) fillColour() color.Color {
	if b.disabled {
		return hex("#17181c")
	}
	switch b.kind {
	case buttonPrimary:
		return hex("#12778a")
	case buttonSecondary:
		if b.hover {
			return hex("#22252b")
		}
		return hex("#1d2025")
	case buttonDanger:
		if b.hover {
			return hex("#241a1c")
		}
		return hex("#1d2025")
	default:
		if b.hover {
			return hex("#1d2025")
		}
		return color.Transparent
	}
}

func (b *button) borderColour() color.Color {
	if b.disabled {
		return hex("#22252b")
	}
	switch b.kind {
	case buttonPrimary, buttonQuiet:
		return nil
	case buttonDanger:
		return hex("#4a2b2e")
	default:
		return hex("#2a2d34")
	}
}

func (b *button) textColour() color.Color {
	if b.disabled {
		return hex("#4d515a")
	}
	switch b.kind {
	case buttonPrimary:
		return hex("#eafcff")
	case buttonDanger:
		return hex("#e0575c")
	case buttonQuiet:
		return hex("#a3a7b0")
	default:
		return hex("#e8eaee")
	}
}

// disabledButton is a button that cannot be pressed yet.
func disabledButton(label string) *button {
	b := newButton(label, buttonSecondary, nil)
	b.disabled = true
	return b
}
