// Package app owns the stored groups, the key policy and the running sessions.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/desynkro/lan-party/internal/autostart"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/session"
	"github.com/desynkro/lan-party/internal/tunnel"
)

// App is the long-lived application state.
type App struct {
	paths config.Paths

	mu          sync.Mutex
	settings    config.Settings
	store       *group.Store
	groups      []group.Group
	keys        *secure.Store
	host        *session.Host
	friend      *session.Friend
	liveGroupID string
	pending     []*pendingApproval
	inviteCodes map[string]string
	limiter     *attemptLimiter

	reconnectMu   sync.Mutex
	autoReconnect *reconnectState
	sessionEpoch  uint64
}

var errSessionStopped = errors.New("the session was stopped")

// New opens the stores and uses owner-only key files under the data dir.
func New(paths config.Paths) (*App, error) {
	return NewWithStore(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")))
}

// NewWithStore is New with an injectable key store, used by the tests.
func NewWithStore(paths config.Paths, kr secure.SecretStore) (*App, error) {
	settings, err := config.LoadSettings(paths)
	if err != nil {
		return nil, err
	}
	_ = autostart.Set(filepath.Dir(paths.ConfigDir), settings.StartOnLogin)
	store := group.OpenStore(filepath.Join(paths.DataDir, "groups.json"))
	groups, err := store.Load()
	if err != nil {
		return nil, err
	}
	return &App{
		paths:       paths,
		settings:    settings,
		store:       store,
		groups:      groups,
		keys:        secure.NewStore(kr),
		inviteCodes: map[string]string{},
		limiter:     newAttemptLimiter(),
	}, nil
}

// Close ends any running session.
func (a *App) Close() error {
	return a.StopSession()
}

// Groups returns a copy of the saved groups.
func (a *App) Groups() []group.Group {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]group.Group, len(a.groups))
	for i, g := range a.groups {
		out[i] = cloneGroup(g)
	}
	return out
}

// Group returns one saved group.
func (a *App) Group(id string) (group.Group, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := a.groupIndexLocked(id)
	if idx < 0 {
		return group.Group{}, false
	}
	return cloneGroup(a.groups[idx]), true
}

// DataDir is where group data lives.
func (a *App) DataDir() string { return a.paths.DataDir }

// ConfigDir is where settings live.
func (a *App) ConfigDir() string { return a.paths.ConfigDir }

// Settings returns the current settings.
func (a *App) Settings() config.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SaveSettings validates and writes the settings.
func (a *App) SaveSettings(s config.Settings) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.host != nil && s.AppPort != a.settings.AppPort {
		idx := a.groupIndexLocked(a.liveGroupID)
		if idx < 0 || a.groups[idx].AppPort == 0 {
			return errors.New("end the session before changing the app port")
		}
	}
	if err := s.Validate(); err != nil {
		return err
	}

	cancelled := a.cancelInheritedInvitesLocked(s)
	if len(cancelled) > 0 {
		if err := a.saveLocked(); err != nil {
			a.restoreInvitesLocked(cancelled)
			return err
		}
	}

	if err := config.SaveSettings(a.paths, s); err != nil {
		a.rollbackInvitesLocked(cancelled)
		return err
	}
	if err := autostart.Set(filepath.Dir(a.paths.ConfigDir), s.StartOnLogin); err != nil {
		_ = config.SaveSettings(a.paths, a.settings)
		a.rollbackInvitesLocked(cancelled)
		return err
	}
	wasAutoReconnect := a.settings.AutoReconnect
	a.settings = s
	if !s.AutoReconnect {
		a.stopAutoReconnectLocked()
	} else if !wasAutoReconnect && a.friend != nil && !a.friend.Presence().Live && a.liveGroupID != "" {
		a.startAutoReconnectLocked(a.liveGroupID, time.Now())
	}
	return nil
}

// rollbackInvitesLocked restores cancelled invites and persists the restore.
// Callers hold a.mu.
func (a *App) rollbackInvitesLocked(cancelled []inviteSnapshot) {
	if len(cancelled) == 0 {
		return
	}
	a.restoreInvitesLocked(cancelled)
	_ = a.saveLocked()
}

// SetPublicAddress updates the address friends connect to.
func (a *App) SetPublicAddress(value string) error {
	s := a.Settings()
	s.PublicAddress = strings.TrimSpace(value)
	return a.SaveSettings(s)
}

