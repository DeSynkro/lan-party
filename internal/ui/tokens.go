package ui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
)

//go:embed tokens.json
var tokenFile []byte

// PaletteTokens is stored as hex strings so the token file stays readable.
type PaletteTokens struct {
	Background  string `json:"background"`
	Surface     string `json:"surface"`
	Raised      string `json:"raised"`
	Hairline    string `json:"hairline"`
	Text        string `json:"text"`
	Secondary   string `json:"secondary"`
	Muted       string `json:"muted"`
	Accent      string `json:"accent"`
	Live        string `json:"live"`
	Warning     string `json:"warning"`
	Danger      string `json:"danger"`
	PrimaryFill string `json:"primaryFill"`
	PrimaryInk  string `json:"primaryInk"`
}

// Tokens is the interface measurement set. The app reads this file at build
// time and the tuner page exports the same keys.
type Tokens struct {
	WindowWidth           int           `json:"windowWidth"`
	WindowHeight          int           `json:"windowHeight"`
	AppBarHeight          float32       `json:"appBarHeight"`
	PagePaddingX          float32       `json:"pagePaddingX"`
	PagePaddingY          float32       `json:"pagePaddingY"`
	CardPadding           float32       `json:"cardPadding"`
	CardRadius            float32       `json:"cardRadius"`
	RowRadius             float32       `json:"rowRadius"`
	ButtonRadius          float32       `json:"buttonRadius"`
	TagRadius             float32       `json:"tagRadius"`
	SectionGap            float32       `json:"sectionGap"`
	RowGap                float32       `json:"rowGap"`
	SmallGap              float32       `json:"smallGap"`
	TinyGap               float32       `json:"tinyGap"`
	DividerGap            float32       `json:"dividerGap"`
	SettingDescriptionGap float32       `json:"settingDescriptionGap"`
	TitleSize             float32       `json:"titleSize"`
	NameSize              float32       `json:"nameSize"`
	BodySize              float32       `json:"bodySize"`
	MetaSize              float32       `json:"metaSize"`
	SectionSize           float32       `json:"sectionSize"`
	AppBarSize            float32       `json:"appBarSize"`
	AddressSize           float32       `json:"addressSize"`
	AddressHeroSize       float32       `json:"addressHeroSize"`
	SettingTitleSize      float32       `json:"settingTitleSize"`
	SettingCheckbox       float32       `json:"settingCheckbox"`
	RowHeight             float32       `json:"rowHeight"`
	Palette               PaletteTokens `json:"palette"`
}

var (
	cardPadding           float32 = 10
	cardRadius            float32 = 12
	rowRadius             float32 = 10
	buttonRadius          float32 = 8
	tagRadius             float32 = 6
	sectionGap            float32 = 12
	rowGap                float32 = 6
	smallGap              float32 = 4
	tinyGap               float32 = 2
	dividerGap            float32 = 8
	settingDescriptionGap float32 = 1
	settingTitleSize      float32 = 15
)

var activeTokens = defaultTokens()

func init() {
	var loaded Tokens
	if err := json.Unmarshal(tokenFile, &loaded); err != nil {
		activeTokens = defaultTokens()
	} else if err := validateTokens(loaded); err != nil {
		activeTokens = defaultTokens()
	} else {
		activeTokens = loaded
	}
	applyTokens()
}

func defaultTokens() Tokens {
	return Tokens{
		WindowWidth:           400,
		WindowHeight:          640,
		AppBarHeight:          48,
		PagePaddingX:          16,
		PagePaddingY:          12,
		CardPadding:           10,
		CardRadius:            12,
		RowRadius:             10,
		ButtonRadius:          8,
		TagRadius:             6,
		SectionGap:            12,
		RowGap:                6,
		SmallGap:              4,
		TinyGap:               2,
		DividerGap:            8,
		SettingDescriptionGap: 1,
		TitleSize:             22,
		NameSize:              14,
		BodySize:              13,
		MetaSize:              12,
		SectionSize:           12,
		AppBarSize:            15,
		AddressSize:           14,
		AddressHeroSize:       16,
		SettingTitleSize:      15,
		SettingCheckbox:       24,
		RowHeight:             40,
		Palette: PaletteTokens{
			Background:  "#0f1013",
			Surface:     "#17181c",
			Raised:      "#1d2025",
			Hairline:    "#2a2d34",
			Text:        "#e8eaee",
			Secondary:   "#a3a7b0",
			Muted:       "#7d8390",
			Accent:      "#5ad1e6",
			Live:        "#4ade80",
			Warning:     "#e0b341",
			Danger:      "#e0575c",
			PrimaryFill: "#12778a",
			PrimaryInk:  "#eafcff",
		},
	}
}

