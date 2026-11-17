package ui

import (
	"image/color"
	"math"
	"testing"
)

func TestPaletteContrast(t *testing.T) {
	bg := palette[ColourBackground]
	surface := palette[ColourSurface]
	pairs := []struct {
		name string
		fg   color.Color
		bg   color.Color
	}{
		{"text on background", palette[ColourText], bg},
		{"secondary on background", palette[ColourSecondary], bg},
		{"muted on background", palette[ColourMuted], bg},
		{"text on surface", palette[ColourText], surface},
		{"secondary on surface", palette[ColourSecondary], surface},
		{"muted on surface", palette[ColourMuted], surface},
		{"accent on background", palette[ColourAccent], bg},
		{"live on background", palette[ColourSuccess], bg},
		{"warning on background", palette[ColourWarning], bg},
		{"danger on background", palette[ColourDanger], bg},
		{"primary button", hex("#eafcff"), hex("#12778a")},
	}
	for _, p := range pairs {
		if got := contrastRatio(p.fg, p.bg); got < 4.5 {
			t.Errorf("%s contrast = %.2f, want >= 4.5", p.name, got)
		}
	}
}

func contrastRatio(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v) / 65535
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}
