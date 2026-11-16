package ui

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Colour names used by the interface. Anything not listed here falls back to
// Fyne's dark theme, so unstyled widgets still look like they belong.
const (
	ColourBackground fyne.ThemeColorName = "lanparty.background"
	ColourCard       fyne.ThemeColorName = "lanparty.card"
	ColourBorder     fyne.ThemeColorName = "lanparty.border"
	ColourMuted      fyne.ThemeColorName = "lanparty.muted"
	ColourAccent     fyne.ThemeColorName = "lanparty.accent"
	ColourDanger     fyne.ThemeColorName = "lanparty.danger"
)

var palette = map[fyne.ThemeColorName]color.Color{
	ColourBackground: hex("#14151a"),
	ColourCard:       hex("#191b21"),
	ColourBorder:     hex("#2b2d36"),
	ColourMuted:      hex("#757c8c"),
	ColourAccent:     hex("#22d3ee"),
	ColourDanger:     hex("#ef4444"),

	theme.ColorNameBackground:      hex("#14151a"),
	theme.ColorNameForeground:      hex("#e9edf5"),
	theme.ColorNameButton:          hex("#23252e"),
	theme.ColorNameDisabledButton:  hex("#1b1d24"),
	theme.ColorNameInputBackground: hex("#0e0f13"),
	theme.ColorNamePrimary:         hex("#0e7490"),
	theme.ColorNameSeparator:       hex("#2b2d36"),
	theme.ColorNameShadow:          color.NRGBA{R: 0, G: 0, B: 0, A: 120},
}

type appTheme struct{}

// NewTheme is the dark theme the whole app uses.
func NewTheme() fyne.Theme {
	return appTheme{}
}

func (appTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if c, ok := palette[name]; ok {
		return c
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (appTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (appTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func hex(s string) color.NRGBA {
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}