func validateTokens(t Tokens) error {
	if t.WindowWidth < 320 || t.WindowWidth > 600 {
		return fmt.Errorf("window width out of range")
	}
	if t.WindowHeight < 480 || t.WindowHeight > 900 {
		return fmt.Errorf("window height out of range")
	}
	numbers := map[string]float32{
		"appBarHeight":          t.AppBarHeight,
		"pagePaddingX":          t.PagePaddingX,
		"pagePaddingY":          t.PagePaddingY,
		"cardPadding":           t.CardPadding,
		"cardRadius":            t.CardRadius,
		"rowRadius":             t.RowRadius,
		"buttonRadius":          t.ButtonRadius,
		"tagRadius":             t.TagRadius,
		"sectionGap":            t.SectionGap,
		"rowGap":                t.RowGap,
		"smallGap":              t.SmallGap,
		"tinyGap":               t.TinyGap,
		"dividerGap":            t.DividerGap,
		"settingDescriptionGap": t.SettingDescriptionGap,
		"titleSize":             t.TitleSize,
		"nameSize":              t.NameSize,
		"bodySize":              t.BodySize,
		"metaSize":              t.MetaSize,
		"sectionSize":           t.SectionSize,
		"appBarSize":            t.AppBarSize,
		"addressSize":           t.AddressSize,
		"addressHeroSize":       t.AddressHeroSize,
		"settingTitleSize":      t.SettingTitleSize,
		"settingCheckbox":       t.SettingCheckbox,
		"rowHeight":             t.RowHeight,
	}
	for name, value := range numbers {
		if value < 0 || value > 80 {
			return fmt.Errorf("%s out of range", name)
		}
	}
	if t.TitleSize < 12 || t.BodySize < 8 || t.MetaSize < 8 || t.SectionSize < 8 {
		return fmt.Errorf("type size out of range")
	}
	if t.SettingCheckbox < 12 || t.RowHeight < 24 {
		return fmt.Errorf("control size out of range")
	}

	colors := []string{
		t.Palette.Background, t.Palette.Surface, t.Palette.Raised,
		t.Palette.Hairline, t.Palette.Text, t.Palette.Secondary,
		t.Palette.Muted, t.Palette.Accent, t.Palette.Live,
		t.Palette.Warning, t.Palette.Danger, t.Palette.PrimaryFill,
		t.Palette.PrimaryInk,
	}
	for _, value := range colors {
		if len(value) != 7 || value[0] != '#' {
			return fmt.Errorf("palette colour %q is not #rrggbb", value)
		}
		if _, err := strconv.ParseUint(value[1:], 16, 32); err != nil {
			return fmt.Errorf("palette colour %q is not #rrggbb", value)
		}
	}
	return nil
}

func applyTokens() {
	t := activeTokens
	windowWidth = float32(t.WindowWidth)
	windowHeight = float32(t.WindowHeight)
	appBarHeight = t.AppBarHeight
	pagePaddingX = t.PagePaddingX
	pagePaddingY = t.PagePaddingY
	cardPadding = t.CardPadding
	cardRadius = t.CardRadius
	rowRadius = t.RowRadius
	buttonRadius = t.ButtonRadius
	tagRadius = t.TagRadius
	sectionGap = t.SectionGap
	rowGap = t.RowGap
	smallGap = t.SmallGap
	tinyGap = t.TinyGap
	dividerGap = t.DividerGap
	settingDescriptionGap = t.SettingDescriptionGap
	sizeTitle = t.TitleSize
	sizeName = t.NameSize
	sizeBody = t.BodySize
	sizeMeta = t.MetaSize
	sizeSection = t.SectionSize
	sizeAppBar = t.AppBarSize
	sizeAddress = t.AddressSize
	sizeAddressHero = t.AddressHeroSize
	settingTitleSize = t.SettingTitleSize
	settingCheckbox = t.SettingCheckbox
	rowHeight = t.RowHeight
	applyPalette(t.Palette)
}
