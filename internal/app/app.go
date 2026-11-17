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
}

// New opens the stores and starts with the OS keyring.
func New(paths config.Paths) (*App, error) {
	return NewWithKeyring(paths, secure.OSKeyring())
}

// NewWithKeyring is New with an injectable keyring, used by the tests.
func NewWithKeyring(paths config.Paths, kr secure.Keyring) (*App, error) {
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
		return errors.New("end the session before changing the app port")
	}
	if err := config.SaveSettings(a.paths, s); err != nil {
		return err
	}
	if err := autostart.Set(filepath.Dir(a.paths.ConfigDir), s.StartOnLogin); err != nil {
		_ = config.SaveSettings(a.paths, a.settings)
		return err
	}
	a.settings = s
	return nil
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
		keyName = "device/" + id
	}
	if err := a.keys.Delete(keyName); err != nil {
		return fmt.Errorf("remove key: %w", err)
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
	host := a.host
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
	host := a.host
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
	public := a.settings.PublicAddress
	appPort := a.settings.AppPort
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
		return "", errors.New("the host key for this group is missing")
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
		ExpiresAt: now.Add(24 * time.Hour),
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
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		return errors.New("a session is already running")
	}
	services := append([]group.Service(nil), a.groups[idx].Services...)
	appPort := a.settings.AppPort
	a.mu.Unlock()

	identity, ok, err := a.keys.Load("host/" + groupID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("the host key for this group is missing")
	}

	host, err := session.StartHost(context.Background(), session.HostConfig{
		Identity:     identity,
		GroupID:      groupID,
		AppPort:      appPort,
		Services:     services,
		Authenticate: hostAuth{app: a, groupID: groupID},
	})
	if err != nil {
		return err
	}

	a.mu.Lock()
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
	inv, err := invite.Decode(code)
	if err != nil {
		return group.Group{}, err
	}

	a.mu.Lock()
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		return group.Group{}, errors.New("end the current session first")
	}
	for _, g := range a.groups {
		if g.ID == inv.GroupID && g.Role == group.RoleFriend {
			a.mu.Unlock()
			return group.Group{}, errors.New("you have already joined this group")
		}
	}
	a.mu.Unlock()

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

	a.mu.Lock()
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
	return cloneGroup(g), nil
}

// Reconnect reopens a saved friend group using the saved device key.
func (a *App) Reconnect(ctx context.Context, groupID string) error {
	a.mu.Lock()
	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		return errors.New("group not found")
	}
	if a.groups[idx].Role != group.RoleFriend {
		a.mu.Unlock()
		return errors.New("only a joined group can be reconnected")
	}
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		return errors.New("a session is already running")
	}
	g := cloneGroup(a.groups[idx])
	a.mu.Unlock()

	identity, ok, err := a.keys.Load("device/" + groupID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("the device key for this group is missing")
	}

	friend, err := session.Join(ctx, session.JoinConfig{
		Identity:   identity,
		Invite:     invite.Invite{Version: invite.CurrentVersion, GroupID: g.ID, GroupName: g.Name, HostName: g.HostName, Endpoint: g.Endpoint, HostPin: g.HostPin, PeerName: g.SelfName},
		DeviceName: a.DeviceName(),
		MemberID:   g.SelfID,
	})
	if err != nil {
		return err
	}

	a.mu.Lock()
	if a.host != nil || a.friend != nil {
		a.mu.Unlock()
		_ = friend.Close()
		return errors.New("a session started while reconnecting")
	}
	idx = a.groupIndexLocked(groupID)
	if idx < 0 {
		a.mu.Unlock()
		_ = friend.Close()
		return errors.New("group not found")
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
