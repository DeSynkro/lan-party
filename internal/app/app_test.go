package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/tunnel"
	"github.com/zalando/go-keyring"
)

type memKeyring map[string]string

func (m memKeyring) slot(service, user string) string { return service + "\x00" + user }

func (m memKeyring) Get(service, user string) (string, error) {
	value, ok := m[m.slot(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (m memKeyring) Set(service, user, secret string) error {
	m[m.slot(service, user)] = secret
	return nil
}

func (m memKeyring) Delete(service, user string) error {
	delete(m, m.slot(service, user))
	return nil
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	paths := config.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "config"),
		DataDir:   filepath.Join(t.TempDir(), "data"),
	}
	a, err := NewWithKeyring(paths, memKeyring{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func newTestIdentity(t *testing.T, name string) secure.Identity {
	t.Helper()
	identity, err := secure.NewStore(memKeyring{}).LoadOrCreate(name)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true in time")
}

func createHostWithInvite(t *testing.T, a *App) (group.Group, invite.Invite, string) {
	t.Helper()
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := invite.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	return g, inv, code
}

func authRequest(inv invite.Invite, identity secure.Identity, token string) tunnel.AuthRequest {
	return tunnel.AuthRequest{
		GroupID:     inv.GroupID,
		PeerName:    inv.PeerName,
		DeviceName:  "ash-desktop",
		Token:       token,
		RemoteAddr:  "203.0.113.5:5555",
		PublicKey:   identity.Public,
		Fingerprint: secure.Fingerprint(identity.Public),
	}
}

func TestCreateGroupAndInvite(t *testing.T) {
	a := newTestApp(t)
	g, inv, code := createHostWithInvite(t, a)

	if g.Role != group.RoleHost {
		t.Fatalf("role = %q", g.Role)
	}
	if inv.GroupID != g.ID || inv.PeerName != "Ash" {
		t.Fatalf("invite = %+v", inv)
	}
	if inv.Endpoint != "home.example.net:51820" {
		t.Fatalf("endpoint = %q", inv.Endpoint)
	}
	if inv.HostPin == "" || inv.Token == "" || !strings.HasPrefix(code, "lp1_") {
		t.Fatal("invite is missing the pin or token")
	}

	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	if a.InviteCode(saved.Invites[0].ID) != code {
		t.Fatal("the in-memory invite code was not kept")
	}

	data, err := os.ReadFile(filepath.Join(a.paths.DataDir, "groups.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), inv.Token) {
		t.Fatal("the invite token was written to disk")
	}
	if strings.Contains(string(data), "privateKey") {
		t.Fatal("a private key field was written to disk")
	}
}

func TestInviteUsesOverridePort(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net:40000"
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := invite.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Endpoint != "home.example.net:40000" {
		t.Fatalf("endpoint = %q", inv.Endpoint)
	}
}

func TestCreateInviteNeedsAddress(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateInvite(g.ID, "Ash"); err == nil {
		t.Fatal("an invite was created without a public address")
	}
}

func TestEnrollWithoutApproval(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)
	s := a.Settings()
	s.ApproveNewDevices = false
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	device := newTestIdentity(t, "device-a")
	decision, err := a.authenticate(context.Background(), g.ID, authRequest(inv, device, inv.Token))
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allow || decision.MemberID == "" {
		t.Fatalf("decision = %+v", decision)
	}

	saved, _ := a.Group(g.ID)
	member, ok := saved.FindMemberByKey(secure.EncodePublicKey(device.Public))
	if !ok {
		t.Fatal("the device was not added")
	}
	if member.State != group.MemberApproved || member.Name != "Ash" {
		t.Fatalf("member = %+v", member)
	}
	inviteRecord, _ := saved.FindInvite(saved.Invites[0].ID)
	if inviteRecord.RedeemedAt.IsZero() {
		t.Fatal("the invite was not redeemed")
	}

	other := newTestIdentity(t, "device-b")
	second, err := a.authenticate(context.Background(), g.ID, authRequest(inv, other, inv.Token))
	if err != nil {
		t.Fatal(err)
	}
	if second.Allow {
		t.Fatal("a burned invite was accepted for another device")
	}
}

func TestEnrollRequiresApproval(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)
	device := newTestIdentity(t, "device-a")

	type result struct {
		decision tunnel.AuthDecision
		err      error
	}
	done := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		decision, err := a.authenticate(ctx, g.ID, authRequest(inv, device, inv.Token))
		done <- result{decision, err}
	}()

	var approval *Approval
	waitFor(t, 2*time.Second, func() bool {
		approval = a.Approval()
		return approval != nil
	})
	if approval.Kind != "join" || approval.PeerName != "Ash" {
		t.Fatalf("approval = %+v", approval)
	}
	if !a.ResolveApproval(approval.ID, true) {
		t.Fatal("the approval could not be resolved")
	}

	r := <-done
	if r.err != nil || !r.decision.Allow {
		t.Fatalf("decision = %+v, err = %v", r.decision, r.err)
	}
	saved, _ := a.Group(g.ID)
	if len(saved.Members) != 1 {
		t.Fatalf("members = %+v", saved.Members)
	}
}

func TestEnrollDenied(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)
	device := newTestIdentity(t, "device-a")

	done := make(chan tunnel.AuthDecision, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		decision, _ := a.authenticate(ctx, g.ID, authRequest(inv, device, inv.Token))
		done <- decision
	}()

	var approval *Approval
	waitFor(t, 2*time.Second, func() bool {
		approval = a.Approval()
		return approval != nil
	})
	if !a.ResolveApproval(approval.ID, false) {
		t.Fatal("the approval could not be resolved")
	}

	decision := <-done
	if decision.Allow {
		t.Fatal("a denied device was allowed")
	}
	saved, _ := a.Group(g.ID)
	if len(saved.Members) != 0 {
		t.Fatalf("members = %+v", saved.Members)
	}
	if saved.Invites[0].RedeemedAt != (time.Time{}) {
		t.Fatal("the invite was burned on a denial")
	}
}

func TestKnownMemberNeedsNoApproval(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)
	s := a.Settings()
	s.ApproveNewDevices = false
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	device := newTestIdentity(t, "device-a")
	first, _ := a.authenticate(context.Background(), g.ID, authRequest(inv, device, inv.Token))
	if !first.Allow {
		t.Fatal("the first join was denied")
	}

	again, err := a.authenticate(context.Background(), g.ID, tunnel.AuthRequest{
		GroupID:     g.ID,
		DeviceName:  "ash-desktop",
		RemoteAddr:  "203.0.113.6:5555",
		PublicKey:   device.Public,
		Fingerprint: secure.Fingerprint(device.Public),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Allow || again.MemberID != first.MemberID {
		t.Fatalf("decision = %+v", again)
	}
	if a.Approval() != nil {
		t.Fatal("a known device asked for approval")
	}
}

func TestNewDeviceNeedsApproval(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)
	s := a.Settings()
	s.ApproveNewDevices = false
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	oldDevice := newTestIdentity(t, "device-old")
	first, _ := a.authenticate(context.Background(), g.ID, authRequest(inv, oldDevice, inv.Token))
	if !first.Allow {
		t.Fatal("the first join was denied")
	}

	newDevice := newTestIdentity(t, "device-new")
	done := make(chan tunnel.AuthDecision, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		decision, _ := a.authenticate(ctx, g.ID, tunnel.AuthRequest{
			GroupID:     g.ID,
			MemberID:    first.MemberID,
			DeviceName:  "new-desktop",
			RemoteAddr:  "203.0.113.7:5555",
			PublicKey:   newDevice.Public,
			Fingerprint: secure.Fingerprint(newDevice.Public),
		})
		done <- decision
	}()

	var approval *Approval
	waitFor(t, 2*time.Second, func() bool {
		approval = a.Approval()
		return approval != nil
	})
	if approval.Kind != "new device" || approval.PeerName != "Ash" {
		t.Fatalf("approval = %+v", approval)
	}
	a.ResolveApproval(approval.ID, true)
	if decision := <-done; !decision.Allow {
		t.Fatal("the new device was denied")
	}

	saved, _ := a.Group(g.ID)
	member, _ := saved.FindMember(first.MemberID)
	if member.PublicKey != secure.EncodePublicKey(newDevice.Public) {
		t.Fatal("the member key was not replaced")
	}
	if _, ok := saved.FindMemberByKey(secure.EncodePublicKey(oldDevice.Public)); ok {
		t.Fatal("the old device key is still a member")
	}
}

func TestExpiredInviteDenied(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)

	a.mu.Lock()
	idx := a.groupIndexLocked(g.ID)
	a.groups[idx].Invites[0].ExpiresAt = time.Now().Add(-time.Hour)
	if err := a.saveLocked(); err != nil {
		a.mu.Unlock()
		t.Fatal(err)
	}
	a.mu.Unlock()

	device := newTestIdentity(t, "device-a")
	decision, err := a.authenticate(context.Background(), g.ID, authRequest(inv, device, inv.Token))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allow {
		t.Fatal("an expired invite was accepted")
	}
}

