// Package group holds the things a host owns: the group, its services, the
// people who joined, and the invites that have been issued.
package group

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Limits keep one group from becoming a port scanner or a memory problem.
const (
	MaxServices = 16
	MaxMembers  = 32

	// InviteTTL is how long a new invite stays valid.
	InviteTTL = time.Hour
)

// Role says which side of the connection this group belongs to.
type Role string

const (
	RoleHost   Role = "host"
	RoleFriend Role = "friend"
)

// Group is one LAN party. A host group owns services and invites; a friend
// group remembers the host, the pinned key and the services it was granted.
type Group struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Role          Role      `json:"role"`
	HostName      string    `json:"hostName"`
	PublicAddress string    `json:"publicAddress,omitempty"`
	AppPort       int       `json:"appPort,omitempty"`
	Endpoint      string    `json:"endpoint,omitempty"`
	HostPin       string    `json:"hostPin,omitempty"`
	HostGroupID   string    `json:"hostGroupId,omitempty"`
	SelfID        string    `json:"selfId,omitempty"`
	SelfName      string    `json:"selfName,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	Services      []Service `json:"services,omitempty"`
	Members       []Member  `json:"members,omitempty"`
	Invites       []Invite  `json:"invites,omitempty"`

	// Connection choices. OverrideSettings makes the group use its own values
	// instead of the saved network defaults. RelayCustom selects the group's
	// relay instead of the one in Settings.
	//
	// ConnectionVersion is 1 once a 1.2 build has saved this group. It keeps
	// the legacy migration below from re-enabling an override the user turned
	// off while the old values were still stored.
	OverrideSettings  bool   `json:"overrideSettings,omitempty"`
	ConnectionVersion int    `json:"connectionVersion,omitempty"`
	UseRelay          bool   `json:"useRelay,omitempty"`
	RelayCustom       bool   `json:"relayCustom,omitempty"`
	RelayHost         string `json:"relayHost,omitempty"`
	RelayPort         int    `json:"relayPort,omitempty"`
	RelayID           string `json:"relayID,omitempty"`
}

// NewID returns a short random identifier for a group, service or member.
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed")
	}
	return hex.EncodeToString(b[:])
}

// CreateHostGroup starts an empty group with the host as its owner.
func CreateHostGroup(name, hostName string) Group {
	return Group{
		ID:        NewID(),
		Name:      strings.TrimSpace(name),
		Role:      RoleHost,
		HostName:  strings.TrimSpace(hostName),
		CreatedAt: time.Now().UTC(),
	}
}

// CreateFriendGroup records a group that was joined with an invite.
func CreateFriendGroup(name, hostName, endpoint, hostPin, selfID, selfName string, services []Service) Group {
	return Group{
		ID:        NewID(),
		Name:      strings.TrimSpace(name),
		Role:      RoleFriend,
		HostName:  strings.TrimSpace(hostName),
		Endpoint:  strings.TrimSpace(endpoint),
		HostPin:   hostPin,
		SelfID:    selfID,
		SelfName:  strings.TrimSpace(selfName),
		CreatedAt: time.Now().UTC(),
		Services:  append([]Service(nil), services...),
	}
}

// IsHost reports whether this group is hosted here.
func (g Group) IsHost() bool { return g.Role == RoleHost }

// FindService looks up a published service.
func (g Group) FindService(id string) (Service, bool) {
	for _, s := range g.Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}

// FindMember looks up a member by id.
func (g Group) FindMember(id string) (Member, bool) {
	for _, m := range g.Members {
		if m.ID == id {
			return m, true
		}
	}
	return Member{}, false
}

// FindMemberByName looks up a member by their display name.
func (g Group) FindMemberByName(name string) (Member, bool) {
	for _, m := range g.Members {
		if m.Name == name {
			return m, true
		}
	}
	return Member{}, false
}

// FindMemberByKey looks up a member by the public key they present.
func (g Group) FindMemberByKey(publicKey string) (Member, bool) {
	if publicKey == "" {
		return Member{}, false
	}
	for _, m := range g.Members {
		if m.PublicKey == publicKey {
			return m, true
		}
	}
	return Member{}, false
}

// FindInvite looks up an invite by id.
func (g Group) FindInvite(id string) (Invite, bool) {
	for _, inv := range g.Invites {
		if inv.ID == id {
			return inv, true
		}
	}
	return Invite{}, false
}

// OpenInvites returns the invites that can still be redeemed.
func (g Group) OpenInvites(now time.Time) []Invite {
	var out []Invite
	for _, inv := range g.Invites {
		if inv.Open(now) {
			out = append(out, inv)
		}
	}
	return out
}

// ClearOpenInvites removes every redeemable invite and returns their ids.
// Redeemed and expired records stay for the host's history.
func (g *Group) ClearOpenInvites(now time.Time) []string {
	var removed []string
	kept := g.Invites[:0]
	for _, inv := range g.Invites {
		if inv.Open(now) {
			removed = append(removed, inv.ID)
			continue
		}
		kept = append(kept, inv)
	}
	g.Invites = kept
	return removed
}

// AddService validates and appends a service, refusing duplicate ids and
// overlapping ports.
func (g *Group) AddService(s Service) error {
	if len(g.Services) >= MaxServices {
		return fmt.Errorf("a group can publish at most %d services", MaxServices)
	}
	if s.ID == "" {
		s.ID = NewID()
	}
	if _, ok := g.FindService(s.ID); ok {
		return errors.New("that service already exists")
	}
	s, err := normalizeServiceTarget(s)
	if err != nil {
		return err
	}
	if err := ValidateService(s); err != nil {
		return err
	}
	for _, other := range g.Services {
		if err := portConflict(s, other); err != nil {
			return err
		}
	}
	g.Services = append(g.Services, s)
	return nil
}

// normalizeServiceTarget stores the canonical target form.
func normalizeServiceTarget(s Service) (Service, error) {
	target, _, err := ParseTarget(s.Target)
	if err != nil {
		return s, err
	}
	s.Target = target
	return s, nil
}

// UpdateService validates and replaces a service with the same id.
func (g *Group) UpdateService(s Service) error {
	idx := -1
	for i := range g.Services {
		if g.Services[i].ID == s.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errors.New("service not found")
	}
	s, err := normalizeServiceTarget(s)
	if err != nil {
		return err
	}
	if err := ValidateService(s); err != nil {
		return err
	}
	for i, other := range g.Services {
		if i == idx {
			continue
		}
		if err := portConflict(s, other); err != nil {
			return err
		}
	}
	g.Services[idx] = s
	return nil
}

// RemoveService drops a service. Active listeners are the session's problem.
func (g *Group) RemoveService(id string) bool {
	for i, s := range g.Services {
		if s.ID == id {
			g.Services = append(g.Services[:i], g.Services[i+1:]...)
			return true
		}
	}
	return false
}

// AddMember records a friend. A member starts pending until the host approves
// the device.
func (g *Group) AddMember(m Member) error {
	if len(g.Members) >= MaxMembers {
		return fmt.Errorf("a group can have at most %d members", MaxMembers)
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("a member needs a name")
	}
	if m.ID == "" {
		m.ID = NewID()
	}
	if _, ok := g.FindMember(m.ID); ok {
		return errors.New("that member already exists")
	}
	if m.PublicKey != "" {
		if _, ok := g.FindMemberByKey(m.PublicKey); ok {
			return errors.New("that device is already a member")
		}
	}
	if m.State == "" {
		m.State = MemberPending
	}
	if m.FirstSeen.IsZero() {
		m.FirstSeen = time.Now().UTC()
	}
	g.Members = append(g.Members, m)
	return nil
}

// ApproveMember marks a pending member as allowed.
func (g *Group) ApproveMember(id string) bool {
	for i := range g.Members {
		if g.Members[i].ID == id {
			g.Members[i].State = MemberApproved
			return true
		}
	}
	return false
}

// RemoveMember kicks a member out of the group.
func (g *Group) RemoveMember(id string) bool {
	for i, m := range g.Members {
		if m.ID == id {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)
			return true
		}
	}
	return false
}

// AddInvite records a token hash. The token itself is never stored.
func (g *Group) AddInvite(inv Invite) error {
	if strings.TrimSpace(inv.Name) == "" {
		return errors.New("an invite needs a name")
	}
	if inv.TokenHash == "" {
		return errors.New("an invite needs a token hash")
	}
	if inv.ID == "" {
		inv.ID = NewID()
	}
	if _, ok := g.FindInvite(inv.ID); ok {
		return errors.New("that invite already exists")
	}
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = time.Now().UTC()
	}
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = inv.CreatedAt.Add(InviteTTL)
	}
	g.Invites = append(g.Invites, inv)
	return nil
}

// RedeemInvite burns an invite if it is still valid.
func (g *Group) RedeemInvite(id string, now time.Time) bool {
	for i := range g.Invites {
		inv := &g.Invites[i]
		if inv.ID != id {
			continue
		}
		if !inv.Open(now) {
			return false
		}
		inv.RedeemedAt = now.UTC()
		return true
	}
	return false
}

func portConflict(a, b Service) error {
	for _, p := range a.Ports {
		for _, q := range b.Ports {
			if p == q && protoOverlap(a.Proto, b.Proto) {
				return fmt.Errorf("port %d is already used by %s", p, b.Name)
			}
		}
	}
	return nil
}

func protoOverlap(a, b string) bool {
	return a == b || a == "both" || b == "both"
}
