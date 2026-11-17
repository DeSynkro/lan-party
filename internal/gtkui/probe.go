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
	"github.com/desynkro/lan-party/internal/group"
)

type probeUI struct {
	app   *gtk.Application
	win   *gtk.ApplicationWindow
	root  string
	state *apppkg.App

	trayAvailable bool
	trayOn        bool
	held          bool
	quitting      bool
	approvalShown string

	stack       *gtk.Stack
	homeScroll  *gtk.ScrolledWindow
	groupScroll *gtk.ScrolledWindow
	logo        *gtk.Image
	back        *gtk.Button
	title       *gtk.Label
	help        *gtk.Button
	settings    *gtk.Button
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

	appID := os.Getenv("LANPARTY_GTK_APP_ID")
	if appID == "" {
		appID = "com.desynkro.lanparty.gtkprobe"
	}
	app := gtk.NewApplication(appID, gio.ApplicationFlagsNone)
	var ui *probeUI
	app.ConnectActivate(func() {
		if ui != nil {
			ui.win.Present()
			return
		}
		loadCSS(root)
		ui = newProbeUI(app, root, state)
		ui.build()
		screen := os.Getenv("LANPARTY_GTK_SCREEN")
		switch {
		case screen == "settings":
			ui.showSettings()
		case screen == "guide":
			ui.showGuide()
		case screen == "create":
			ui.showHome()
			ui.showCreateGroupDialog()
		case screen == "join":
			ui.showHome()
			ui.showJoinDialog()
		case strings.HasPrefix(screen, "invite:"):
			if g, ok := ui.state.Group(strings.TrimPrefix(screen, "invite:")); ok {
				ui.showGroup(g.ID)
				ui.showInviteDialog(g)
			}
		case strings.HasPrefix(screen, "service:"):
			if g, ok := ui.state.Group(strings.TrimPrefix(screen, "service:")); ok {
				ui.showGroup(g.ID)
				ui.showAddServiceDialog(g)
			}
		case strings.HasPrefix(screen, "delete:"):
			if g, ok := ui.state.Group(strings.TrimPrefix(screen, "delete:")); ok {
				ui.showGroup(g.ID)
				ui.confirmDeleteGroup(g)
			}
		case strings.HasPrefix(screen, "member:"):
			if g, ok := ui.state.Group(strings.TrimPrefix(screen, "member:")); ok && len(g.Members) > 0 {
				ui.showGroup(g.ID)
				ui.showMemberDialog(g, g.Members[0])
			}
		case strings.HasPrefix(screen, "service-detail:"):
			if g, ok := ui.state.Group(strings.TrimPrefix(screen, "service-detail:")); ok && len(g.Services) > 0 {
				ui.showGroup(g.ID)
				ui.showServiceDialog(g, g.Services[0])
			}
		case strings.HasPrefix(screen, "group:"):
			ui.showGroup(strings.TrimPrefix(screen, "group:"))
		default:
			ui.showHome()
		}
		ui.start()
	})
	return app.Run(argv)
}

func newProbeUI(app *gtk.Application, root string, state *apppkg.App) *probeUI {
	return &probeUI{
		app:           app,
		root:          root,
		state:         state,
		trayAvailable: statusNotifierAvailable(),
	}
}

func (u *probeUI) build() {
	u.win = gtk.NewApplicationWindow(u.app)
	u.win.SetTitle("LAN Party")
	u.win.SetDefaultSize(400, 640)
	u.win.SetResizable(false)
	u.win.SetDecorated(false)

	theme := gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	theme.AddSearchPath(filepath.Join(u.root, "internal", "gtkui", "assets", "icons"))
	u.win.SetIconName("lan-party")

	u.homeScroll = u.buildHomePage()
	u.stack = gtk.NewStack()
	u.stack.AddNamed(u.homeScroll, "home")
	u.stack.AddNamed(u.buildSettingsPage(), "settings")
	u.stack.AddNamed(u.buildGuidePage(), "guide")

	u.groupScroll = gtk.NewScrolledWindow()
	u.groupScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	u.groupScroll.SetVExpand(true)
	u.stack.AddNamed(u.groupScroll, "group")

	appBar := u.buildAppBar()

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.Append(appBar)
	root.Append(u.stack)
	u.win.SetChild(root)
}

