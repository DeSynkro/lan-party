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
		gap(rowGap),
		bodyLabel("1. Open your group and add the game server you want to share. Use the IP and port it listens on."),
		gap(rowGap),
		bodyLabel("2. Forward one UDP port on your router. That is the app port, not the game port."),
		gap(rowGap),
		container.NewBorder(nil, nil, nil, container.NewCenter(copyButton), ruleLabel),
		gap(rowGap),
		mutedLabel("Only this one port faces the internet. The game ports themselves stay closed."),
		gap(rowGap),
		addressLine,
		container.NewHBox(settingsButton),
		gap(rowGap),
		bodyLabel("3. Start the session, create an invite, and send the code to your friend."),
		gap(rowGap),
		bodyLabel("4. When your friend joins, an approval card shows their device name and address. Allow it once."),
	))

	joining := card(container.NewVBox(
		sectionLabel("Joining"),
		gap(rowGap),
		bodyLabel("Paste the invite code and wait for the host to approve your device."),
		gap(rowGap),
		bodyLabel("Open the game, then use the address shown under Ready to play. For Minecraft Java that is usually Direct Connect."),
	))

	problems := card(container.NewVBox(
		sectionLabel("Problems"),
		gap(rowGap),
		nameText("A friend can't reach you"),
		mutedLabel("Check the public address in Settings, the UDP rule on your router, and the firewall on this PC. A VPN on the host can also hide the address. If your router's WAN address differs from your public address, your ISP may use carrier-grade NAT; this version has no relay, so direct hosting will not work."),
		gap(rowGap),
		nameText("Your public address changed"),
		mutedLabel("Update it in Settings. Old invites still point at the old address, so create new ones after the change."),
		gap(rowGap),
		nameText("A port is already in use"),
		mutedLabel("The friend app tries the next local port and shows the address it actually got. The host's published port does not change."),
		gap(rowGap),
		nameText("An invite expired or was already used"),
		mutedLabel("Create a new invite. Codes work once and expire after 24 hours."),
	))

	security := card(container.NewVBox(
		sectionLabel("Security"),
		gap(rowGap),
		bodyLabel("Invites are single use, expire after 24 hours, and need the host to be online when they are redeemed."),
		gap(rowGap),
		bodyLabel("The host approves each device. A different device key asks again."),
		gap(rowGap),
		bodyLabel("Only the services you publish exist. Friends cannot reach your router, your other devices, or each other."),
		gap(rowGap),
		bodyLabel("Keep invite codes private. Anyone with a code can try to redeem it."),
	))

	return container.NewVBox(
		u.howItWorks(),
		gap(sectionGap),
		hosting,
		gap(sectionGap),
		joining,
		gap(sectionGap),
		problems,
		gap(sectionGap),
		security,
		gap(rowGap),
		mutedLabel("App port "+strconv.Itoa(settings.AppPort)+" · UDP"),
	)
}

func (u *UI) howItWorks() fyne.CanvasObject {
	left := container.NewBorder(nil, nil, nil, metaText("your server"), addressText("192.168.1.50:25565"))
	middle := container.NewCenter(metaText("encrypted"))
	right := container.NewBorder(nil, nil, nil, metaText("their game"), addressText("127.0.0.1:25565"))

	return card(container.NewVBox(
		sectionLabel("How it works"),
		gap(rowGap),
		left,
		gap(tinyGap),
		middle,
		gap(tinyGap),
		right,
		gap(rowGap),
		mutedLabel("Example addresses. Only published ports cross."),
	))
}
