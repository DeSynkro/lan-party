package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (u *UI) showGuide(returnTo func()) {
	u.back = returnTo
	u.current = func() { u.showGuide(returnTo) }
	u.screenName = "guide"
	u.screen(u.appBar("How to Use", false), u.guideContent(returnTo), nil)
}

func (u *UI) guideContent(returnTo func()) fyne.CanvasObject {
	settings := u.app.Settings()
	rule := fmt.Sprintf("Forward UDP %d to %s", settings.AppPort, localIPv4())

	var copyButton *button
	copyButton = smallButton("Copy", buttonSecondary, func() {
		u.copyText(rule, copyButton)
	})
	ruleLabel := widget.NewLabel(rule)
	ruleLabel.TextStyle = fyne.TextStyle{Monospace: true}
	ruleLabel.Wrapping = fyne.TextWrapWord

	addressLine := mutedLabel("No public address set yet. Open Settings and fill it in.")
	settingsButton := smallButton("Settings", buttonQuiet, func() {
		u.showSettings(func() { u.showGuide(returnTo) })
	})
	if strings.TrimSpace(settings.PublicAddress) != "" {
		addressLine = mutedLabel("Your public address: " + settings.PublicAddress)
	}

	hosting := card(container.NewVBox(
		sectionLabel("Hosting"),
		gap(8),
		bodyLabel("1. Open your group and add the game server you want to share. Use the IP and port it listens on."),
		gap(8),
		bodyLabel("2. Forward one UDP port on your router. That is the app port, not the game port."),
		gap(8),
		container.NewBorder(nil, nil, nil, container.NewCenter(copyButton), ruleLabel),
		gap(8),
		mutedLabel("Only this one port faces the internet. The game ports themselves stay closed."),
		gap(8),
		addressLine,
		container.NewHBox(settingsButton),
		gap(8),
		bodyLabel("3. Start the session, create an invite, and send the code to your friend."),
		gap(8),
		bodyLabel("4. When your friend joins, an approval card shows their device name and address. Allow it once."),
	))

	joining := card(container.NewVBox(
		sectionLabel("Joining"),
		gap(8),
		bodyLabel("Paste the invite code and wait for the host to approve your device."),
		gap(8),
		bodyLabel("Open the game, then use the address shown under Ready to play. For Minecraft Java that is usually Direct Connect."),
	))

	problems := card(container.NewVBox(
		sectionLabel("Problems"),
		gap(8),
		nameText("A friend can't reach you"),
		mutedLabel("Check the public address in Settings, the UDP rule on your router, and the firewall on this PC. A VPN on the host can also hide the address. If your router's WAN address differs from your public address, your ISP may use carrier-grade NAT; this version has no relay, so direct hosting will not work."),
		gap(10),
		nameText("Your public address changed"),
		mutedLabel("Update it in Settings. Old invites still point at the old address, so create new ones after the change."),
		gap(10),
		nameText("A port is already in use"),
		mutedLabel("The friend app tries the next local port and shows the address it actually got. The host's published port does not change."),
		gap(10),
		nameText("An invite expired or was already used"),
		mutedLabel("Create a new invite. Codes work once and expire after 24 hours."),
	))

	security := card(container.NewVBox(
		sectionLabel("Security"),
		gap(8),
		bodyLabel("Invites are single use, expire after 24 hours, and need the host to be online when they are redeemed."),
		gap(8),
		bodyLabel("The host approves each device. A different device key asks again."),
		gap(8),
		bodyLabel("Only the services you publish exist. Friends cannot reach your router, your other devices, or each other."),
		gap(8),
		bodyLabel("Keep invite codes private. Anyone with a code can try to redeem it."),
	))

	return container.NewVBox(
		hosting,
		gap(16),
		joining,
		gap(16),
		problems,
		gap(16),
		security,
		gap(8),
		mutedLabel("App port "+strconv.Itoa(settings.AppPort)+" · UDP"),
	)
}
