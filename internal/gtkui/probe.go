// Package gtkui contains the GTK4 interface. The probe is a standalone screen
// used to validate layout, fonts and theme before the Fyne UI is replaced.
package gtkui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
)

type probeUI struct {
	app   *gtk.Application
	win   *gtk.ApplicationWindow
	root  string
	state *apppkg.App

	stack    *gtk.Stack
	logo     *gtk.Image
	back     *gtk.Button
	title    *gtk.Label
	help     *gtk.Button
	settings *gtk.Button
}

// RunProbe opens the GTK probe window and blocks until it closes.
func RunProbe(argv []string) int {
	root, err := repoRoot()
	if err != nil {
		log.Printf("gtk probe: %v", err)
		return 1
	}
	setupFonts(root)

	state, err := apppkg.New(config.Resolve())
	if err != nil {
		log.Printf("gtk probe: %v", err)
		return 1
	}
	defer state.Close()

	app := gtk.NewApplication("com.desynkro.lanparty.gtkprobe", gio.ApplicationFlagsNone)
	var ui *probeUI
	app.ConnectActivate(func() {
		if ui != nil {
			ui.win.Present()
			return
		}
		loadCSS(root)
		ui = newProbeUI(app, root, state)
		ui.build()
		if os.Getenv("LANPARTY_GTK_SCREEN") == "settings" {
			ui.showSettings()
		} else {
			ui.showHome()
		}
		ui.win.Present()
	})
	return app.Run(argv)
}

func newProbeUI(app *gtk.Application, root string, state *apppkg.App) *probeUI {
	return &probeUI{app: app, root: root, state: state}
}

func (u *probeUI) build() {
	u.win = gtk.NewApplicationWindow(u.app)
	u.win.SetTitle("LAN Party")
	u.win.SetDefaultSize(400, 640)
	u.win.SetResizable(false)

	theme := gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	theme.AddSearchPath(filepath.Join(u.root, "internal", "gtkui", "assets", "icons"))
	u.win.SetIconName("lan-party")

	u.stack = gtk.NewStack()
	u.stack.AddNamed(u.buildHomePage(), "home")
	u.stack.AddNamed(u.buildSettingsPage(), "settings")

	u.win.SetTitlebar(u.buildAppBar())
	u.win.SetChild(u.stack)
}

func (u *probeUI) buildAppBar() *gtk.Box {
	bar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	bar.AddCSSClass("appbar")

	u.logo = gtk.NewImageFromFile(filepath.Join(u.root, "brand", "png", "mark-22.png"))
	bar.Append(u.logo)

	u.back = gtk.NewButtonWithLabel("Back")
	u.back.AddCSSClass("quiet")
	u.back.SetVisible(false)
	u.back.ConnectClicked(func() {
		u.showHome()
	})
	bar.Append(u.back)

	u.title = gtk.NewLabel("LAN Party")
	u.title.AddCSSClass("app-title")
	bar.Append(u.title)

	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	bar.Append(spacer)

	u.help = gtk.NewButtonWithLabel("?")
	u.help.AddCSSClass("quiet")
	u.help.SetTooltipText("How to Use")
	u.help.ConnectClicked(func() {
		log.Println("gtk probe: guide requested")
	})
	bar.Append(u.help)

	u.settings = gtk.NewButtonWithLabel("Settings")
	u.settings.AddCSSClass("quiet")
	u.settings.ConnectClicked(func() {
		u.showSettings()
	})
	bar.Append(u.settings)

	minimizeButton := gtk.NewButtonWithLabel("−")
	minimizeButton.AddCSSClass("quiet")
	minimizeButton.SetTooltipText("Minimise")
	minimizeButton.ConnectClicked(func() {
		u.win.Minimize()
	})
	bar.Append(minimizeButton)

	closeButton := gtk.NewButtonWithLabel("×")
	closeButton.AddCSSClass("quiet")
	closeButton.SetTooltipText("Close")
	closeButton.ConnectClicked(func() {
		u.app.Quit()
	})
	bar.Append(closeButton)

	return bar
}

func (u *probeUI) showHome() {
	u.stack.SetVisibleChildName("home")
	u.title.SetText("LAN Party")
	u.logo.SetVisible(true)
	u.back.SetVisible(false)
	u.help.SetVisible(true)
	u.settings.SetVisible(true)
}

func (u *probeUI) showSettings() {
	u.stack.SetVisibleChildName("settings")
	u.title.SetText("Settings")
	u.logo.SetVisible(false)
	u.back.SetVisible(true)
	u.help.SetVisible(false)
	u.settings.SetVisible(false)
}

