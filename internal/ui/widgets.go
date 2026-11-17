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

// Type scale. The compact window carries five sizes and no more.
const (
	sizeTitle       = 22
	sizeAppBar      = 15
	sizeName        = 14
	sizeBody        = 13
	sizeMeta        = 12
	sizeSection     = 12
	sizeAddress     = 14
	sizeAddressHero = 16
)

func nameText(text string) *canvas.Text {
	t := canvas.NewText(text, colourText)
	t.TextSize = sizeName
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func bodyText(text string) *canvas.Text {
	t := canvas.NewText(text, colourSecondary)
	t.TextSize = sizeBody
	return t
}

func metaText(text string) *canvas.Text {
	t := canvas.NewText(text, colourMuted)
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
		surface(colourSurface, colourHairline, 12),
		pad(10, 10, content),
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

func (t *tapArea) Cursor() desktop.Cursor { return desktop.PointerCursor }

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
	small    bool
	large    bool
	disabled bool
	onTap    func()

	hover   bool
	focused bool
}

func newButton(label string, kind buttonKind, onTap func()) *button {
	b := &button{label: label, kind: kind, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func smallButton(label string, kind buttonKind, onTap func()) *button {
	b := newButton(label, kind, onTap)
	b.small = true
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
func (b *button) FocusGained()                { b.focused = true; b.Refresh() }
func (b *button) FocusLost()                  { b.focused = false; b.Refresh() }
func (b *button) TypedRune(rune)              {}

func (b *button) TypedKey(event *fyne.KeyEvent) {
	if event == nil {
		return
	}
	switch event.Name {
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		b.Tapped(nil)
	}
}

func (b *button) Cursor() desktop.Cursor {
	if b.disabled {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *button) setLabel(label string) {
	b.label = label
	b.Refresh()
}

func (b *button) textSize() float32 {
	if b.small {
		return sizeMeta
	}
	return sizeBody
}

func (b *button) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText(b.label, b.textColour())
	text.TextSize = b.textSize()
	text.Alignment = fyne.TextAlignCenter

	bg := surface(b.fillColour(), b.borderColour(), 8)

	return &buttonRenderer{button: b, bg: bg, text: text}
}

type buttonRenderer struct {
	button *button
	bg     *canvas.Rectangle
	text   *canvas.Text
}

func (r *buttonRenderer) MinSize() fyne.Size {
	padding := float32(28)
	height := float32(36)
	switch {
	case r.button.small:
		padding = 20
		height = 28
	case r.button.large:
		padding = 32
		height = 40
	}
	width := fyne.MeasureText(r.button.label, r.button.textSize(), r.text.TextStyle).Width + padding
	return fyne.NewSize(width, height)
}

func (r *buttonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	// canvas.Text centres itself inside its resized box. Moving it again
	// would double the offset and push the label to the bottom of the button.
	r.text.Resize(size)
	r.text.Move(fyne.NewPos(0, 0))
}

func (r *buttonRenderer) Refresh() {
	r.bg.FillColor = r.button.fillColour()
	r.bg.StrokeColor = r.button.borderColour()
	r.bg.StrokeWidth = 1
	if r.button.focused && !r.button.disabled {
		r.bg.StrokeColor = colourAccent
		r.bg.StrokeWidth = 2
	}
	r.text.Text = r.button.label
	r.text.TextSize = r.button.textSize()
	r.text.Color = r.button.textColour()
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *buttonRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.text} }
func (r *buttonRenderer) Destroy()                     {}

func (b *button) fillColour() color.Color {
	if b.disabled {
		return colourSurface
	}
	switch b.kind {
	case buttonPrimary:
		if b.hover {
			return hex("#0f6c7e")
		}
		return colourPrimaryFill
	case buttonSecondary:
		if b.hover {
			return hex("#2b2620")
		}
		return colourRaised
	case buttonDanger:
		if b.hover {
			return hex("#2b1f1e")
		}
		return colourRaised
	default:
		if b.hover {
			return colourRaised
		}
		return color.Transparent
	}
}

func (b *button) borderColour() color.Color {
	if b.disabled {
		return colourHairline
	}
	switch b.kind {
	case buttonPrimary, buttonQuiet:
		return nil
	case buttonDanger:
		return hex("#4a302d")
	default:
		return colourHairline
	}
}

func (b *button) textColour() color.Color {
	if b.disabled {
		return colourMuted
	}
	switch b.kind {
	case buttonPrimary:
		return colourPrimaryInk
	case buttonDanger:
		return colourDanger
	case buttonQuiet:
		return colourSecondary
	default:
		return colourText
	}
}
