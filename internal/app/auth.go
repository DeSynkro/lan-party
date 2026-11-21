package app

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/session"
	"github.com/desynkro/lan-party/internal/tunnel"
)

const (
	attemptWindow       = time.Minute
	attemptLimit        = 5
	maxAttemptIPs       = 1024
	approvalLimit       = 2 * time.Minute
	maxPendingApprovals = 32
)

var errTooManyApprovals = errors.New("too many pending approvals")

// Approval is one device waiting for the host's answer.
type Approval struct {
	ID          string
	GroupID     string
	Kind        string
	PeerName    string
	DeviceName  string
	Address     string
	Fingerprint string
	Expires     time.Time
}

type pendingApproval struct {
	Approval
	ch chan bool
}

type hostAuth struct {
	app     *App
	groupID string
}

func (h hostAuth) Authenticate(ctx context.Context, req tunnel.AuthRequest) (tunnel.AuthDecision, error) {
	return h.app.authenticate(ctx, h.groupID, req)
}

func (a *App) authenticate(ctx context.Context, groupID string, req tunnel.AuthRequest) (tunnel.AuthDecision, error) {
	if req.GroupID != groupID {
		return deny("this device is asking for a different group"), nil
	}

	ip := hostOnly(req.RemoteAddr)
	limiterKey := ip
	displayAddr := ip
	if req.Relay {
		limiterKey = req.RemoteAddr
		displayAddr = "via relay"
	}
	now := time.Now()
	if !a.limiter.Allow(limiterKey, now) {
		return deny("too many attempts; wait a minute and try again"), nil
	}

	key := secure.EncodePublicKey(req.PublicKey)

	a.mu.Lock()
	g, ok := a.groupLocked(groupID)
	settings := a.settings
	a.mu.Unlock()
	if !ok {
		return deny("the group is no longer here"), nil
	}

	if member, ok := g.FindMemberByKey(key); ok && member.IsApproved() {
		a.noteConnection(groupID, member.ID, displayAddr)
		return a.reconnectDecision(groupID, member.ID, g.Services, req.HasRelayToken), nil
	}

	if req.MemberID != "" {
		member, ok := g.FindMember(req.MemberID)
		if !ok {
			a.limiter.Fail(limiterKey, now)
			return deny("this device is not in the group"), nil
		}

		allowed, err := a.requestApproval(ctx, Approval{
			GroupID:     groupID,
			Kind:        "new device",
			PeerName:    member.Name,
			DeviceName:  req.DeviceName,
			Address:     displayAddr,
			Fingerprint: req.Fingerprint,
		})
		if err != nil {
			if errors.Is(err, errTooManyApprovals) {
				return deny("too many devices are waiting for approval; try again later"), nil
			}
			return deny("the host did not answer in time"), nil
		}
		if !allowed {
			a.limiter.Fail(limiterKey, now)
			return deny("the host denied this device"), nil
		}
		if err := a.replaceDevice(groupID, member.ID, req, displayAddr); err != nil {
			return deny("the device could not be saved"), nil
		}
		if host := a.currentHost(); host != nil {
			_ = host.Revoke(member.ID)
		}
		current, _ := a.groupCopy(groupID)
		return a.reconnectDecision(groupID, member.ID, current.Services, req.HasRelayToken), nil
	}

	if req.Token == "" {
		a.limiter.Fail(limiterKey, now)
		return deny("this device is not in the group"), nil
	}

	hash := invite.HashToken(req.Token)
	var matched *group.Invite
	for i := range g.Invites {
		inv := &g.Invites[i]
		if !inv.Open(now) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(inv.TokenHash), []byte(hash)) == 1 {
			matched = inv
			break
		}
	}
	if matched == nil {
		a.limiter.Fail(limiterKey, now)
		return deny("that invite is not valid"), nil
	}

	if settings.ApproveNewDevices {
		allowed, err := a.requestApproval(ctx, Approval{
			GroupID:     groupID,
			Kind:        "join",
			PeerName:    matched.Name,
			DeviceName:  req.DeviceName,
			Address:     displayAddr,
			Fingerprint: req.Fingerprint,
		})
		if err != nil {
			if errors.Is(err, errTooManyApprovals) {
				return deny("too many devices are waiting for approval; try again later"), nil
			}
			return deny("the host did not answer in time"), nil
		}
		if !allowed {
			a.limiter.Fail(limiterKey, now)
			return deny("the host denied the join"), nil
		}
	}

	memberID, replaced, err := a.enroll(groupID, matched.ID, req, displayAddr)
	if err != nil {
		return deny("the member could not be saved"), nil
	}
	if replaced {
		if host := a.currentHost(); host != nil {
			_ = host.Revoke(memberID)
		}
	}
	current, _ := a.groupCopy(groupID)
	return a.reconnectDecision(groupID, memberID, current.Services, req.HasRelayToken), nil
}

