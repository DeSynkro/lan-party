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
)

var probeWindow *gtk.ApplicationWindow

// RunProbe opens the GTK probe window and blocks until it closes.
func RunProbe(argv []string) int {
	root, err := repoRoot()
	if err != nil {
		log.Printf("gtk probe: %v", err)
		return 1
	}
	setupFonts(root)

	app := gtk.NewApplication("com.desynkro.lanparty.gtkprobe", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() {
		if probeWindow != nil {
			probeWindow.Present()
			return
		}
		loadCSS(root)
		probeWindow = buildProbeWindow(app, root)
		probeWindow.Present()
	})
	return app.Run(argv)
}

func buildProbeWindow(app *gtk.Application, root string) *gtk.ApplicationWindow {
	win := gtk.NewApplicationWindow(app)
	win.SetTitle("LAN Party · GTK probe")
	win.SetDefaultSize(400, 640)
	win.SetResizable(false)

	theme := gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	theme.AddSearchPath(filepath.Join(root, "internal", "gtkui", "assets", "icons"))
	win.SetIconName("lan-party")

	win.SetTitlebar(buildAppBar(root, app, win))
	win.SetChild(buildProbePage())
	return win
}

func buildAppBar(root string, app *gtk.Application, win *gtk.ApplicationWindow) *gtk.Box {
	bar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	bar.AddCSSClass("appbar")

	mark := gtk.NewImageFromFile(filepath.Join(root, "brand", "png", "mark-22.png"))
	bar.Append(mark)

	name := gtk.NewLabel("LAN Party")
	name.AddCSSClass("app-title")
	bar.Append(name)

	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	bar.Append(spacer)

	help := gtk.NewButtonWithLabel("?")
	help.AddCSSClass("quiet")
	help.SetTooltipText("How to Use")
	bar.Append(help)

	settings := gtk.NewButtonWithLabel("Settings")
	settings.AddCSSClass("quiet")
	bar.Append(settings)

	minimizeButton := gtk.NewButtonWithLabel("−")
	minimizeButton.AddCSSClass("quiet")
	minimizeButton.SetTooltipText("Minimise")
	minimizeButton.ConnectClicked(func() {
		win.Minimize()
	})
	bar.Append(minimizeButton)

	closeButton := gtk.NewButtonWithLabel("×")
	closeButton.AddCSSClass("quiet")
	closeButton.SetTooltipText("Close")
	closeButton.ConnectClicked(func() {
		app.Quit()
	})
	bar.Append(closeButton)

	return bar
}

func buildProbePage() *gtk.ScrolledWindow {
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

	page.Append(buildProbeCard())

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

func buildProbeCard() *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginTop(24)

	heading := textLabel("Probe controls", "card-title")
	card.Append(heading)

	check := gtk.NewCheckButton()
	check.AddCSSClass("setting-check")
	check.SetActive(true)

	checkTitle := textLabel("Close to tray", "setting-title")
	checkTitle.SetVAlign(gtk.AlignCenter)

	checkRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	checkRow.SetMarginTop(10)
	checkRow.Append(check)
	checkRow.Append(checkTitle)
	card.Append(checkRow)

	description := textLabel("Closing the window keeps the app and any live session running in the tray. Untick this to make closing the window quit.", "desc")
	description.SetWrap(true)
	description.SetMarginStart(26)
	description.SetMarginTop(2)
	card.Append(description)

	entry := gtk.NewEntry()
	entry.SetPlaceholderText("Invite code")
	entry.AddCSSClass("field")
	entry.SetMarginTop(12)
	card.Append(entry)

	save := gtk.NewButtonWithLabel("Save network settings")
	save.AddCSSClass("primary")
	save.SetHAlign(gtk.AlignStart)
	save.SetMarginTop(12)
	card.Append(save)

	check.ConnectToggled(func() {
		description.SetText(fmt.Sprintf("Close to tray is %t", check.Active()))
	})
	save.ConnectClicked(func() {
		entry.SetText("Invite code received")
	})

	return card
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