func (u *probeUI) enableDrag(handle *gtk.WindowHandle) {
	gesture := gtk.NewGestureDrag()
	gesture.SetButton(1)
	gesture.ConnectDragBegin(func(x, y float64) {
		device := gesture.Device()
		if device == nil {
			seat := gdk.DisplayGetDefault().DefaultSeat()
			if seat, ok := seat.(*gdk.Seat); ok {
				device = seat.Pointer()
			}
		}
		if device == nil {
			return
		}
		surface := u.win.Widget.Native().Surface()
		if surface == nil {
			return
		}
		toplevel := &gdk.Toplevel{Surface: *gdk.BaseSurface(surface)}
		toplevel.BeginMove(device, 1, x, y, gesture.CurrentEventTime())
	})
	handle.AddController(gesture)
}

func (u *probeUI) buildAppBar() *gtk.Box {
	bar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	bar.AddCSSClass("appbar")

	u.back = gtk.NewButtonWithLabel("Back")
	u.back.AddCSSClass("quiet")
	u.back.SetVisible(false)
	u.back.ConnectClicked(func() {
		u.showHome()
	})
	bar.Append(u.back)

	dragArea := gtk.NewBox(gtk.OrientationHorizontal, 8)
	dragArea.SetHExpand(true)

	u.logo = gtk.NewImageFromFile(filepath.Join(u.root, "brand", "png", "mark-22.png"))
	dragArea.Append(u.logo)

	u.title = gtk.NewLabel("LAN Party")
	u.title.AddCSSClass("app-title")
	dragArea.Append(u.title)

	spacer := gtk.NewBox(gtk.OrientationHorizontal, 0)
	spacer.SetHExpand(true)
	dragArea.Append(spacer)

	dragHandle := gtk.NewWindowHandle()
	dragHandle.SetHExpand(true)
	dragHandle.SetChild(dragArea)
	u.enableDrag(dragHandle)
	bar.Append(dragHandle)

	u.help = gtk.NewButtonWithLabel("?")
	u.help.AddCSSClass("quiet")
	u.help.SetTooltipText("How to Use")
	u.help.ConnectClicked(func() {
		u.showGuide()
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
		u.requestClose()
	})
	bar.Append(closeButton)

	return bar
}

func (u *probeUI) showHome() {
	if u.homeScroll != nil {
		u.homeScroll.SetChild(u.buildHomeContent())
	}
	u.stack.SetVisibleChildName("home")
	u.title.SetText("LAN Party")
	u.logo.SetVisible(true)
	u.back.SetVisible(false)
	u.help.SetVisible(true)
	u.settings.SetVisible(true)
}