func (a *App) noteConnection(groupID, memberID, ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		return
	}
	for i := range a.groups[idx].Members {
		if a.groups[idx].Members[i].ID != memberID {
			continue
		}
		a.groups[idx].Members[i].LastAddress = ip
		a.groups[idx].Members[i].LastSeen = time.Now().UTC()
		_ = a.saveLocked()
		return
	}
}

func (a *App) replaceDevice(groupID, memberID string, req tunnel.AuthRequest, ip string) error {
	key := secure.EncodePublicKey(req.PublicKey)

	a.mu.Lock()
	defer a.mu.Unlock()

	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		return errors.New("group not found")
	}
	if other, ok := a.groups[idx].FindMemberByKey(key); ok && other.ID != memberID {
		return errors.New("that device is already a member")
	}
	for i := range a.groups[idx].Members {
		if a.groups[idx].Members[i].ID != memberID {
			continue
		}
		a.groups[idx].Members[i].PublicKey = key
		a.groups[idx].Members[i].DeviceName = req.DeviceName
		a.groups[idx].Members[i].Fingerprint = req.Fingerprint
		a.groups[idx].Members[i].LastAddress = ip
		a.groups[idx].Members[i].LastSeen = time.Now().UTC()
		a.groups[idx].Members[i].State = group.MemberApproved
		return a.saveLocked()
	}
	return errors.New("member not found")
}

func (a *App) enroll(groupID, inviteID string, req tunnel.AuthRequest, ip string) (string, bool, error) {
	key := secure.EncodePublicKey(req.PublicKey)

	a.mu.Lock()
	defer a.mu.Unlock()

	idx := a.groupIndexLocked(groupID)
	if idx < 0 {
		return "", false, errors.New("group not found")
	}
	g := &a.groups[idx]
	inv, ok := g.FindInvite(inviteID)
	if !ok || !inv.Open(time.Now()) {
		return "", false, errors.New("invite is no longer open")
	}

	if member, ok := g.FindMemberByKey(key); ok {
		_ = g.RedeemInvite(inviteID, time.Now())
		_ = a.saveLocked()
		return member.ID, false, nil
	}

	name := inv.Name
	now := time.Now().UTC()
	if member, ok := g.FindMemberByName(name); ok {
		for i := range g.Members {
			if g.Members[i].ID != member.ID {
				continue
			}
			g.Members[i].PublicKey = key
			g.Members[i].DeviceName = req.DeviceName
			g.Members[i].Fingerprint = req.Fingerprint
			g.Members[i].LastAddress = ip
			g.Members[i].LastSeen = now
			g.Members[i].State = group.MemberApproved
			break
		}
		_ = g.RedeemInvite(inviteID, now)
		if err := a.saveLocked(); err != nil {
			return "", true, err
		}
		return member.ID, true, nil
	}

	member := group.Member{
		ID:          group.NewID(),
		Name:        name,
		DeviceName:  req.DeviceName,
		PublicKey:   key,
		Fingerprint: req.Fingerprint,
		LastAddress: ip,
		FirstSeen:   now,
		LastSeen:    now,
		State:       group.MemberApproved,
	}
	if err := g.AddMember(member); err != nil {
		return "", false, err
	}
	_ = g.RedeemInvite(inviteID, now)
	if err := a.saveLocked(); err != nil {
		return "", false, err
	}
	return member.ID, false, nil
}