func TestRateLimit(t *testing.T) {
	a := newTestApp(t)
	g, inv, _ := createHostWithInvite(t, a)
	device := newTestIdentity(t, "device-a")

	for i := 0; i < attemptLimit; i++ {
		req := authRequest(inv, device, "LP1-NOT-A-REAL-TOKEN")
		decision, _ := a.authenticate(context.Background(), g.ID, req)
		if decision.Allow {
			t.Fatal("a bad token was accepted")
		}
	}
	req := authRequest(inv, device, "LP1-NOT-A-REAL-TOKEN")
	decision, _ := a.authenticate(context.Background(), g.ID, req)
	if decision.Allow {
		t.Fatal("the rate limit did not stop the attempt")
	}
	if !strings.Contains(decision.Reason, "too many") {
		t.Fatalf("reason = %q", decision.Reason)
	}
}

func TestApprovalQueue(t *testing.T) {
	a := newTestApp(t)
	g, inv1, _ := createHostWithInvite(t, a)
	code2, err := a.CreateInvite(g.ID, "Pip")
	if err != nil {
		t.Fatal(err)
	}
	inv2, err := invite.Decode(code2)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make(chan tunnel.AuthDecision, 2)
	for _, pair := range []struct {
		inv  invite.Invite
		name string
	}{
		{inv1, "device-a"},
		{inv2, "device-b"},
	} {
		wg.Add(1)
		go func(inv invite.Invite, name string) {
			defer wg.Done()
			device := newTestIdentity(t, name)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			decision, _ := a.authenticate(ctx, g.ID, authRequest(inv, device, inv.Token))
			results <- decision
		}(pair.inv, pair.name)
	}

	for i := 0; i < 2; i++ {
		var approval *Approval
		waitFor(t, 2*time.Second, func() bool {
			approval = a.Approval()
			return approval != nil
		})
		if !a.ResolveApproval(approval.ID, true) {
			t.Fatal("the approval could not be resolved")
		}
	}
	wg.Wait()
	close(results)
	for decision := range results {
		if !decision.Allow {
			t.Fatalf("decision = %+v", decision)
		}
	}
}

func TestDeleteGroup(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := a.keys.Load("host/" + g.ID); err != nil || !ok {
		t.Fatal("the host key was not created")
	}
	if err := a.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Group(g.ID); ok {
		t.Fatal("the group was not removed")
	}
	if _, ok, err := a.keys.Load("host/" + g.ID); err != nil || ok {
		t.Fatal("the host key was not removed")
	}
}

func TestPublicEndpoint(t *testing.T) {
	cases := []struct {
		public string
		want   string
	}{
		{"home.example.net", "home.example.net:51820"},
		{"home.example.net:40000", "home.example.net:40000"},
		{"::1", "[::1]:51820"},
		{"[::1]:40000", "[::1]:40000"},
	}
	for _, tc := range cases {
		got, err := publicEndpoint(tc.public, 51820)
		if err != nil {
			t.Errorf("%q: %v", tc.public, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q = %q, want %q", tc.public, got, tc.want)
		}
	}
	for _, public := range []string{"", "http://home", "home example"} {
		if _, err := publicEndpoint(public, 51820); err == nil {
			t.Errorf("%q was accepted", public)
		}
	}
}
