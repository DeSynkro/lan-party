package gtkui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/buildinfo"
	"github.com/desynkro/lan-party/internal/config"
)

const descriptionWidth = 280

func (u *probeUI) buildSettingsPage() *gtk.ScrolledWindow {
	settings := u.state.Settings()
	feedback := newFeedback()

	page := gtk.NewBox(gtk.OrientationVertical, 0)
	page.AddCSSClass("page")
	page.SetMarginTop(20)
	page.SetMarginBottom(20)
	page.SetMarginStart(20)
	page.SetMarginEnd(20)
	page.Append(feedback.box)

	saveSetting := func(mutate func(*config.Settings)) error {
		s := u.state.Settings()
		mutate(&s)
		return u.state.SaveSettings(s)
	}

	startup := textLabel("Startup", "section")
	startup.SetMarginBottom(8)
	page.Append(startup)

	closeNote := "Keep the app running in the tray when you close the window. Untick to quit on close."
	minimiseNote := "Start hidden in the tray."
	if !u.trayAvailable {
		closeNote = "No tray was found, so closing the window quits."
		minimiseNote = "No tray was found; the window still opens."
	}

	page.Append(u.settingCard(
		"Close to tray",
		closeNote,
		settings.CloseToTray,
		func(v bool) error {
			return saveSetting(func(s *config.Settings) { s.CloseToTray = v })
		},
		feedback,
	))

	page.Append(u.settingCard(
		"Start on login",
		"Launch LAN Party when you sign in.",
		settings.StartOnLogin,
		func(v bool) error {
			return saveSetting(func(s *config.Settings) { s.StartOnLogin = v })
		},
		feedback,
	))

	page.Append(u.settingCard(
		"Start minimised",
		minimiseNote,
		settings.StartMinimised,
		func(v bool) error {
			return saveSetting(func(s *config.Settings) { s.StartMinimised = v })
		},
		feedback,
	))

	network := textLabel("Network defaults", "section")
	network.SetMarginTop(18)
	network.SetMarginBottom(8)
	page.Append(network)
	networkNote := textLabel("Host groups use these defaults unless they set their own connection. Changing them cancels open invites for groups that use these defaults.", "help")
	networkNote.SetMarginBottom(8)
	page.Append(networkNote)

	publicEntry := gtk.NewEntry()
	publicEntry.SetText(settings.PublicAddress)
	publicEntry.SetPlaceholderText("home.example.net")
	publicEntry.AddCSSClass("field")
	publicEntry.SetHExpand(true)

	portEntry := gtk.NewEntry()
	portEntry.SetText(strconv.Itoa(settings.AppPort))
	portEntry.SetPlaceholderText("51820")
	portEntry.AddCSSClass("field")
	portEntry.SetHExpand(true)

	preview := textLabel("", "mono")
	preview.SetMarginTop(2)

	updatePreview := func() {
		feedback.hide()
		public := strings.TrimSpace(publicEntry.Text())
		port, err := strconv.Atoi(strings.TrimSpace(portEntry.Text()))
		switch {
		case public == "":
			preview.SetText("Friends connect to: not set")
		case err != nil:
			preview.SetText("Friends connect to: check the app port")
		default:
			endpoint, err := apppkg.Endpoint(public, port)
			if err != nil {
				preview.SetText("Friends connect to: check the address")
				return
			}
			preview.SetText("Friends connect to: " + endpoint)
		}
	}
	publicEntry.ConnectChanged(updatePreview)
	portEntry.ConnectChanged(updatePreview)
	updatePreview()

	networkCard := gtk.NewBox(gtk.OrientationVertical, 0)
	networkCard.AddCSSClass("card")
	networkCard.SetMarginBottom(10)
	networkCard.Append(formRow("Public address", publicEntry))
	networkCard.Append(gapBox(8))
	networkCard.Append(formRow("App port (UDP)", portEntry))
	networkCard.Append(gapBox(10))
	networkHelp := textLabel("Use a hostname or IP. Leave the port off unless your router forwards a different outside port, for example home.example.net:40000. The app port is the UDP port on this PC.", "help")
	networkHelp.SetSizeRequest(descriptionWidth, -1)
	networkCard.Append(networkHelp)
	networkCard.Append(gapBox(10))
	networkCard.Append(preview)

	save := gtk.NewButtonWithLabel("Save network settings")
	save.AddCSSClass("primary")
	save.SetHAlign(gtk.AlignStart)
	save.SetMarginTop(12)
	save.ConnectClicked(func() {
		port, err := strconv.Atoi(strings.TrimSpace(portEntry.Text()))
		if err != nil {
			feedback.showError(errors.New("the app port must be a number"))
			return
		}
		s := u.state.Settings()
		s.PublicAddress = strings.TrimSpace(publicEntry.Text())
		s.AppPort = port
		if err := u.state.SaveSettings(s); err != nil {
			feedback.showError(err)
			return
		}
		updatePreview()
		feedback.showStatus("Saved.")
	})
	networkCard.Append(save)
	page.Append(networkCard)

	security := textLabel("Security", "section")
	security.SetMarginTop(18)
	security.SetMarginBottom(8)
	page.Append(security)

	page.Append(u.settingCard(
		"Approve new devices",
		"Ask before a new device joins. A changed device key always asks.",
		settings.ApproveNewDevices,
		func(v bool) error {
			return saveSetting(func(s *config.Settings) { s.ApproveNewDevices = v })
		},
		feedback,
	))

	aboutSection := textLabel("About", "section")
	aboutSection.SetMarginTop(18)
	aboutSection.SetMarginBottom(8)
	page.Append(aboutSection)

	about := gtk.NewBox(gtk.OrientationVertical, 0)
	about.AddCSSClass("card")
	about.Append(textLabel("LAN Party · "+buildinfo.Version, "body"))
	about.Append(gapBox(4))
	about.Append(textLabel("Data: "+u.state.DataDir(), "desc"))
	about.Append(textLabel("Keys: owner-only files in keys/", "desc"))
	page.Append(about)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(page)
	scroll.SetVExpand(true)
	return scroll
}

