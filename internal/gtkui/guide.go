package gtkui

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func (u *probeUI) buildGuidePage() *gtk.ScrolledWindow {
	settings := u.state.Settings()
	relayOn := settings.UseRelay && strings.TrimSpace(settings.RelayHost) != ""
	rule := fmt.Sprintf("Forward the group's UDP app port (default %d) to %s", settings.AppPort, localIPv4())
	if relayOn {
		rule = "Relay: " + settings.RelayHost + " · port " + strconv.Itoa(settings.RelayPort)
	}

	page := gtk.NewBox(gtk.OrientationVertical, 0)
	page.AddCSSClass("page")
	page.SetMarginTop(20)
	page.SetMarginBottom(20)
	page.SetMarginStart(20)
	page.SetMarginEnd(20)

	how := gtk.NewBox(gtk.OrientationVertical, 0)
	how.AddCSSClass("card")
	how.SetMarginBottom(10)
	how.Append(textLabel("How it works", "card-title"))
	how.Append(gapBox(10))
	how.Append(guideAddressRow("192.168.1.50:25565", "your server"))
	how.Append(gapBox(2))
	encrypted := textLabel("encrypted", "desc")
	encrypted.SetHAlign(gtk.AlignCenter)
	how.Append(encrypted)
	how.Append(gapBox(2))
	how.Append(guideAddressRow("127.0.0.1:25565", "their game"))
	how.Append(gapBox(10))
	how.Append(cardText("Example addresses. Only published ports cross.", "desc"))
	page.Append(how)

	hosting := gtk.NewBox(gtk.OrientationVertical, 0)
	hosting.AddCSSClass("card")
	hosting.SetMarginBottom(10)
	hosting.Append(textLabel("Hosting", "card-title"))
	hosting.Append(gapBox(10))
	hosting.Append(cardText("1. Open your group and add the game server you want to share. Use the IP and port it listens on.", "desc"))
	hosting.Append(gapBox(10))
	if relayOn {
		hosting.Append(cardText("2. Friends connect through your relay. No port forward is needed on this PC.", "desc"))
	} else {
		hosting.Append(cardText("2. Forward one UDP port on your router. That is the app port, not the game port.", "desc"))
	}
	hosting.Append(gapBox(10))

	copyButton := gtk.NewButtonWithLabel("Copy")
	copyButton.AddCSSClass("secondary")
	copyButton.ConnectClicked(func() {
		u.win.Widget.Clipboard().SetText(rule)
		copyButton.SetLabel("Copied")
	})
	ruleLabel := textLabel(rule, "mono")
	ruleLabel.SetHExpand(true)
	ruleRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	ruleRow.Append(ruleLabel)
	ruleRow.Append(copyButton)
	hosting.Append(ruleRow)
	hosting.Append(gapBox(10))
	if relayOn {
		hosting.Append(cardText("The relay is the only reachable machine. The host PC needs no inbound port, and the game ports stay closed.", "desc"))
	} else {
		hosting.Append(cardText("Only this one port faces the internet. The game ports themselves stay closed.", "desc"))
	}
	hosting.Append(gapBox(10))

	addressLine := "No public address set yet. Open Settings and fill it in."
	if relayOn {
		addressLine = "Relay route: " + settings.RelayHost + " · port " + strconv.Itoa(settings.RelayPort) + " · Relay ID " + shortRelayID(settings.RelayID)
	} else if strings.TrimSpace(settings.PublicAddress) != "" {
		addressLine = "Your public address: " + settings.PublicAddress
	}
	hosting.Append(cardText(addressLine, "desc"))
	settingsButton := gtk.NewButtonWithLabel("Settings")
	settingsButton.AddCSSClass("quiet")
	settingsButton.SetHAlign(gtk.AlignStart)
	settingsButton.ConnectClicked(func() {
		u.showSettings()
	})
	hosting.Append(settingsButton)
	hosting.Append(gapBox(10))
	hosting.Append(cardText("3. Start the session, create an invite, and send the code to your friend.", "desc"))
	hosting.Append(gapBox(10))
	hosting.Append(cardText("4. When your friend joins, an approval card shows their device name and address. Allow it once.", "desc"))
	page.Append(hosting)

	joining := gtk.NewBox(gtk.OrientationVertical, 0)
	joining.AddCSSClass("card")
	joining.SetMarginBottom(10)
	joining.Append(textLabel("Joining", "card-title"))
	joining.Append(gapBox(10))
	joining.Append(cardText("Paste the invite code and wait for the host to approve your device.", "desc"))
	joining.Append(gapBox(10))
	joining.Append(cardText("Open the game, then use the address shown under Ready to play. For Minecraft Java that is usually Direct Connect.", "desc"))
	page.Append(joining)

	problems := gtk.NewBox(gtk.OrientationVertical, 0)
	problems.AddCSSClass("card")
	problems.SetMarginBottom(10)
	problems.Append(textLabel("Problems", "card-title"))
	problems.Append(gapBox(10))
	problems.Append(cardText("A friend can't reach you", "setting-title"))
	problems.Append(cardText("Check the public address in Settings, the UDP rule on your router, and the firewall on this PC. A VPN on the host can also hide the address. If your router's WAN address differs from your public address, your ISP may use carrier-grade NAT; use Settings → Use Relay with your self-hosted relay instead, and choose Test relay.", "desc"))
	problems.Append(gapBox(10))
	problems.Append(cardText("Your public address changed", "setting-title"))
	problems.Append(cardText("Update it in Settings. Old invites still point at the old address, so create new ones after the change.", "desc"))
	problems.Append(gapBox(10))
	problems.Append(cardText("A port is already in use", "setting-title"))
	problems.Append(cardText("The friend app tries the next local port and shows the address it actually got. The host's published port does not change.", "desc"))
	problems.Append(gapBox(10))
	problems.Append(cardText("An invite expired or was already used", "setting-title"))
	problems.Append(cardText("Create a new invite. Codes work once and expire after 1 hour.", "desc"))
	page.Append(problems)

	security := gtk.NewBox(gtk.OrientationVertical, 0)
	security.AddCSSClass("card")
	security.SetMarginBottom(10)
	security.Append(textLabel("Security", "card-title"))
	security.Append(gapBox(10))
	security.Append(cardText("Invites are single use, expire after 1 hour, and need the host to be online when they are redeemed.", "desc"))
	security.Append(gapBox(10))
	security.Append(cardText("The host approves each device. A different device key asks again.", "desc"))
	security.Append(gapBox(10))
	security.Append(cardText("Only the services you publish exist. Friends cannot reach your router, your other devices, or each other.", "desc"))
	security.Append(gapBox(10))
	security.Append(cardText("Keep invite codes private. Anyone with a code can try to redeem it.", "desc"))
	page.Append(security)

	page.Append(gapBox(10))
	if relayOn {
		page.Append(cardText("Relay "+settings.RelayHost+" · port "+strconv.Itoa(settings.RelayPort)+" · no port forward", "desc"))
	} else {
		page.Append(cardText("Default app port "+strconv.Itoa(settings.AppPort)+" · UDP", "desc"))
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(page)
	scroll.SetVExpand(true)
	return scroll
}

func guideAddressRow(address, note string) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 8)
	addr := textLabel(address, "mono")
	addr.SetHExpand(true)
	row.Append(addr)
	row.Append(textLabel(note, "meta"))
	return row
}

func localIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "this PC"
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if v4 := ipnet.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	return "this PC"
}

const cardTextWidth = 300

func cardText(text, class string) *gtk.Label {
	label := textLabel(text, class)
	label.SetSizeRequest(cardTextWidth, -1)
	return label
}