// SetAppPort updates the local UDP port.
func (a *App) SetAppPort(port int) error {
	s := a.Settings()
	s.AppPort = port
	return a.SaveSettings(s)
}

// LiveGroupID is the group with a running session, if any. A friend session
// that has lost its host is not live, even though its saved group remains.
func (a *App) LiveGroupID() string {
	a.mu.Lock()
	id := a.liveGroupID
	friend := a.friend
	a.mu.Unlock()

	if friend != nil && !friend.Presence().Live {
		return ""
	}
	return id
}

// DefaultHostName is a reasonable first guess for the host's display name.
func DefaultHostName() string {
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return "Host"
}

// DeviceName is the reported name of this machine.
func (a *App) DeviceName() string {
	if name, err := os.Hostname(); err == nil && strings.TrimSpace(name) != "" {
		return name
	}
	return "this device"
}

// CreateGroup makes a host group and its long-lived host key.
func (a *App) CreateGroup(name, hostName string) (group.Group, error) {
	name = strings.TrimSpace(name)
	hostName = strings.TrimSpace(hostName)
	if name == "" {
		return group.Group{}, errors.New("give the group a name")
	}
	if hostName == "" {
		return group.Group{}, errors.New("tell your friends what to call you")
	}
	if len(name) > 60 || len(hostName) > 40 {
		return group.Group{}, errors.New("that name is too long")
	}

	g := group.CreateHostGroup(name, hostName)
	if _, err := a.keys.LoadOrCreate("host/" + g.ID); err != nil {
		return group.Group{}, fmt.Errorf("host key: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.groups = append(a.groups, g)
	if err := a.saveLocked(); err != nil {
		a.groups = a.groups[:len(a.groups)-1]
		return group.Group{}, err
	}
	return cloneGroup(g), nil
}

// DeleteGroup removes a group, stops its session and deletes its key.
func (a *App) DeleteGroup(id string) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(id)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	g := a.groups[idx]
	live := a.liveGroupID == id
	if a.autoReconnect != nil && a.autoReconnect.groupID == id {
		a.stopAutoReconnectLocked()
	}
	a.mu.Unlock()

	if live {
		_ = a.StopSession()
	}

	a.mu.Lock()
	idx = a.groupIndexLocked(id)
	if idx < 0 {
		a.mu.Unlock()
		return nil
	}
	old := a.groups
	next := make([]group.Group, 0, len(a.groups)-1)
	next = append(next, a.groups[:idx]...)
	next = append(next, a.groups[idx+1:]...)
	a.groups = next
	if err := a.saveLocked(); err != nil {
		a.groups = old
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()

	keyName := "host/" + id
	if g.Role == group.RoleFriend {
		if g.HostGroupID != "" {
			keyName = "device/" + g.HostGroupID
		} else {
			keyName = "device/" + id
		}
	}
	if err := a.keys.Delete(keyName); err != nil {
		return fmt.Errorf("remove key: %w", err)
	}
	return nil
}

// hostForGroupLocked returns the running host only when it belongs to groupID.
// Callers hold a.mu.
func (a *App) hostForGroupLocked(groupID string) *session.Host {
	if a.host != nil && a.liveGroupID == groupID {
		return a.host
	}
	return nil
}

// AddService publishes another service. A live session gets the update.
func (a *App) AddService(groupID string, s group.Service) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	if a.groups[idx].Role != group.RoleHost {
		a.mu.Unlock()
		return errors.New("only the host can publish services")
	}
	if err := a.groups[idx].AddService(s); err != nil {
		a.mu.Unlock()
		return err
	}
	services := append([]group.Service(nil), a.groups[idx].Services...)
	host := a.hostForGroupLocked(groupID)
	err := a.saveLocked()
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if host != nil {
		return host.UpdateServices(services)
	}
	return nil
}

// UpdateService edits a published service. A live session gets the update.
func (a *App) UpdateService(groupID string, s group.Service) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	if a.groups[idx].Role != group.RoleHost {
		a.mu.Unlock()
		return errors.New("only the host can publish services")
	}
	old := append([]group.Service(nil), a.groups[idx].Services...)
	if err := a.groups[idx].UpdateService(s); err != nil {
		a.mu.Unlock()
		return err
	}
	services := append([]group.Service(nil), a.groups[idx].Services...)
	host := a.hostForGroupLocked(groupID)
	if err := a.saveLocked(); err != nil {
		a.groups[idx].Services = old
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()
	if host != nil {
		return host.UpdateServices(services)
	}
	return nil
}

// RemoveService unpublishes a service.
func (a *App) RemoveService(groupID, serviceID string) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	if !a.groups[idx].RemoveService(serviceID) {
		a.mu.Unlock()
		return errors.New("service not found")
	}
	services := append([]group.Service(nil), a.groups[idx].Services...)
	host := a.hostForGroupLocked(groupID)
	err := a.saveLocked()
	a.mu.Unlock()
	if err != nil {
		return err
	}
	if host != nil {
		return host.UpdateServices(services)
	}
	return nil
}

