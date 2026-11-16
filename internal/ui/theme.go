package ui

import (
	"embed"
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed assets/fonts/*.ttf
var fontFiles embed.FS

var (
	fontRegular  = resource("Inter-Regular.ttf")
	fontMedium   = resource("Inter-Medium.ttf")
	fontSemibold = resource("Inter-SemiBold.ttf")
	fontMono     = resource("JetBrainsMono-Regular.ttf")
)

func resource(name string) fyne.Resource {
	data, err := fontFiles.ReadFile("assets/fonts/" + name)
	if err != nil {
		panic("missing font " + name)
	}
	return fyne.NewStaticResource(name, data)
}

// Colour names used by the interface. Anything not listed falls back to Fyne's
// dark theme so stray widgets still sit inside the palette.
const (
	ColourBackground fyne.ThemeColorName = "lanparty.background"
	ColourSurface    fyne.ThemeColorName = "lanparty.surface"
	ColourRaised     fyne.ThemeColorName = "lanparty.raised"
	ColourHairline   fyne.ThemeColorName = "lanparty.hairline"
	ColourText       fyne.ThemeColorName = "lanparty.text"
	ColourSecondary  fyne.ThemeColorName = "lanparty.secondary"
	ColourMuted      fyne.ThemeColorName = "lanparty.muted"
	ColourAccent     fyne.ThemeColorName = "lanparty.accent"
	ColourAccentFill fyne.ThemeColorName = "lanparty.accentFill"
	ColourSuccess    fyne.ThemeColorName = "lanparty.success"
	ColourWarning    fyne.ThemeColorName = "lanparty.warning"
	ColourDanger     fyne.ThemeColorName = "lanparty.danger"
)

var palette = map[fyne.ThemeColorName]color.Color{
	ColourBackground: hex("#0f1013"),
	ColourSurface:    hex("#17181c"),
	ColourRaised:     hex("#1d2025"),
	ColourHairline:   hex("#2a2d34"),
	ColourText:       hex("#e8eaee"),
	ColourSecondary:  hex("#a3a7b0"),
	ColourMuted:      hex("#6f737d"),
	ColourAccent:     hex("#5ad1e6"),
	ColourAccentFill: hex("#12778a"),
	ColourSuccess:    hex("#4ade80"),
	ColourWarning:    hex("#e0b341"),
	ColourDanger:     hex("#e0575c"),

	theme.ColorNameBackground:          hex("#0f1013"),
	theme.ColorNameForeground:          hex("#e8eaee"),
	theme.ColorNameButton:              hex("#1d2025"),
	theme.ColorNameDisabledButton:      hex("#17181c"),
	theme.ColorNameDisabled:            hex("#4d515a"),
	theme.ColorNameInputBackground:     hex("#17181c"),
	theme.ColorNamePrimary:             hex("#12778a"),
	theme.ColorNameForegroundOnPrimary: hex("#eafcff"),
	theme.ColorNameSeparator:           hex("#2a2d34"),
	theme.ColorNameShadow:              color.NRGBA{R: 0, G: 0, B: 0, A: 160},
	theme.ColorNameHover:               hex("#22252b"),
	theme.ColorNameScrollBar:           hex("#343841"),
	theme.ColorNameScrollBarBackground: color.Transparent,
	theme.ColorNameSelection:           hex("#12778a"),
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
	switch {
	case style.Monospace:
		return fontMono
	case style.Bold:
		return fontSemibold
	case style.Italic || style.Symbol:
		return fontMedium
	default:
		return fontRegular
	}
}

func (appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (appTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	}
	return theme.DefaultTheme().Size(name)
}

func hex(s string) color.NRGBA {
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{A: 255}
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}
