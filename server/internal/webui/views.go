package webui

import (
	"fmt"
	"strings"
	"time"

	"github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

// BaseView carries the fields every page template uses.
type BaseView struct {
	Title           string
	CSRF            string
	Authenticated   bool
	Flash           string
	Error           string
	Approval        *ApprovalView
	LiveGroupID     string
	MemberSignature string
	InviteSignature string
}

type errorView struct {
	BaseView
	Status  int
	Message string
}

// GroupView is the host-safe summary of a group.
type GroupView struct {
	ID           string
	Name         string
	HostName     string
	Live         bool
	Connected    int
	Members      int
	Services     int
	OpenInvites  int
	Endpoint     string
	AppPort      int
	HostNote     string
	CreatedAt    time.Time
	PendingCount int
}

// MemberView is the host-safe summary of a member.
type MemberView struct {
	ID          string
	Name        string
	DeviceName  string
	Address     string
	Fingerprint string
	FirstSeen   time.Time
	LastSeen    time.Time
	Live        bool
	Approved    bool
}

// InviteView is an invite without its secret token.
type InviteView struct {
	ID        string
	Name      string
	CreatedAt time.Time
	ExpiresAt time.Time
	Open      bool
	Redeemed  bool
	Code      string
	Expired   bool
}

// ServiceView is one published service.
type ServiceView struct {
	ID        string
	Name      string
	Proto     string
	Target    string
	Ports     []int
	PortsText string
	Hint      string
}

// ApprovalView is a pending device approval.
type ApprovalView struct {
	ID          string
	GroupID     string
	PeerName    string
	DeviceName  string
	Address     string
	Fingerprint string
}

// DashboardView is the home page.
type DashboardView struct {
	BaseView
	Groups  []GroupView
	Pending *ApprovalView
}

// GroupPageView is one group's page.
type GroupPageView struct {
	BaseView
	Group    GroupView
	Services []ServiceView
	Members  []MemberView
	Invites  []InviteView
	Approval *ApprovalView
}

// SettingsView is the settings page.
type SettingsView struct {
	BaseView
	PublicAddress      string
	AppPort            int
	WebPort            int
	ApproveNewDevices  bool
	ForgetDays         int
	DataDir            string
	LogPath            string
	Version            string
	AddressSuggestions []string
	Activity           []AuditEntry
}

// NewGroupView builds a view that only carries host-visible fields.
func NewGroupView(g group.Group, presence map[string]session.Presence, live bool, connected int, endpoint string) GroupView {
	if connected == 0 && presence != nil {
		for _, p := range presence {
			if p.Live {
				connected++
			}
		}
	}
	return GroupView{
		ID:          g.ID,
		Name:        g.Name,
		HostName:    g.HostName,
		Live:        live && g.IsHost(),
		Connected:   connected,
		Members:     len(g.Members),
		Services:    len(g.Services),
		OpenInvites: openInvites(g.Invites, time.Now()),
		Endpoint:    endpoint,
		CreatedAt:   g.CreatedAt,
	}
}

// NewMemberView builds a view without friend-only connection fields.
func NewMemberView(m group.Member, presence map[string]session.Presence) MemberView {
	p := presence[m.ID]
	return MemberView{
		ID:          m.ID,
		Name:        m.Name,
		DeviceName:  m.DeviceName,
		Address:     m.LastAddress,
		Fingerprint: m.Fingerprint,
		FirstSeen:   m.FirstSeen,
		LastSeen:    m.LastSeen,
		Live:        p.Live,
		Approved:    m.IsApproved(),
	}
}

// NewInviteView builds an invite view. Code is empty unless supplied and the
// invite is still open; redeemed and expired invites never show their secret.
func NewInviteView(inv group.Invite, code string, now time.Time) InviteView {
	open := inv.Open(now)
	if !open {
		code = ""
	}
	return InviteView{
		ID:        inv.ID,
		Name:      inv.Name,
		CreatedAt: inv.CreatedAt,
		ExpiresAt: inv.ExpiresAt,
		Open:      open,
		Redeemed:  !inv.RedeemedAt.IsZero(),
		Code:      code,
		Expired:   !inv.ExpiresAt.IsZero() && now.After(inv.ExpiresAt),
	}
}

// NewServiceView builds a view of a service.
func NewServiceView(s group.Service) ServiceView {
	ports := make([]string, len(s.Ports))
	for i, p := range s.Ports {
		ports[i] = fmt.Sprint(p)
	}
	return ServiceView{
		ID:        s.ID,
		Name:      s.Name,
		Proto:     s.Proto,
		Target:    s.Target,
		Ports:     s.Ports,
		PortsText: strings.Join(ports, ", "),
		Hint:      s.Hint,
	}
}

// NewApprovalView builds a view of a pending approval.
func NewApprovalView(a *app.Approval) *ApprovalView {
	if a == nil {
		return nil
	}
	return &ApprovalView{
		ID:          a.ID,
		GroupID:     a.GroupID,
		PeerName:    a.PeerName,
		DeviceName:  a.DeviceName,
		Address:     a.Address,
		Fingerprint: a.Fingerprint,
	}
}

func openInvites(invites []group.Invite, now time.Time) int {
	n := 0
	for _, inv := range invites {
		if inv.Open(now) {
			n++
		}
	}
	return n
}
