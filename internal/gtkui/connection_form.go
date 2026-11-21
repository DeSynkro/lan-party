package gtkui

import (
	"strconv"
	"strings"

	apppkg "github.com/desynkro/lan-party/internal/app"
)

func connectionEndpointNote(endpointChanges, portChanges bool, openInvites int, restart bool) string {
	after := "saving"
	if restart {
		after = "the restart"
	}
	switch {
	case endpointChanges && openInvites > 0:
		return "The advertised endpoint changes. This cancels " +
			plural(openInvites, "open invite") + ". Create new invites after " + after + "."
	case portChanges && !endpointChanges:
		return "The advertised address stays the same. Update your router forward to the new local port."
	default:
		return ""
	}
}

// liveConnectionChange reports whether applying next would need the live host
// session to restart. An address-only direct change keeps the same UDP socket.
func liveConnectionChange(current, next apppkg.GroupConnection) bool {
	if current.UseRelay || next.UseRelay {
		return current.UseRelay != next.UseRelay ||
			current.RelayHost != next.RelayHost ||
			current.RelayPort != next.RelayPort ||
			current.RelayID != next.RelayID
	}
	return current.AppPort != next.AppPort
}

func connectionChangeNote(relayChanges, endpointChanges, portChanges bool, openInvites int, restart bool) string {
	if relayChanges {
		if openInvites == 0 {
			return ""
		}
		after := "saving"
		if restart {
			after = "the restart"
		}
		return "The relay changes. This cancels " + plural(openInvites, "open invite") +
			". Create new invites after " + after + "."
	}
	return connectionEndpointNote(endpointChanges, portChanges, openInvites, restart)
}

func shortRelayID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 16 {
		return id
	}
	return id[:16] + "…"
}

func relayRoute(host string, port int, id string) string {
	return host + " · port " + strconv.Itoa(port) + " · Relay ID " + shortRelayID(id)
}

// connectionDialogHeight keeps the group connection dialog compact when the
// inherited summary is all that is shown.
func connectionDialogHeight(override, relay, custom bool) int {
	switch {
	case !override:
		return 330
	case !relay:
		return 480
	case !custom:
		return 520
	default:
		return 620
	}
}