// Endpoint combines the public address with the app port. A port written in
// the public address is the outside port; without one, the app port is used.
func Endpoint(public string, appPort int) (string, error) {
	return publicEndpoint(public, appPort)
}

// CurrentEndpoint is Endpoint using the saved settings.
func (a *App) CurrentEndpoint() (string, error) {
	a.mu.Lock()
	public := a.settings.PublicAddress
	appPort := a.settings.AppPort
	a.mu.Unlock()
	return publicEndpoint(public, appPort)
}

// GroupConnection is a host group's effective connection values.
type GroupConnection struct {
	PublicAddress string
	AppPort       int
	Custom        bool
}

// ValidateConnection checks a group's optional public address and app port.
func ValidateConnection(publicAddress string, appPort int) error {
	if err := config.ValidateAddress(strings.TrimSpace(publicAddress)); err != nil {
		return err
	}
	if appPort != 0 && (appPort < 1 || appPort > 65535) {
		return errors.New("the app port must be between 1 and 65535")
	}
	return nil
}

// EffectiveConnection resolves a group's overrides against the saved defaults.
func (a *App) EffectiveConnection(groupID string) (GroupConnection, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		return GroupConnection{}, errors.New("group not found")
	}
	g := a.groups[idx]
	public, appPort := a.effectiveConnectionLocked(g)
	return GroupConnection{
		PublicAddress: public,
		AppPort:       appPort,
		Custom:        g.PublicAddress != "" || g.AppPort != 0,
	}, nil
}

// GroupEndpoint resolves the endpoint friends use for this group.
func (a *App) GroupEndpoint(groupID string) (string, error) {
	connection, err := a.EffectiveConnection(groupID)
	if err != nil {
		return "", err
	}
	return publicEndpoint(connection.PublicAddress, connection.AppPort)
}

// GroupAppPort returns the UDP port the host listens on for this group.
func (a *App) GroupAppPort(groupID string) (int, error) {
	connection, err := a.EffectiveConnection(groupID)
	if err != nil {
		return 0, err
	}
	return connection.AppPort, nil
}

// SetGroupConnection saves a host group's optional connection overrides.
// Empty address and appPort zero both mean "use the saved defaults".
func (a *App) SetGroupConnection(groupID, publicAddress string, appPort int) error {
	publicAddress = strings.TrimSpace(publicAddress)
	if err := ValidateConnection(publicAddress, appPort); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		return errors.New("group not found")
	}
	g := &a.groups[idx]
	if !g.IsHost() {
		return errors.New("only a hosted group can change its connection")
	}

	oldPublic, oldPort := a.effectiveConnectionLocked(*g)
	newPublic := publicAddress
	if newPublic == "" {
		newPublic = a.settings.PublicAddress
	}
	newPort := appPort
	if newPort == 0 {
		newPort = a.settings.AppPort
	}
	if a.host != nil && a.liveGroupID == groupID && newPort != oldPort {
		return errors.New("end the session before changing this group's app port")
	}

	oldEndpoint, oldErr := publicEndpoint(oldPublic, oldPort)
	newEndpoint, newErr := publicEndpoint(newPublic, newPort)
	endpointChanged := oldErr == nil && (newErr != nil || oldEndpoint != newEndpoint)

	oldPublicOverride, oldPortOverride := g.PublicAddress, g.AppPort
	var cancelled []inviteSnapshot
	if endpointChanged {
		cancelled = append(cancelled, a.cancelOpenInvitesLocked(g, time.Now().UTC()))
	}

	g.PublicAddress = publicAddress
	g.AppPort = appPort
	if err := a.saveLocked(); err != nil {
		g.PublicAddress = oldPublicOverride
		g.AppPort = oldPortOverride
		a.restoreInvitesLocked(cancelled)
		return err
	}
	return nil
}

