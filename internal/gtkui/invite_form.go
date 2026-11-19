package gtkui

import (
	"fmt"
	"strings"

	apppkg "github.com/desynkro/lan-party/internal/app"
)

type inviteDialogState struct {
	Warning string
	Action  string
	Blocked bool
}

func inviteDialogStateFor(liveGroupID, groupID, liveGroupName string) inviteDialogState {
	switch {
	case liveGroupID == groupID:
		return inviteDialogState{Action: "Create invite"}
	case liveGroupID == "":
		return inviteDialogState{
			Warning: "Session is off. It will start when you create this invite.",
			Action:  "Start session and invite",
		}
	default:
		name := strings.TrimSpace(liveGroupName)
		if name == "" {
			name = "another group"
		}
		return inviteDialogState{
			Warning: "The session for " + name + " is live. End it before inviting from this group.",
			Action:  "Create invite",
			Blocked: true,
		}
	}
}

func ensureInviteSession(state *apppkg.App, groupID string) error {
	live := state.LiveGroupID()
	if live == groupID {
		return nil
	}
	if live != "" {
		name := "another group"
		if other, ok := state.Group(live); ok {
			name = other.Name
		}
		return fmt.Errorf("the session for %s is live; end it before inviting from this group", name)
	}
	return state.StartSession(groupID)
}
