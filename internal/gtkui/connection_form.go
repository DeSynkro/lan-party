package gtkui

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
