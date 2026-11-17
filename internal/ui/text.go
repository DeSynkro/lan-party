package ui

import (
	"fmt"
	"image/color"
	"net"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func titleText(text string) *canvas.Text {
	t := canvas.NewText(text, colourText)
	t.TextSize = sizeTitle
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func settingTitle(text string) *canvas.Text {
	t := canvas.NewText(text, colourText)
	t.TextSize = settingTitleSize
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func appBarTitle(text string) *canvas.Text {
	t := canvas.NewText(truncate(text, 30), colourText)
	t.TextSize = sizeAppBar
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func sectionLabel(text string) *canvas.Text {
	t := canvas.NewText(text, colourMuted)
	t.TextSize = sizeSection
	return t
}

func addressText(text string) *canvas.Text {
	t := canvas.NewText(text, colourAccent)
	t.TextSize = sizeAddress
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func addressHero(text string) *canvas.Text {
	t := canvas.NewText(text, colourAccent)
	t.TextSize = sizeAddressHero
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func addressMuted(text string) *canvas.Text {
	t := canvas.NewText(text, colourSecondary)
	t.TextSize = sizeMeta
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func warningLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Importance = widget.WarningImportance
	label.Wrapping = fyne.TextWrapWord
	return label
}

// mutedLabel is a wrapping paragraph in the secondary colour.
func mutedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Importance = widget.LowImportance
	label.Wrapping = fyne.TextWrapWord
	return label
}

// metaParagraph is a wrapping description at the meta token size and colour.
func metaParagraph(text string) *widget.RichText {
	paragraph := widget.NewRichText(&widget.TextSegment{
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNamePlaceHolder,
			SizeName:  theme.SizeNameCaptionText,
		},
		Text: text,
	})
	paragraph.Wrapping = fyne.TextWrapWord
	return paragraph
}

// bodyLabel is a wrapping paragraph in the normal text colour.
func bodyLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func statusDot(live bool) fyne.CanvasObject {
	colour := color.Color(colourMuted)
	if live {
		colour = colourLive
	}
	return dotColour(colour)
}

func dotColour(colour color.Color) fyne.CanvasObject {
	dot := canvas.NewText("●", colour)
	dot.TextSize = 8
	return container.NewCenter(dot)
}

func tag(text string) fyne.CanvasObject {
	label := canvas.NewText(text, colourSecondary)
	label.TextSize = sizeMeta
	return container.NewStack(
		surface(color.Transparent, colourHairline, tagRadius),
		container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), label),
	)
}

func pill(text string, colour color.Color) fyne.CanvasObject {
	dot := canvas.NewText("●", colour)
	dot.TextSize = 8
	label := canvas.NewText(text, colour)
	label.TextSize = sizeMeta
	label.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewHBox(container.NewCenter(dot), gapW(6), container.NewCenter(label))
}

func hairline() fyne.CanvasObject {
	rect := canvas.NewRectangle(colourHairline)
	rect.SetMinSize(fyne.NewSize(0, 1))
	return rect
}

func divider() fyne.CanvasObject {
	return container.NewVBox(gap(dividerGap), hairline(), gap(dividerGap))
}

func truncate(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max < 2 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func plural(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", count, word)
}

func when(at time.Time) string {
	if at.IsZero() {
		return "never"
	}
	return at.Local().Format("2 Jan 15:04")
}

func expiresIn(at time.Time) string {
	left := time.Until(at)
	switch {
	case left <= 0:
		return "expired"
	case left < time.Minute:
		return "expires in under a minute"
	case left < time.Hour:
		return fmt.Sprintf("expires in %dm", int(left.Minutes()))
	default:
		return fmt.Sprintf("expires in %dh", int(left.Hours()))
	}
}

func localIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "this PC"
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if v4 := ipnet.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	return "this PC"
}

func (u *UI) copyText(text string, feedback *button) {
	u.fyneApp.Clipboard().SetContent(text)
	if feedback == nil {
		return
	}
	feedback.setLabel("Copied")
	time.AfterFunc(1500*time.Millisecond, func() {
		fyne.Do(func() { feedback.setLabel("Copy") })
	})
}
