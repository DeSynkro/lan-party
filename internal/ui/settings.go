package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/buildinfo"
)

func (u *UI) showSettings(returnTo func()) {
	u.back = returnTo
	u.current = func() { u.showSettings(returnTo) }
	u.screenName = "settings"

	settings := u.app.Settings()

	closeNote := "Closing the window keeps the app and any live session running in the tray. Untick this to make closing the window quit."
	minimiseNote := "Start hidden in the tray instead of opening the window."
	if !u.hasTray {
		closeNote = "Your desktop did not report a tray, so closing the window quits anyway."
		minimiseNote = "No tray is available, so the window still opens."
	}

	closeToTray := u.checkRow("Close to tray", closeNote,
		settings.CloseToTray,
		func(value bool) error {
			s := u.app.Settings()
			s.CloseToTray = value
			return u.app.SaveSettings(s)
		})

	startOnLogin := u.checkRow("Start on login", "Launch LAN Party automatically when you sign in.",
		settings.StartOnLogin,
		func(value bool) error {
			s := u.app.Settings()
			s.StartOnLogin = value
			return u.app.SaveSettings(s)
		})

	startMinimised := u.checkRow("Start minimised", minimiseNote,
		settings.StartMinimised,
		func(value bool) error {
			s := u.app.Settings()
			s.StartMinimised = value
			return u.app.SaveSettings(s)
		})

	approve := u.checkRow("Approve new devices", "Ask before a new friend's device joins. Off accepts any valid invite without asking. A changed device key still asks.",
		settings.ApproveNewDevices,
		func(value bool) error {
			s := u.app.Settings()
			s.ApproveNewDevices = value
			return u.app.SaveSettings(s)
		})

	publicEntry := widget.NewEntry()
	publicEntry.SetText(settings.PublicAddress)
	publicEntry.SetPlaceHolder("home.example.net")

	portEntry := widget.NewEntry()
	portEntry.SetText(strconv.Itoa(settings.AppPort))
	portEntry.SetPlaceHolder("51820")

	preview := widget.NewLabel("")
	preview.Importance = widget.MediumImportance
	preview.TextStyle = fyne.TextStyle{Monospace: true}
	preview.Wrapping = fyne.TextWrapWord

	networkError := widget.NewLabel("")
	networkError.Importance = widget.DangerImportance
	networkError.Wrapping = fyne.TextWrapWord
	networkError.Hide()

	networkStatus := widget.NewLabel("")
	networkStatus.Importance = widget.MediumImportance
	networkStatus.Wrapping = fyne.TextWrapWord
	networkStatus.Hide()

	updatePreview := func(string) {
		networkError.Hide()
		networkStatus.Hide()

		public := strings.TrimSpace(publicEntry.Text)
		port, err := strconv.Atoi(strings.TrimSpace(portEntry.Text))
		if public == "" {
			preview.SetText("Friends connect to: not set")
			return
		}
		if err != nil {
			preview.SetText("Friends connect to: check the app port")
			return
		}
		endpoint, err := apppkg.Endpoint(public, port)
		if err != nil {
			preview.SetText("Friends connect to: check the address")
			return
		}
		preview.SetText("Friends connect to: " + endpoint)
	}
	publicEntry.OnChanged = updatePreview
	portEntry.OnChanged = updatePreview
	updatePreview("")

	saveNetwork := newButton("Save network settings", buttonPrimary, func() {
		networkError.Hide()
		networkStatus.Hide()
		port, err := strconv.Atoi(strings.TrimSpace(portEntry.Text))
		if err != nil {
			networkError.SetText("The app port must be a number.")
			networkError.Show()
			return
		}
		s := u.app.Settings()
		s.PublicAddress = strings.TrimSpace(publicEntry.Text)
		s.AppPort = port
		if err := u.app.SaveSettings(s); err != nil {
			networkError.SetText(err.Error())
			networkError.Show()
			return
		}
		networkStatus.SetText("Saved.")
		networkStatus.Show()
	})

	networkForm := widget.NewForm(
		widget.NewFormItem("Public address", publicEntry),
		widget.NewFormItem("App port (UDP)", portEntry),
	)
	networkHelp := mutedLabel("Use a hostname or IP. Leave the port off unless your router forwards a different outside port, for example home.example.net:40000. The app port is the UDP port on this PC.")
	networkCard := card(container.NewVBox(
		networkForm,
		networkHelp,
		gap(rowGap),
		preview,
		gap(rowGap),
		networkError,
		networkStatus,
		gap(rowGap),
		container.NewHBox(
			saveNetwork,
			smallButton("Setup guide", buttonQuiet, func() {
				u.showGuide(func() { u.showSettings(returnTo) })
			}),
		),
	))

	about := container.NewVBox(
		metaText("LAN Party · "+buildinfo.Version),
		mutedLabel("Data: "+u.app.DataDir()),
		mutedLabel("Keys: owner-only files in keys/"),
	)

	content := container.NewVBox(
		sectionLabel("Startup"),
		gap(rowGap),
		closeToTray,
		gap(rowGap),
		startOnLogin,
		gap(rowGap),
		startMinimised,
		gap(sectionGap),
		sectionLabel("Network"),
		gap(rowGap),
		networkCard,
		gap(sectionGap),
		sectionLabel("Security"),
		gap(rowGap),
		approve,
		gap(sectionGap),
		sectionLabel("About"),
		gap(rowGap),
		card(about),
	)

	u.screen(u.appBar("Settings", false), content, nil)
}

var settingCheckbox float32 = 24

func (u *UI) checkRow(title, description string, checked bool, onChanged func(bool) error) fyne.CanvasObject {
	var check *checkBox
	check = newCheckBox(checked, func(value bool) {
		if err := onChanged(value); err != nil {
			check.SetChecked(!value)
			u.showError(err)
		}
	})

	header := container.New(layout.NewCustomPaddedHBoxLayout(0),
		vCentered(check),
		gapW(8),
		vCentered(settingTitle(title)),
	)

	text := container.NewVBox(header)
	if description != "" {
		text.Add(gap(settingDescriptionGap))
		text.Add(container.NewBorder(nil, nil, gapW(settingCheckbox+8), nil, metaParagraph(description)))
	}
	return card(text)
}