func (u *probeUI) settingCard(title, description string, active bool, changed func(bool) error, feedback *feedback) *gtk.Box {
	check := gtk.NewCheckButton()
	check.AddCSSClass("setting-check")
	check.SetActive(active)

	checkTitle := textLabel(title, "setting-title")
	checkTitle.SetVAlign(gtk.AlignCenter)

	checkRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	checkRow.Append(check)
	checkRow.Append(checkTitle)

	descriptionLabel := textLabel(description, "desc")
	descriptionLabel.SetHExpand(true)
	descriptionLabel.SetSizeRequest(descriptionWidth, -1)

	descriptionRow := gtk.NewBox(gtk.OrientationHorizontal, 0)
	descriptionRow.Append(gapWidth(26))
	descriptionRow.Append(descriptionLabel)
	descriptionRow.Append(gapWidth(24))
	descriptionRow.SetMarginTop(2)

	card := gtk.NewBox(gtk.OrientationVertical, 0)
	card.AddCSSClass("card")
	card.SetMarginBottom(10)
	card.Append(checkRow)
	card.Append(descriptionRow)

	reverting := false
	check.ConnectToggled(func() {
		if reverting {
			return
		}
		if err := changed(check.Active()); err != nil {
			reverting = true
			check.SetActive(!check.Active())
			reverting = false
			feedback.showError(err)
			return
		}
		feedback.hide()
	})
	return card
}

func formRow(labelText string, entry *gtk.Entry) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 10)
	label := gtk.NewLabel(labelText)
	label.AddCSSClass("form-label")
	label.SetXAlign(0)
	label.SetVAlign(gtk.AlignCenter)
	label.SetSizeRequest(108, -1)
	row.Append(label)
	row.Append(entry)
	return row
}

func gapBox(height int) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetSizeRequest(-1, height)
	return box
}

func gapWidth(width int) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetSizeRequest(width, -1)
	return box
}

type feedback struct {
	box    *gtk.Box
	error  *gtk.Label
	status *gtk.Label
}

func newFeedback() *feedback {
	f := &feedback{
		box:    gtk.NewBox(gtk.OrientationVertical, 0),
		error:  textLabel("", "error"),
		status: textLabel("", "status"),
	}
	f.box.Append(f.error)
	f.box.Append(f.status)
	f.box.SetVisible(false)
	f.box.SetMarginBottom(10)
	return f
}

func (f *feedback) showError(err error) {
	f.error.SetText(err.Error())
	f.error.SetVisible(true)
	f.status.SetVisible(false)
	f.box.SetVisible(true)
}

func (f *feedback) showStatus(text string) {
	f.status.SetText(text)
	f.status.SetVisible(true)
	f.error.SetVisible(false)
	f.box.SetVisible(true)
}

func (f *feedback) hide() {
	f.box.SetVisible(false)
}
