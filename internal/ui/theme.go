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
	fontRegular  = resource("IBMPlexSans-Regular.ttf")
	fontMedium   = resource("IBMPlexSans-Medium.ttf")
	fontSemibold = resource("IBMPlexSans-SemiBold.ttf")
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

var (
	colourBackground  = hex("#0f1013")
	colourSurface     = hex("#17181c")
	colourRaised      = hex("#1d2025")
	colourHairline    = hex("#2a2d34")
	colourText        = hex("#e8eaee")
	colourSecondary   = hex("#a3a7b0")
	colourMuted       = hex("#7d8390")
	colourAccent      = hex("#5ad1e6")
	colourLive        = hex("#4ade80")
	colourWarning     = hex("#e0b341")
	colourDanger      = hex("#e0575c")
	colourPrimaryFill = hex("#12778a")
	colourPrimaryInk  = hex("#eafcff")
)

var palette = map[fyne.ThemeColorName]color.Color{
	ColourBackground: colourBackground,
	ColourSurface:    colourSurface,
	ColourRaised:     colourRaised,
	ColourHairline:   colourHairline,
	ColourText:       colourText,
	ColourSecondary:  colourSecondary,
	ColourMuted:      colourMuted,
	ColourAccent:     colourAccent,
	ColourAccentFill: colourPrimaryFill,
	ColourSuccess:    colourLive,
	ColourWarning:    colourWarning,
	ColourDanger:     colourDanger,

	theme.ColorNameBackground:                colourBackground,
	theme.ColorNameForeground:                colourText,
	theme.ColorNameButton:                    colourRaised,
	theme.ColorNameDisabledButton:            colourSurface,
	theme.ColorNameDisabled:                  hex("#4d515a"),
	theme.ColorNameError:                     colourDanger,
	theme.ColorNameFocus:                     colourAccent,
	theme.ColorNameForegroundOnError:         colourBackground,
	theme.ColorNameForegroundOnPrimary:       colourPrimaryInk,
	theme.ColorNameForegroundOnSuccess:       colourBackground,
	theme.ColorNameForegroundOnWarning:       colourBackground,
	theme.ColorNameHeaderBackground:          colourSurface,
	theme.ColorNameHover:                     colourRaised,
	theme.ColorNameHyperlink:                 colourAccent,
	theme.ColorNameInnerWindowBorder:         colourHairline,
	theme.ColorNameInnerWindowBorderInactive: colourSurface,
	theme.ColorNameInputBackground:           colourSurface,
	theme.ColorNameInputBorder:               colourHairline,
	theme.ColorNameMenuBackground:            colourSurface,
	theme.ColorNameOverlayBackground:         color.NRGBA{R: 0, G: 0, B: 0, A: 170},
	theme.ColorNamePlaceHolder:               colourMuted,
	theme.ColorNamePressed:                   hex("#242830"),
	theme.ColorNamePrimary:                   colourPrimaryFill,
	theme.ColorNameScrollBar:                 hex("#343841"),
	theme.ColorNameScrollBarBackground:       color.Transparent,
	theme.ColorNameSelection:                 colourPrimaryFill,
	theme.ColorNameSeparator:                 colourHairline,
	theme.ColorNameShadow:                    color.NRGBA{R: 0, G: 0, B: 0, A: 160},
	theme.ColorNameSuccess:                   colourLive,
	theme.ColorNameWarning:                   colourWarning,
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
		return 12
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameButtonRadius:
		return 8
	case theme.SizeNameCardRadius:
		return 12
	case theme.SizeNameDialogRadius:
		return 14
	case theme.SizeNamePopupRadius:
		return 10
	case theme.SizeNameMenuRadius:
		return 10
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