// inviteSnapshot records a group's invites and the codes that are only kept
// in memory, so a failed save can restore them.
type inviteSnapshot struct {
	groupID string
	invites []group.Invite
	codes   map[string]string
}

// cancelOpenInvitesLocked removes a group's redeemable invites and their
// in-memory codes. Callers hold a.mu.
func (a *App) cancelOpenInvitesLocked(g *group.Group, now time.Time) inviteSnapshot {
	snapshot := inviteSnapshot{
		groupID: g.ID,
		invites: append([]group.Invite(nil), g.Invites...),
		codes:   map[string]string{},
	}
	for _, inv := range g.OpenInvites(now) {
		if code, ok := a.inviteCodes[inv.ID]; ok {
			snapshot.codes[inv.ID] = code
			delete(a.inviteCodes, inv.ID)
		}
	}
	g.ClearOpenInvites(now)
	return snapshot
}

// restoreInvitesLocked puts back invites and codes captured by a snapshot.
// Callers hold a.mu.
func (a *App) restoreInvitesLocked(snapshots []inviteSnapshot) {
	for _, snapshot := range snapshots {
		idx := a.groupIndexLocked(snapshot.groupID)
		if idx < 0 {
			continue
		}
		a.groups[idx].Invites = snapshot.invites
		for id, code := range snapshot.codes {
			a.inviteCodes[id] = code
		}
	}
}

// cancelInheritedInvitesLocked removes invites whose endpoint changes because
// the network defaults change. Groups with a full connection override are
// unaffected. Callers hold a.mu.
func (a *App) cancelInheritedInvitesLocked(next config.Settings) []inviteSnapshot {
	now := time.Now().UTC()
	var cancelled []inviteSnapshot
	for i := range a.groups {
		g := &a.groups[i]
		if !g.IsHost() {
			continue
		}
		if strings.TrimSpace(g.PublicAddress) != "" && g.AppPort != 0 {
			continue
		}

		oldPublic := g.PublicAddress
		if strings.TrimSpace(oldPublic) == "" {
			oldPublic = a.settings.PublicAddress
		}
		oldPort := g.AppPort
		if oldPort == 0 {
			oldPort = a.settings.AppPort
		}
		newPublic := g.PublicAddress
		if strings.TrimSpace(newPublic) == "" {
			newPublic = next.PublicAddress
		}
		newPort := g.AppPort
		if newPort == 0 {
			newPort = next.AppPort
		}

		oldEndpoint, oldErr := publicEndpoint(oldPublic, oldPort)
		newEndpoint, newErr := publicEndpoint(newPublic, newPort)
		if oldErr != nil || (newErr == nil && oldEndpoint == newEndpoint) {
			continue
		}
		if len(g.OpenInvites(now)) == 0 {
			continue
		}
		cancelled = append(cancelled, a.cancelOpenInvitesLocked(g, now))
	}
	return cancelled
}

// effectiveConnectionLocked applies the group override or the saved default.
// Callers hold a.mu.
func (a *App) effectiveConnectionLocked(g group.Group) (string, int) {
	public := g.PublicAddress
	if strings.TrimSpace(public) == "" {
		public = a.settings.PublicAddress
	}
	appPort := g.AppPort
	if appPort == 0 {
		appPort = a.settings.AppPort
	}
	return public, appPort
}

// CreateInvite issues a one-time code for a named friend.
func (a *App) CreateInvite(groupID, peerName string) (string, error) {
	peerName = strings.TrimSpace(peerName)
	if peerName == "" {
		return "", errors.New("give the invite a name")
	}

	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return "", errors.New("group not found")
	}
	g := cloneGroup(a.groups[idx])
	public, appPort := a.effectiveConnectionLocked(g)
	a.mu.Unlock()

	if g.Role != group.RoleHost {
		return "", errors.New("only the host can create invites")
	}
	if strings.TrimSpace(public) == "" {
		return "", errors.New("set your public address first")
	}
	endpoint, err := publicEndpoint(public, appPort)
	if err != nil {
		return "", err
	}
	identity, ok, err := a.keys.Load("host/" + groupID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("the host key for this group is missing. If this group came from an older build, delete it and create it again")
	}

	token, hash, err := invite.NewToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	record := group.Invite{
		ID:        group.NewID(),
		Name:      peerName,
		TokenHash: hash,
		CreatedAt: now,
		ExpiresAt: now.Add(group.InviteTTL),
	}
	code, err := invite.Encode(invite.Invite{
		Version:   invite.CurrentVersion,
		GroupID:   g.ID,
		GroupName: g.Name,
		HostName:  g.HostName,
		Endpoint:  endpoint,
		HostPin:   secure.Pin(identity.Public),
		Token:     token,
		PeerName:  peerName,
		Expires:   record.ExpiresAt,
	})
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	idx = a.groupIndexLocked(groupID)
	if idx < 0 {
		return "", errors.New("group not found")
	}
	if err := a.groups[idx].AddInvite(record); err != nil {
		return "", err
	}
	if err := a.saveLocked(); err != nil {
		a.groups[idx].Invites = a.groups[idx].Invites[:len(a.groups[idx].Invites)-1]
		return "", err
	}
	a.inviteCodes[record.ID] = code
	return code, nil
}

