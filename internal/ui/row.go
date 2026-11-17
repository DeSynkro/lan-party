package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// rowButton is a full-width navigation row with an optional status light.
type rowButton struct {
	widget.BaseWidget

	title    string
	trailing string
	onTap    func()
	dot      bool
	live     bool
	chevron  bool
	hover    bool
}

func newRowButton(title, trailing string, onTap func()) *rowButton {
	r := &rowButton{title: title, trailing: trailing, onTap: onTap, chevron: true}
	r.ExtendBaseWidget(r)
	return r
}

func (r *rowButton) setLive(live bool) {
	r.dot = true
	r.live = live
	r.Refresh()
}

func (r *rowButton) setChevron(show bool) {
	r.chevron = show
	r.Refresh()
}

func (r *rowButton) Tapped(*fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}

func (r *rowButton) MouseIn(*desktop.MouseEvent) { r.hover = true; r.Refresh() }
func (r *rowButton) MouseOut()                   { r.hover = false; r.Refresh() }

func (r *rowButton) Cursor() desktop.Cursor {
	if r.onTap == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (r *rowButton) CreateRenderer() fyne.WidgetRenderer {
	return &rowRenderer{
		row:   r,
		bg:    surface(colourSurface, colourHairline, 10),
		dot:   canvas.NewText("", colourMuted),
		title: canvas.NewText(r.title, colourText),
		meta:  canvas.NewText(r.trailing, colourMuted),
		chev:  canvas.NewText("›", colourMuted),
	}
}

type rowRenderer struct {
	row   *rowButton
	bg    *canvas.Rectangle
	dot   *canvas.Text
	title *canvas.Text
	meta  *canvas.Text
	chev  *canvas.Text
}

func (r *rowRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 40) }

func (r *rowRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	const inset float32 = 14
	dotWidth := float32(0)
	if r.row.dot {
		dotWidth = 14
		r.dot.Resize(fyne.NewSize(10, 10))
		r.dot.Move(fyne.NewPos(inset, (size.Height-10)/2))
	}

	titleX := inset + dotWidth
	titleSize := r.title.MinSize()
	metaSize := r.meta.MinSize()

	chevWidth := float32(0)
	if r.row.chevron {
		chevWidth = r.chev.MinSize().Width + 10
		r.chev.Resize(r.chev.MinSize())
		r.chev.Move(fyne.NewPos(size.Width-inset-r.chev.MinSize().Width,
			(size.Height-r.chev.MinSize().Height)/2))
	}

	metaX := size.Width - inset - chevWidth - metaSize.Width
	if metaX < titleX {
		metaX = titleX
	}
	titleWidth := metaX - titleX - 8
	if titleWidth < 0 {
		titleWidth = 0
	}
	r.title.Resize(fyne.NewSize(titleWidth, titleSize.Height))
	r.title.Move(fyne.NewPos(titleX, (size.Height-titleSize.Height)/2))

	r.meta.Resize(metaSize)
	r.meta.Move(fyne.NewPos(size.Width-inset-chevWidth-metaSize.Width,
		(size.Height-metaSize.Height)/2))
}

func (r *rowRenderer) Refresh() {
	switch {
	case r.row.hover && r.row.onTap != nil:
		r.bg.FillColor = colourRaised
	default:
		r.bg.FillColor = colourSurface
	}
	r.bg.StrokeColor = colourHairline
	r.bg.StrokeWidth = 1

	r.dot.Text = ""
	if r.row.dot {
		r.dot.Text = "●"
		if r.row.live {
			r.dot.Color = colourLive
		} else {
			r.dot.Color = colourMuted
		}
	}
	r.dot.TextSize = 8

	r.title.Text = truncate(r.row.title, 32)
	r.title.Color = colourText
	r.title.TextSize = sizeBody

	r.meta.Text = truncate(r.row.trailing, 24)
	r.meta.Color = colourMuted
	r.meta.TextSize = sizeMeta

	if r.row.chevron {
		r.chev.Text = "›"
		r.chev.Color = colourMuted
	} else {
		r.chev.Text = ""
	}
	r.chev.TextSize = 16

	r.bg.Refresh()
	r.dot.Refresh()
	r.title.Refresh()
	r.meta.Refresh()
	r.chev.Refresh()
	r.Layout(r.row.Size())
}

func (r *rowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.dot, r.title, r.meta, r.chev}
}

func (r *rowRenderer) Destroy() {}