func (u *probeUI) showGroup(id string) {
	g, ok := u.state.Group(id)
	if !ok {
		u.showHome()
		return
	}
	u.groupScroll.SetChild(u.buildGroupContent(g))
	u.stack.SetVisibleChildName("group")
	u.title.SetText(truncateText(g.Name, 30))
	u.logo.SetVisible(false)
	u.back.SetVisible(true)
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

func (u *probeUI) showGuide() {
	u.stack.SetVisibleChildName("guide")
	u.title.SetText("How to Use")
	u.logo.SetVisible(false)
	u.back.SetVisible(true)
	u.help.SetVisible(false)
	u.settings.SetVisible(true)
}

func (u *probeUI) start() {
	u.trayOn = startTray(u)
	go u.watchApprovals()
	if u.trayOn {
		u.app.Hold()
		u.held = true
	}
	u.win.ConnectCloseRequest(func() bool {
		u.requestClose()
		return true
	})
	if u.trayOn && u.state.Settings().StartMinimised {
		return
	}
	u.reveal()
}

func (u *probeUI) reveal() {
	u.win.SetVisible(true)
	u.win.Present()
}

func (u *probeUI) requestClose() {
	if u.trayOn && u.state.Settings().CloseToTray {
		u.win.SetVisible(false)
		return
	}
	u.quit()
}

func (u *probeUI) openSettings() {
	u.reveal()
	u.showSettings()
}

func (u *probeUI) quit() {
	if u.quitting {
		return
	}
	u.quitting = true
	if u.trayOn {
		u.trayOn = false
		systrayQuit()
	}
	if u.held {
		u.held = false
		u.app.Release()
	}
	u.app.Quit()
}

func (u *probeUI) buildHomePage() *gtk.ScrolledWindow {
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(u.buildHomeContent())
	scroll.SetVExpand(true)
	return scroll
}

func (u *probeUI) buildHomeContent() *gtk.Box {
	page := gtk.NewBox(gtk.OrientationVertical, 0)
	page.AddCSSClass("page")
	page.SetMarginTop(20)
	page.SetMarginBottom(20)
	page.SetMarginStart(20)
	page.SetMarginEnd(20)

	lockup := gtk.NewPictureForFilename(filepath.Join(u.root, "brand", "png", "lockup-horizontal-dark-640.png"))
	lockup.SetSizeRequest(340, 81)
	lockup.SetCanShrink(true)
	lockup.SetContentFit(gtk.ContentFitContain)
	lockup.SetHAlign(gtk.AlignCenter)
	lockup.SetMarginBottom(10)
	page.Append(lockup)

	body := textLabel("Host a game on your LAN. Friends open it at 127.0.0.1; nothing else is reachable.", "body")
	body.SetHAlign(gtk.AlignFill)
	body.SetXAlign(0.5)
	body.SetJustify(gtk.JustifyCenter)
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
		u.showCreateGroupDialog()
	})
	join.ConnectClicked(func() {
		u.showJoinDialog()
	})

	guide := gtk.NewButtonWithLabel("First time? Read the setup guide")
	guide.AddCSSClass("quiet")
	guide.SetHAlign(gtk.AlignCenter)
	guide.SetMarginBottom(18)
	guide.ConnectClicked(func() {
		u.showGuide()
	})
	page.Append(guide)

	groups := u.state.Groups()
	hostGroups := make([]group.Group, 0, len(groups))
	joinedGroups := make([]group.Group, 0, len(groups))
	for _, g := range groups {
		if g.IsHost() {
			hostGroups = append(hostGroups, g)
		} else {
			joinedGroups = append(joinedGroups, g)
		}
	}

	if len(hostGroups) > 0 {
		section := textLabel("Your groups", "section")
		section.SetMarginBottom(8)
		page.Append(section)
		for i, g := range hostGroups {
			if i > 0 {
				page.Append(gapBox(8))
			}
			page.Append(u.homeGroupRow(g))
		}
	}
	if len(joinedGroups) > 0 {
		joined := textLabel("Groups you joined", "section")
		joined.SetMarginTop(20)
		joined.SetMarginBottom(8)
		page.Append(joined)
		for i, g := range joinedGroups {
			if i > 0 {
				page.Append(gapBox(8))
			}
			page.Append(u.homeGroupRow(g))
		}
	}
	if len(groups) == 0 {
		section := textLabel("Your groups", "section")
		section.SetMarginTop(18)
		section.SetMarginBottom(8)
		page.Append(section)
		empty := gtk.NewBox(gtk.OrientationVertical, 0)
		empty.AddCSSClass("card")
		empty.Append(cardText("Nothing saved yet. Hosted and joined groups appear here.", "desc"))
		page.Append(empty)
	}

	return page
}

func (u *probeUI) homeGroupRow(g group.Group) *gtk.Button {
	meta := "host · " + plural(len(g.Members), "member")
	if !g.IsHost() {
		meta = "joined · hosted by " + g.HostName
	}
	return navRow(g.Name, meta, u.state.LiveGroupID() == g.ID, func() {
		u.showGroup(g.ID)
	})
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

func truncateText(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max < 2 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}