// InviteCode returns a code created in this run. Codes are not written to
// disk, so a restart makes old invites display-only.
func (a *App) InviteCode(inviteID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inviteCodes[inviteID]
}

// RemoveInvite cancels an open invite.
func (a *App) RemoveInvite(groupID, inviteID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		return errors.New("group not found")
	}
	for i, inv := range a.groups[idx].Invites {
		if inv.ID == inviteID {
			a.groups[idx].Invites = append(a.groups[idx].Invites[:i], a.groups[idx].Invites[i+1:]...)
			delete(a.inviteCodes, inviteID)
			return a.saveLocked()
		}
	}
	return errors.New("invite not found")
}

// StartSession opens the tunnel for a host group.
func (a *App) StartSession(groupID string) error {
	a.stopAutoReconnect()
	a.reconnectMu.Lock()
	defer a.reconnectMu.Unlock()

	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	if a.groups[idx].Role != group.RoleHost {
		a.mu.Unlock()
		return errors.New("only a group you host can be started")
	}
	epoch := a.sessionEpoch
	a.stopAutoReconnectLocked()
	var stale *session.Friend
	if a.friend != nil && !a.friend.Presence().Live {
		stale = a.friend
		a.friend = nil
		a.liveGroupID = ""
	}
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		if stale != nil {
			_ = stale.Close()
		}
		return errors.New("a session is already running")
	}
	services := append([]group.Service(nil), a.groups[idx].Services...)
	_, appPort := a.effectiveConnectionLocked(a.groups[idx])
	a.mu.Unlock()
	if stale != nil {
		_ = stale.Close()
	}

	identity, ok, err := a.keys.Load("host/" + groupID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("the host key for this group is missing. If this group came from an older build, delete it and create it again")
	}

	var host *session.Host
	deadline := time.Now().Add(3 * time.Second)
	for {
		host, err = session.StartHost(context.Background(), session.HostConfig{
			Identity:     identity,
			GroupID:      groupID,
			AppPort:      appPort,
			Services:     services,
			Authenticate: hostAuth{app: a, groupID: groupID},
		})
		if err == nil {
			break
		}
		if !isAddrInUse(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}

	a.mu.Lock()
	if a.sessionEpoch != epoch {
		a.mu.Unlock()
		_ = host.Close()
		return errSessionStopped
	}
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		_ = host.Close()
		return errors.New("a session is already running")
	}
	a.host = host
	a.liveGroupID = groupID
	a.mu.Unlock()
	return nil
}

// StopSession closes whatever is running.
func (a *App) StopSession() error {
	a.mu.Lock()
	a.sessionEpoch++
	a.stopAutoReconnectLocked()
	host := a.host
	friend := a.friend
	a.host = nil
	a.friend = nil
	a.liveGroupID = ""
	a.pending = nil
	a.mu.Unlock()

	if host != nil {
		_ = host.Close()
	}
	if friend != nil {
		_ = friend.Close()
	}
	return nil
}