func (u *probeUI) buildHomePage() *gtk.ScrolledWindow {
	page := gtk.NewBox(gtk.OrientationVertical, 0)
	page.AddCSSClass("page")
	page.SetMarginTop(20)
	page.SetMarginBottom(20)
	page.SetMarginStart(20)
	page.SetMarginEnd(20)

	title := textLabel("A LAN cable across the internet.", "title")
	title.SetMarginBottom(10)
	page.Append(title)

	body := textLabel("Host a game on your LAN. Friends open it at 127.0.0.1; nothing else is reachable.", "body")
	body.SetMarginBottom(16)
	page.Append(body)

	actions := gtk.NewBox(gtk.OrientationHorizontal, 8)
	actions.SetHomogeneous(true)
	actions.SetMarginBottom(8)

	host := gtk.NewButtonWithLabel("Host a group")
	host.AddCSSClass("primary")
	host.SetHExpand(true)
	actions.Append(host)

	join := gtk.NewButtonWithLabel("Join with an invite")
	join.AddCSSClass("secondary")
	join.SetHExpand(true)
	actions.Append(join)
	page.Append(actions)

	host.ConnectClicked(func() {
		log.Println("gtk probe: Host a group clicked")
	})
	join.ConnectClicked(func() {
		log.Println("gtk probe: Join with an invite clicked")
	})

	guide := gtk.NewButtonWithLabel("First time? Read the setup guide")
	guide.AddCSSClass("quiet")
	guide.SetHAlign(gtk.AlignCenter)
	guide.SetMarginBottom(18)
	guide.ConnectClicked(func() {
		log.Println("gtk probe: setup guide clicked")
	})
	page.Append(guide)

	section := textLabel("Your groups", "section")
	section.SetMarginBottom(8)
	page.Append(section)
	page.Append(groupRow("Friday Night", "host · 1 member"))

	joined := textLabel("Groups you joined", "section")
	joined.SetMarginTop(20)
	joined.SetMarginBottom(8)
	page.Append(joined)
	page.Append(groupRow("Friday Night", "joined · hosted by Desynkro"))

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(page)
	scroll.SetVExpand(true)
	return scroll
}

func groupRow(title, meta string) *gtk.Button {
	row := gtk.NewButton()
	row.AddCSSClass("row")
	row.SetHExpand(true)

	content := gtk.NewBox(gtk.OrientationHorizontal, 8)
	content.AddCSSClass("row-content")

	dot := gtk.NewLabel("●")
	dot.AddCSSClass("dot")
	content.Append(dot)

	name := gtk.NewLabel(title)
	name.AddCSSClass("row-title")
	name.SetHExpand(true)
	name.SetXAlign(0)
	content.Append(name)

	info := gtk.NewLabel(meta)
	info.AddCSSClass("row-meta")
	content.Append(info)

	chevron := gtk.NewLabel("›")
	chevron.AddCSSClass("chevron")
	content.Append(chevron)

	row.SetChild(content)
	return row
}

func textLabel(text, class string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.AddCSSClass(class)
	label.SetWrap(true)
	label.SetXAlign(0)
	label.SetHAlign(gtk.AlignStart)
	return label
}

func loadCSS(root string) {
	path := filepath.Join(root, "internal", "gtkui", "style.css")
	provider := gtk.NewCSSProvider()
	provider.ConnectParsingError(func(_ *gtk.CSSSection, err error) {
		log.Printf("gtk css: %v", err)
	})
	provider.LoadFromPath(path)
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		provider,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)
}

func setupFonts(root string) {
	dir, err := os.MkdirTemp("", "lanparty-fonts-")
	if err != nil {
		log.Printf("gtk fonts: %v", err)
		return
	}
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		log.Printf("gtk fonts: %v", err)
		return
	}

	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	config := fmt.Sprintf(`<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "urn:fontconfig:fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">/etc/fonts/fonts.conf</include>
  <dir>%s</dir>
  <dir>%s</dir>
  <cachedir>%s</cachedir>
</fontconfig>
`, escape.Replace(filepath.Join(root, "brand", "fonts")),
		escape.Replace(filepath.Join(root, "internal", "gtkui", "assets", "fonts")),
		escape.Replace(cache))

	path := filepath.Join(dir, "fonts.conf")
	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		log.Printf("gtk fonts: %v", err)
		return
	}
	if err := os.Setenv("FONTCONFIG_FILE", path); err != nil {
		log.Printf("gtk fonts: %v", err)
	}
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "brand", "fonts", "SpaceGrotesk.ttf")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find brand fonts from %s", dir)
		}
		dir = parent
	}
}
