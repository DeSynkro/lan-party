package webui

import (
	"github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

// App is the slice of the engine the web UI uses. *app.App satisfies it.
type App interface {
	Groups() []group.Group
	Group(id string) (group.Group, bool)
	Settings() config.Settings
	SaveSettings(config.Settings) error
	SetPublicAddress(string) error
	LiveGroupID() string
	HostPresence() map[string]session.Presence
	HostNote() string
	Approval() *app.Approval
	ResolveApproval(id string, allow bool) bool
	CreateGroup(name, hostName string) (group.Group, error)
	DeleteGroup(id string) error
	AddService(groupID string, s group.Service) error
	UpdateService(groupID string, s group.Service) error
	RemoveService(groupID, serviceID string) error
	CreateInvite(groupID, peerName string) (string, error)
	InviteCode(inviteID string) string
	RemoveInvite(groupID, inviteID string) error
	StartSession(groupID string) error
	StopSession() error
	RemoveMember(groupID, memberID string) error
	GroupEndpoint(groupID string) (string, error)
	GroupAppPort(groupID string) (int, error)
	DeviceName() string
	DataDir() string
	ConfigDir() string
	Close() error
}