// JoinInvite accepts a pasted code and waits for the host's approval.
func (a *App) JoinInvite(ctx context.Context, code, deviceName string) (group.Group, error) {
	a.stopAutoReconnect()
	a.reconnectMu.Lock()
	defer a.reconnectMu.Unlock()

	inv, err := invite.Decode(code)
	if err != nil {
		return group.Group{}, err
	}

	a.mu.Lock()
	a.stopAutoReconnectLocked()
	epoch := a.sessionEpoch
	var stale *session.Friend
	if a.friend != nil && !a.friend.Presence().Live {
		stale = a.friend
		a.friend = nil
		a.liveGroupID = ""
	}
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		if stale != nil {
			_ = stale.Close()
		}
		return group.Group{}, errors.New("end the current session first")
	}
	for _, g := range a.groups {
		if g.Role != group.RoleFriend {
			continue
		}
		if g.HostGroupID == inv.GroupID || (g.HostGroupID == "" && g.HostPin == inv.HostPin) {
			a.mu.Unlock()
			if stale != nil {
				_ = stale.Close()
			}
			return group.Group{}, errors.New("you have already joined this group")
		}
	}
	a.mu.Unlock()
	if stale != nil {
		_ = stale.Close()
	}

	if strings.TrimSpace(deviceName) == "" {
		deviceName = a.DeviceName()
	}
	identity, err := a.keys.LoadOrCreate("device/" + inv.GroupID)
	if err != nil {
		return group.Group{}, err
	}

	friend, err := session.Join(ctx, session.JoinConfig{
		Identity:   identity,
		Invite:     inv,
		DeviceName: deviceName,
		Token:      inv.Token,
	})
	if err != nil {
		return group.Group{}, err
	}

	g := group.CreateFriendGroup(
		inv.GroupName,
		inv.HostName,
		inv.Endpoint,
		inv.HostPin,
		friend.MemberID,
		inv.PeerName,
		friendServices(friend.Services()),
	)
	g.HostGroupID = inv.GroupID

	a.mu.Lock()
	if a.sessionEpoch != epoch {
		a.mu.Unlock()
		_ = friend.Close()
		return group.Group{}, errSessionStopped
	}
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		_ = friend.Close()
		return group.Group{}, errors.New("a session started while joining")
	}
	a.groups = append(a.groups, g)
	if err := a.saveLocked(); err != nil {
		a.groups = a.groups[:len(a.groups)-1]
		a.mu.Unlock()
		_ = friend.Close()
		return group.Group{}, err
	}
	a.friend = friend
	a.liveGroupID = g.ID
	a.mu.Unlock()
	go a.watchFriend(friend, g.ID)
	return cloneGroup(g), nil
}

// Reconnect reopens a saved friend group using the saved device key.
func (a *App) Reconnect(ctx context.Context, groupID string) error {
	a.stopAutoReconnect()
	a.reconnectMu.Lock()
	err := a.reconnect(ctx, groupID)
	a.reconnectMu.Unlock()
	if err != nil && ctx.Err() == nil && !isPermanentReconnect(err) {
		a.startAutoReconnect(groupID, time.Now())
	}
	return err
}

// reconnect is the shared manual and automatic reconnect path.
func (a *App) reconnect(ctx context.Context, groupID string) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return permanentReconnect(errors.New("group not found"))
	}
	if a.groups[idx].Role != group.RoleFriend {
		a.mu.Unlock()
		return permanentReconnect(errors.New("only a joined group can be reconnected"))
	}
	epoch := a.sessionEpoch
	if a.host != nil {
		a.mu.Unlock()
		return permanentReconnect(errors.New("a session is already running"))
	}
	var stale *session.Friend
	if a.friend != nil {
		if a.friend.Presence().Live {
			a.mu.Unlock()
			return permanentReconnect(errors.New("a session is already running"))
		}
		stale = a.friend
		a.friend = nil
		a.liveGroupID = ""
	}
	g := cloneGroup(a.groups[idx])
	a.mu.Unlock()
	if stale != nil {
		_ = stale.Close()
	}

	if g.HostGroupID == "" {
		return permanentReconnect(errors.New("this joined group was saved by an older build; delete it and join again with a new invite"))
	}
	identity, ok, err := a.keys.Load("device/" + g.HostGroupID)
	if err != nil {
		return err
	}
	if !ok {
		return permanentReconnect(errors.New("the device key for this group is missing. If this group came from an older build, delete it and join again"))
	}

	friend, err := session.Join(ctx, session.JoinConfig{
		Identity:   identity,
		Invite:     invite.Invite{Version: invite.CurrentVersion, GroupID: g.HostGroupID, GroupName: g.Name, HostName: g.HostName, Endpoint: g.Endpoint, HostPin: g.HostPin, PeerName: g.SelfName},
		DeviceName: a.DeviceName(),
		MemberID:   g.SelfID,
	})
	if err != nil {
		if errors.Is(err, tunnel.ErrDenied) {
			return permanentReconnect(err)
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = friend.Close()
		return err
	}

	a.mu.Lock()
	if err := ctx.Err(); err != nil {
		a.mu.Unlock()
		_ = friend.Close()
		return err
	}
	if a.sessionEpoch != epoch {
		a.mu.Unlock()
		_ = friend.Close()
		return permanentReconnect(errSessionStopped)
	}
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		_ = friend.Close()
		return permanentReconnect(errors.New("a session started while reconnecting"))
	}
	idx = a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		_ = friend.Close()
		return permanentReconnect(errors.New("group not found"))
	}
	a.groups[idx].SelfID = friend.MemberID
	a.groups[idx].Services = friendServices(friend.Services())
	if err := a.saveLocked(); err != nil {
		a.mu.Unlock()
		_ = friend.Close()
		return err
	}
	a.friend = friend
	a.liveGroupID = groupID
	a.mu.Unlock()
	go a.watchFriend(friend, groupID)
	return nil
}