func (a *App) requestApproval(ctx context.Context, approval Approval) (bool, error) {
	approval.ID = group.NewID()
	approval.Expires = time.Now().Add(approvalLimit)
	pending := &pendingApproval{Approval: approval, ch: make(chan bool, 1)}

	a.mu.Lock()
	if len(a.pending) >= maxPendingApprovals {
		a.mu.Unlock()
		return false, errTooManyApprovals
	}
	a.pending = append(a.pending, pending)
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		for i, p := range a.pending {
			if p == pending {
				a.pending = append(a.pending[:i], a.pending[i+1:]...)
				break
			}
		}
		a.mu.Unlock()
	}()

	select {
	case allowed := <-pending.ch:
		return allowed, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// Approval returns the first device waiting for an answer.
func (a *App) Approval() *Approval {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.pending) == 0 {
		return nil
	}
	approval := a.pending[0].Approval
	return &approval
}

// ResolveApproval answers a pending device request.
func (a *App) ResolveApproval(id string, allow bool) bool {
	a.mu.Lock()
	for i, pending := range a.pending {
		if pending.ID != id {
			continue
		}
		a.pending = append(a.pending[:i], a.pending[i+1:]...)
		a.mu.Unlock()
		select {
		case pending.ch <- allow:
		default:
		}
		return true
	}
	a.mu.Unlock()
	return false
}

func (a *App) groupLocked(id string) (group.Group, bool) {
	idx := a.groupIndexLocked(id)
	if idx < 0 {
		return group.Group{}, false
	}
	return cloneGroup(a.groups[idx]), true
}

func (a *App) groupCopy(id string) (group.Group, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.groupLocked(id)
}

func (a *App) currentHost() *session.Host {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.host
}

func allow(memberID string, services []group.Service) tunnel.AuthDecision {
	return tunnel.AuthDecision{
		Allow:    true,
		MemberID: memberID,
		Services: session.ServiceInfos(services),
	}
}

func deny(reason string) tunnel.AuthDecision {
	return tunnel.AuthDecision{Allow: false, Reason: reason}
}

func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	return addr
}

type attemptLimiter struct {
	mu   sync.Mutex
	byIP map[string][]time.Time
}

func newAttemptLimiter() *attemptLimiter {
	return &attemptLimiter{byIP: map[string][]time.Time{}}
}

func (l *attemptLimiter) Allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(ip, now)
	return len(l.byIP[ip]) < attemptLimit
}

func (l *attemptLimiter) Fail(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(ip, now)

	if _, exists := l.byIP[ip]; !exists && len(l.byIP) >= maxAttemptIPs {
		l.pruneAllLocked(now)
		if len(l.byIP) >= maxAttemptIPs {
			for victim := range l.byIP {
				delete(l.byIP, victim)
				break
			}
		}
	}
	l.byIP[ip] = append(l.byIP[ip], now)
}

func (l *attemptLimiter) pruneAllLocked(now time.Time) {
	cutoff := now.Add(-attemptWindow)
	for ip, times := range l.byIP {
		kept := times[:0]
		for _, at := range times {
			if at.After(cutoff) {
				kept = append(kept, at)
			}
		}
		if len(kept) == 0 {
			delete(l.byIP, ip)
		} else {
			l.byIP[ip] = kept
		}
	}
}

func (l *attemptLimiter) pruneLocked(ip string, now time.Time) {
	cutoff := now.Add(-attemptWindow)
	kept := l.byIP[ip][:0]
	for _, at := range l.byIP[ip] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(l.byIP, ip)
		return
	}
	l.byIP[ip] = kept
}
