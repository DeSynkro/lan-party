package gtkui

import (
	"strings"

	invitepkg "github.com/desynkro/lan-party/internal/invite"
)

func joinInvitePreview(code string) (nameLine, groupLine string) {
	inv, err := invitepkg.Decode(strings.TrimSpace(code))
	if err != nil {
		return "", ""
	}
	return "The host has assigned you the name " + inv.PeerName + ".",
		"Joining " + inv.GroupName + " · hosted by " + inv.HostName + "."
}