// HostPresence is the live state of each connected member.
func (a *App) HostPresence() map[string]session.Presence {
	a.mu.Lock()
	host := a.host
	a.mu.Unlock()
	if host == nil {
		return nil
	}
	return host.Presence()
}

// FriendBounds is the current list of local game addresses.
func (a *App) FriendBounds() []session.Bound {
	a.mu.Lock()
	friend := a.friend
	a.mu.Unlock()
	if friend == nil {
		return nil
	}
	return friend.Bound()
}

// FriendPresence is whether the friend can still see the host.
func (a *App) FriendPresence() session.Presence {
	a.mu.Lock()
	friend := a.friend
	a.mu.Unlock()
	if friend == nil {
		return session.Presence{}
	}
	return friend.Presence()
}

// FriendNote explains why a friend session ended.
func (a *App) FriendNote() string {
	a.mu.Lock()
	friend := a.friend
	a.mu.Unlock()
	if friend == nil {
		return ""
	}
	return friend.Note()
}

// RemoveMember revokes a friend and drops them from the group.
func (a *App) RemoveMember(groupID, memberID string) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	if a.groups[idx].Role != group.RoleHost {
		a.mu.Unlock()
		return errors.New("only the host can remove members")
	}
	a.mu.Unlock()

	a.mu.Lock()
	host := a.host
	a.mu.Unlock()
	if host != nil {
		_ = host.Revoke(memberID)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	idx = a.groupIndexLocked(groupID)
	if idx < 0 {
		return errors.New("group not found")
	}
	if !a.groups[idx].RemoveMember(memberID) {
		return errors.New("member not found")
	}
	return a.saveLocked()
}

func (a *App) saveLocked() error {
	return a.store.Save(a.groups)
}

func (a *App) groupIndexLocked(id string) int {
	for i, g := range a.groups {
		if g.ID == id {
			return i
		}
	}
	return -1
}

func cloneGroup(g group.Group) group.Group {
	g.Services = append([]group.Service(nil), g.Services...)
	for i := range g.Services {
		g.Services[i].Ports = append([]int(nil), g.Services[i].Ports...)
	}
	g.Members = append([]group.Member(nil), g.Members...)
	g.Invites = append([]group.Invite(nil), g.Invites...)
	return g
}

func friendServices(infos []tunnel.ServiceInfo) []group.Service {
	out := make([]group.Service, 0, len(infos))
	for _, s := range infos {
		out = append(out, group.Service{
			ID:    s.ID,
			Name:  s.Name,
			Proto: s.Proto,
			Ports: append([]int(nil), s.Ports...),
			Hint:  s.Hint,
		})
	}
	return out
}

func publicEndpoint(public string, appPort int) (string, error) {
	public = strings.TrimSpace(public)
	if public == "" {
		return "", errors.New("set your public address first")
	}
	if err := config.ValidateAddress(public); err != nil {
		return "", err
	}
	if host, port, err := net.SplitHostPort(public); err == nil {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n < 1 || n > 65535 {
			return "", errors.New("the public address port is invalid")
		}
		return net.JoinHostPort(host, strconv.Itoa(n)), nil
	}
	if ip := net.ParseIP(public); ip != nil {
		return net.JoinHostPort(public, strconv.Itoa(appPort)), nil
	}
	if strings.ContainsAny(public, " /:") {
		return "", errors.New("the public address is invalid")
	}
	return net.JoinHostPort(public, strconv.Itoa(appPort)), nil
}
