package gtkui

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/applog"
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

	useRelay := gtk.NewCheckButtonWithLabel("Use Relay")
	useRelay.SetActive(settings.UseRelay)
	useRelay.SetMarginBottom(10)

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

	directPreview := textLabel("", "mono")
	directPreview.SetMarginTop(2)

	updateDirectPreview := func() {
		public := strings.TrimSpace(publicEntry.Text())
		port, err := strconv.Atoi(strings.TrimSpace(portEntry.Text()))
		switch {
		case public == "":
			directPreview.SetText("Friends connect to: not set")
		case err != nil:
			directPreview.SetText("Friends connect to: check the app port")
		default:
			endpoint, err := apppkg.Endpoint(public, port)
			if err != nil {
				directPreview.SetText("Friends connect to: check the address")
				return
			}
			directPreview.SetText("Friends connect to: " + endpoint)
		}
	}
	publicEntry.ConnectChanged(func() {
		feedback.hide()
		updateDirectPreview()
	})
	portEntry.ConnectChanged(func() {
		feedback.hide()
		updateDirectPreview()
	})

	directBox := gtk.NewBox(gtk.OrientationVertical, 0)
	directBox.Append(formRow("Public address", publicEntry))
	directBox.Append(gapBox(8))
	directBox.Append(formRow("App port (UDP)", portEntry))
	directBox.Append(gapBox(10))
	directHelp := textLabel("Use a hostname or IP. Leave the port off unless your router forwards a different outside port, for example home.example.net:40000. The app port is the UDP port on this PC.", "help")
	directHelp.SetSizeRequest(descriptionWidth, -1)
	directBox.Append(directHelp)
	directBox.Append(gapBox(10))
	directBox.Append(directPreview)

	relayHostEntry := gtk.NewEntry()
	relayHostEntry.SetText(settings.RelayHost)
	relayHostEntry.SetPlaceholderText("relay.example.net")
	relayHostEntry.AddCSSClass("field")
	relayHostEntry.SetHExpand(true)

	relayPortEntry := gtk.NewEntry()
	relayPortEntry.SetText(strconv.Itoa(settings.RelayPort))
	relayPortEntry.SetPlaceholderText("7443")
	relayPortEntry.AddCSSClass("field")
	relayPortEntry.SetHExpand(true)

	relayIDEntry := gtk.NewEntry()
	relayIDEntry.SetText(settings.RelayID)
	relayIDEntry.SetPlaceholderText("64-character Relay ID")
	relayIDEntry.AddCSSClass("field")
	relayIDEntry.SetHExpand(true)

	relayPasswordEntry := gtk.NewEntry()
	relayPasswordEntry.SetVisibility(false)
	relayPasswordEntry.SetInputPurpose(gtk.InputPurposePassword)
	relayPasswordEntry.AddCSSClass("field")
	relayPasswordEntry.SetHExpand(true)

	passwordStatus := textLabel("", "desc")
	updatePasswordStatus := func() {
		if u.state.RelayPasswordSet() {
			passwordStatus.SetText("A relay password is saved. Leave this blank to keep it.")
			relayPasswordEntry.SetPlaceholderText("Saved password")
		} else {
			passwordStatus.SetText("No relay password is saved yet.")
			relayPasswordEntry.SetPlaceholderText("Relay password")
		}
	}

	relayPreview := textLabel("", "mono")
	relayPreview.SetMarginTop(2)
	updateRelayPreview := func() {
		host := strings.TrimSpace(relayHostEntry.Text())
		port, err := strconv.Atoi(strings.TrimSpace(relayPortEntry.Text()))
		switch {
		case host == "":
			relayPreview.SetText("Friends connect through: set the relay host")
		case err != nil:
			relayPreview.SetText("Friends connect through: check the relay port")
		default:
			relayPreview.SetText("Friends connect through " + host + " on port " + strconv.Itoa(port) + ". No port forward is needed.")
		}
	}
	for _, entry := range []*gtk.Entry{relayHostEntry, relayPortEntry, relayIDEntry} {
		entry.ConnectChanged(func() {
			feedback.hide()
			updateRelayPreview()
		})
	}
	relayPasswordEntry.ConnectChanged(func() { feedback.hide() })

	relayBox := gtk.NewBox(gtk.OrientationVertical, 0)
	relayBox.Append(formRow("Relay host", relayHostEntry))
	relayBox.Append(gapBox(8))
	relayBox.Append(formRow("Relay port", relayPortEntry))
	relayBox.Append(gapBox(8))
	relayBox.Append(formRow("Relay ID", relayIDEntry))
	relayBox.Append(gapBox(8))
	relayBox.Append(formRow("Relay password", relayPasswordEntry))
	relayBox.Append(gapBox(4))
	relayBox.Append(passwordStatus)
	relayBox.Append(gapBox(10))
	relayHelp := textLabel("The relay is a server you run. Friends do not need the relay password; each invite carries a short-lived token. The Relay ID is the relay's public fingerprint.", "help")
	relayHelp.SetSizeRequest(descriptionWidth, -1)
	relayBox.Append(relayHelp)
	relayBox.Append(gapBox(10))
	relayBox.Append(relayPreview)

	syncConnectionFields := func() {
		relay := useRelay.Active()
		directBox.SetVisible(!relay)
		relayBox.SetVisible(relay)
	}
	useRelay.ConnectToggled(func() {
		feedback.hide()
		syncConnectionFields()
	})

	networkCard := gtk.NewBox(gtk.OrientationVertical, 0)
	networkCard.AddCSSClass("card")
	networkCard.SetMarginBottom(10)
	networkCard.Append(useRelay)
	networkCard.Append(directBox)
	networkCard.Append(relayBox)

	importProfile := gtk.NewButtonWithLabel("Import relay profile…")
	importProfile.AddCSSClass("secondary")
	importProfile.SetHAlign(gtk.AlignStart)
	importProfile.ConnectClicked(func() {
		chooser := gtk.NewFileChooserNative("Import relay profile", &u.win.Window, gtk.FileChooserActionOpen, "Import", "Cancel")
		filter := gtk.NewFileFilter()
		filter.SetName("Relay profile")
		filter.AddPattern("relay-profile.json")
		filter.AddPattern("*.json")
		chooser.AddFilter(filter)
		chooser.ConnectResponse(func(response int) {
			defer chooser.Destroy()
			if response != int(gtk.ResponseAccept) {
				return
			}
			file := chooser.File()
			if file == nil {
				return
			}
			data, err := os.ReadFile(file.Path())
			if err != nil {
				feedback.showError(errors.New("could not read that relay profile"))
				return
			}
			imported, err := u.state.ImportRelayProfile(data)
			if err != nil {
				feedback.showError(err)
				return
			}
			settings = imported
			useRelay.SetActive(true)
			relayHostEntry.SetText(imported.RelayHost)
			relayPortEntry.SetText(strconv.Itoa(imported.RelayPort))
			relayIDEntry.SetText(imported.RelayID)
			relayPasswordEntry.SetText("")
			updatePasswordStatus()
			updateRelayPreview()
			syncConnectionFields()
			feedback.showStatus("Imported the relay profile.")
		})
		chooser.Show()
	})

	testRelay := gtk.NewButtonWithLabel("Test relay")
	testRelay.AddCSSClass("secondary")
	testRelay.SetHAlign(gtk.AlignStart)
	testRelay.ConnectClicked(func() {
		host := strings.TrimSpace(relayHostEntry.Text())
		port, err := strconv.Atoi(strings.TrimSpace(relayPortEntry.Text()))
		if err != nil {
			feedback.showError(errors.New("the relay port must be a number"))
			return
		}
		relayID := strings.TrimSpace(relayIDEntry.Text())
		password := relayPasswordEntry.Text()
		testRelay.SetSensitive(false)
		feedback.showStatus("Testing the relay…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			err := u.state.TestRelayConnection(ctx, host, port, relayID, password)
			glib.IdleAdd(func() {
				testRelay.SetSensitive(true)
				if err != nil {
					feedback.showError(err)
					return
				}
				feedback.showStatus("Relay reachable and the password is correct.")
			})
		}()
	})

	save := gtk.NewButtonWithLabel("Save network settings")
	save.AddCSSClass("primary")
	save.SetHAlign(gtk.AlignStart)
	save.ConnectClicked(func() {
		s := u.state.Settings()
		directPort, err := strconv.Atoi(strings.TrimSpace(portEntry.Text()))
		if err != nil {
			feedback.showError(errors.New("the app port must be a number"))
			return
		}
		relayPort := u.state.Settings().RelayPort
		if text := strings.TrimSpace(relayPortEntry.Text()); text != "" {
			relayPort, err = strconv.Atoi(text)
			if err != nil {
				feedback.showError(errors.New("the relay port must be a number"))
				return
			}
		}
		s.PublicAddress = strings.TrimSpace(publicEntry.Text())
		s.AppPort = directPort
		s.UseRelay = useRelay.Active()
		s.RelayHost = strings.TrimSpace(relayHostEntry.Text())
		s.RelayPort = relayPort
		s.RelayID = strings.TrimSpace(relayIDEntry.Text())
		if err := u.state.SaveSettingsWithRelayPassword(s, relayPasswordEntry.Text()); err != nil {
			feedback.showError(err)
			return
		}
		settings = u.state.Settings()
		relayPasswordEntry.SetText("")
		updatePasswordStatus()
		updateDirectPreview()
		updateRelayPreview()
		feedback.showStatus("Saved.")
	})

	buttonRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	buttonRow.Append(testRelay)
	buttonRow.Append(importProfile)
	buttonRow.Append(save)
	buttonRow.SetHAlign(gtk.AlignStart)
	buttonRow.SetMarginTop(12)
	networkCard.Append(buttonRow)
	page.Append(networkCard)

	updateDirectPreview()
	updateRelayPreview()
	updatePasswordStatus()
	syncConnectionFields()

	connection := textLabel("Connection", "section")
	connection.SetMarginTop(18)
	connection.SetMarginBottom(8)
	page.Append(connection)
	page.Append(u.settingCard(
		"Reconnect automatically",
		"Retry for up to 30 minutes when a friend connection drops.",
		settings.AutoReconnect,
		func(v bool) error {
			return saveSetting(func(s *config.Settings) { s.AutoReconnect = v })
		},
		feedback,
	))

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
	logLabel := textLabel("Log: "+applog.Path(u.state.DataDir()), "desc")
	logLabel.SetSelectable(true)
	about.Append(logLabel)
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
