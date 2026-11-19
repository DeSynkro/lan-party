package app

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/tunnel"
)

type memStore map[string]string

func (m memStore) slot(service, user string) string { return service + "\x00" + user }

func (m memStore) Get(service, user string) (string, error) {
	value, ok := m[m.slot(service, user)]
	if !ok {
		return "", fs.ErrNotExist
	}
	return value, nil
}

func (m memStore) Set(service, user, secret string) error {
	m[m.slot(service, user)] = secret
	return nil
}

func (m memStore) Delete(service, user string) error {
	delete(m, m.slot(service, user))
	return nil
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	paths := config.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "config"),
		DataDir:   filepath.Join(t.TempDir(), "data"),
	}
	a, err := NewWithStore(paths, memStore{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func newTestIdentity(t *testing.T, name string) secure.Identity {
	t.Helper()
	identity, err := secure.NewStore(memStore{}).LoadOrCreate(name)
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

func TestNewStoresKeysAsOwnerOnlyFiles(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(root, "config"),
		DataDir:   filepath.Join(root, "data"),
	}
	a, err := New(paths)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	if _, err := a.CreateGroup("Friday Night", "Desynkro"); err != nil {
		t.Fatal(err)
	}

	keyDir := filepath.Join(paths.DataDir, "keys")
	entries, err := os.ReadDir(keyDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("wrote %d key files, want 1", len(entries))
	}
	if runtime.GOOS != "windows" {
		info, err := entries[0].Info()
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("key file mode is %o, want 600", perm)
		}
		dirInfo, err := os.Stat(keyDir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Fatalf("key directory mode is %o, want 700", perm)
		}
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
		got, err := Endpoint(tc.public, 51820)
		if err != nil {
			t.Errorf("%q: %v", tc.public, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q = %q, want %q", tc.public, got, tc.want)
		}
	}
	for _, public := range []string{"", "http://home", "home example"} {
		if _, err := Endpoint(public, 51820); err == nil {
			t.Errorf("%q was accepted", public)
		}
	}
}

func TestCreateInvitesAreUniqueForSameName(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	first, err := a.CreateInvite(g.ID, "desynkro-work")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.CreateInvite(g.ID, "desynkro-work")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two invites for the same name produced the same code")
	}
	firstInvite, err := invite.Decode(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInvite, err := invite.Decode(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstInvite.Token == secondInvite.Token {
		t.Fatal("two invites for the same name produced the same token")
	}
}

func TestEffectiveConnectionFallsBackToSettings(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicAddress != "home.example.net" || got.AppPort != 51820 || got.Custom {
		t.Fatalf("connection = %+v", got)
	}
}

func TestEffectiveConnectionUsesGroupOverrides(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := a.SetGroupConnection(g.ID, "group.example.net:40000", 41000); err != nil {
		t.Fatal(err)
	}
	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicAddress != "group.example.net:40000" || got.AppPort != 41000 || !got.Custom {
		t.Fatalf("connection = %+v", got)
	}
	endpoint, err := a.GroupEndpoint(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "group.example.net:40000" {
		t.Fatalf("endpoint = %q", endpoint)
	}
	port, err := a.GroupAppPort(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if port != 41000 {
		t.Fatalf("app port = %d", port)
	}

	if err := a.SetGroupConnection(g.ID, "", 0); err != nil {
		t.Fatal(err)
	}
	got, err = a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicAddress != "home.example.net" || got.AppPort != 51820 || got.Custom {
		t.Fatalf("connection after clear = %+v", got)
	}
}

func TestSetGroupConnectionValidates(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}

	if err := a.SetGroupConnection(g.ID, "https://example.net", 51820); err == nil {
		t.Fatal("a URL was accepted as a public address")
	}
	if err := a.SetGroupConnection(g.ID, "home.example.net", 70000); err == nil {
		t.Fatal("an invalid app port was accepted")
	}
	if err := a.SetGroupConnection("missing", "home.example.net", 51820); err == nil {
		t.Fatal("a missing group was accepted")
	}

	friend := group.CreateFriendGroup("Friday Night", "Desynkro", "home.example.net:51820", "pin", "self", "Ash", nil)
	a.groups = append(a.groups, friend)
	if err := a.SetGroupConnection(friend.ID, "home.example.net", 51820); err == nil {
		t.Fatal("a friend group was changed")
	}

	if err := a.SetGroupConnection(g.ID, "group.example.net", 41000); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnection(g.ID, "", 0); err != nil {
		t.Fatal(err)
	}
}

func differentUDPPort(t *testing.T, avoid ...int) int {
	t.Helper()
	for i := 0; i < 20; i++ {
		port := freeUDPPort(t)
		ok := true
		for _, used := range avoid {
			if port == used {
				ok = false
				break
			}
		}
		if ok {
			return port
		}
	}
	t.Fatal("could not find a different free UDP port")
	return 0
}

func TestCreateInviteUsesGroupConnection(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnection(g.ID, "group.example.net:40000", 41000); err != nil {
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
	if inv.Endpoint != "group.example.net:40000" {
		t.Fatalf("endpoint = %q", inv.Endpoint)
	}
}

func TestExistingInviteCodeKeepsEndpointSnapshot(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inviteID := saved.Invites[0].ID

	if err := a.SetGroupConnection(g.ID, "group.example.net:40000", 41000); err != nil {
		t.Fatal(err)
	}
	if got := a.InviteCode(inviteID); got != "" {
		t.Fatalf("stale invite code is still available: %q", got)
	}
	after, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if len(after.OpenInvites(time.Now())) != 0 {
		t.Fatal("an invite survived an endpoint change")
	}
	oldInvite, err := invite.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if oldInvite.Endpoint != "home.example.net:51820" {
		t.Fatalf("old endpoint = %q", oldInvite.Endpoint)
	}

	newCode, err := a.CreateInvite(g.ID, "Blair")
	if err != nil {
		t.Fatal(err)
	}
	newInvite, err := invite.Decode(newCode)
	if err != nil {
		t.Fatal(err)
	}
	if newInvite.Endpoint != "group.example.net:40000" {
		t.Fatalf("new endpoint = %q", newInvite.Endpoint)
	}
}

func TestStartSessionUsesGroupPort(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}

	globalHeld, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer globalHeld.Close()
	globalPort := globalHeld.LocalAddr().(*net.UDPAddr).Port
	groupPort := differentUDPPort(t, globalPort)

	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = globalPort
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnection(g.ID, "", groupPort); err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatalf("session did not use the group port: %v", err)
	}
	if a.LiveGroupID() != g.ID {
		t.Fatalf("live group = %q", a.LiveGroupID())
	}
}

func TestSetGroupConnectionLiveRules(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = freeUDPPort(t)
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	groupPort := differentUDPPort(t, s.AppPort)
	if err := a.SetGroupConnection(g.ID, "", groupPort); err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	defer a.StopSession()

	otherPort := differentUDPPort(t, groupPort, s.AppPort)
	if err := a.SetGroupConnection(g.ID, "", otherPort); err == nil {
		t.Fatal("a live app-port change was accepted")
	}
	if err := a.SetGroupConnection(g.ID, "new.example.net", groupPort); err != nil {
		t.Fatalf("an address-only change was blocked: %v", err)
	}
	if err := a.SetGroupConnection(g.ID, "", groupPort); err != nil {
		t.Fatalf("the same effective port was blocked: %v", err)
	}

	if err := a.StopSession(); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnection(g.ID, "", otherPort); err != nil {
		t.Fatalf("a port change after stopping was blocked: %v", err)
	}
}

func TestSaveSettingsPortRules(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	globalPort := freeUDPPort(t)
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = globalPort
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}

	nextGlobal := differentUDPPort(t, globalPort)
	s = a.Settings()
	s.AppPort = nextGlobal
	if err := a.SaveSettings(s); err == nil {
		t.Fatal("the global port changed while the live group inherited it")
	}
	if err := a.StopSession(); err != nil {
		t.Fatal(err)
	}

	groupPort := differentUDPPort(t, nextGlobal)
	if err := a.SetGroupConnection(g.ID, "", groupPort); err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	defer a.StopSession()

	after := differentUDPPort(t, groupPort, nextGlobal)
	s = a.Settings()
	s.AppPort = after
	if err := a.SaveSettings(s); err != nil {
		t.Fatalf("the global port change was blocked by a group override: %v", err)
	}
}

func TestCreateInviteExpiryIsOneHour(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateInvite(g.ID, "Ash"); err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inv := saved.Invites[0]
	if got := inv.ExpiresAt.Sub(inv.CreatedAt); got < 59*time.Minute || got > 61*time.Minute {
		t.Fatalf("invite lifetime = %v", got)
	}
}

func TestEffectiveConnectionOneSidedOverrides(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	if err := a.SetGroupConnection(g.ID, "group.example.net", 0); err != nil {
		t.Fatal(err)
	}
	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicAddress != "group.example.net" || got.AppPort != 51820 || !got.Custom {
		t.Fatalf("address-only connection = %+v", got)
	}

	if err := a.SetGroupConnection(g.ID, "", 41000); err != nil {
		t.Fatal(err)
	}
	got, err = a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicAddress != "home.example.net" || got.AppPort != 41000 || !got.Custom {
		t.Fatalf("port-only connection = %+v", got)
	}
}

func TestSetGroupConnectionRestoresOnSaveFailure(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnection(g.ID, "group.example.net", 41000); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inviteID := saved.Invites[0].ID

	if err := os.RemoveAll(a.DataDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.DataDir(), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := a.SetGroupConnection(g.ID, "other.example.net", 42000); err == nil {
		t.Fatal("a failed save was not reported")
	}
	saved, ok = a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if saved.PublicAddress != "group.example.net" || saved.AppPort != 41000 {
		t.Fatalf("in-memory group changed after a failed save: %+v", saved)
	}
	if len(saved.OpenInvites(time.Now())) != 1 {
		t.Fatal("the open invite was lost after a failed save")
	}
	if got := a.InviteCode(inviteID); got != code {
		t.Fatalf("invite code = %q, want %q", got, code)
	}
}

func TestSetGroupConnectionCancelsInvitesWhenEndpointChanges(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inviteID := saved.Invites[0].ID

	if err := a.SetGroupConnection(g.ID, "", 41000); err != nil {
		t.Fatal(err)
	}
	after, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if len(after.OpenInvites(time.Now())) != 0 {
		t.Fatal("an invite survived an endpoint change")
	}
	if got := a.InviteCode(inviteID); got != "" {
		t.Fatalf("stale invite code is still available: %q", got)
	}
	if code == "" {
		t.Fatal("the original invite was empty")
	}
}

func TestSetGroupConnectionKeepsInvitesWhenEndpointUnchanged(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net:40000"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inviteID := saved.Invites[0].ID

	if err := a.SetGroupConnection(g.ID, "", 41000); err != nil {
		t.Fatal(err)
	}
	after, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if len(after.OpenInvites(time.Now())) != 1 {
		t.Fatal("the invite was cancelled even though the advertised endpoint stayed the same")
	}
	if got := a.InviteCode(inviteID); got != code {
		t.Fatalf("invite code = %q, want %q", got, code)
	}
}

func TestSaveSettingsCancelsInheritedInvites(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inviteID := saved.Invites[0].ID

	s = a.Settings()
	s.AppPort = 41000
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	after, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if len(after.OpenInvites(time.Now())) != 0 {
		t.Fatal("an invite survived a default endpoint change")
	}
	if got := a.InviteCode(inviteID); got != "" {
		t.Fatalf("stale invite code is still available: %q", got)
	}
	if code == "" {
		t.Fatal("the original invite was empty")
	}
}

func TestSaveSettingsKeepsInvitesForFixedOutsidePort(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.PublicAddress = "home.example.net"
	s.AppPort = 51820
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnection(g.ID, "fixed.example.net:40000", 0); err != nil {
		t.Fatal(err)
	}
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.Invites) != 1 {
		t.Fatalf("saved group = %+v", saved)
	}
	inviteID := saved.Invites[0].ID

	s = a.Settings()
	s.AppPort = 41000
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	after, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if len(after.OpenInvites(time.Now())) != 1 {
		t.Fatal("the invite was cancelled even though the advertised endpoint stayed the same")
	}
	if got := a.InviteCode(inviteID); got != code {
		t.Fatalf("invite code = %q, want %q", got, code)
	}
}

func TestUpdateService(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	original := group.Service{ID: "mc", Name: "Minecraft", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25565}}
	if err := a.AddService(g.ID, original); err != nil {
		t.Fatal(err)
	}
	updated := original
	updated.Name = "Minecraft Modded"
	updated.Ports = []int{25566}
	if err := a.UpdateService(g.ID, updated); err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	got, ok := saved.FindService("mc")
	if !ok || got.Name != "Minecraft Modded" || len(got.Ports) != 1 || got.Ports[0] != 25566 {
		t.Fatalf("updated service = %+v", got)
	}
}

func TestUpdateServiceRestoresOnSaveFailure(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	original := group.Service{ID: "mc", Name: "Minecraft", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25565}}
	if err := a.AddService(g.ID, original); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(a.DataDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.DataDir(), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	updated := original
	updated.Name = "Changed"
	updated.Ports = []int{25566}
	if err := a.UpdateService(g.ID, updated); err == nil {
		t.Fatal("a failed save was not reported")
	}
	saved, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	got, ok := saved.FindService("mc")
	if !ok || got.Name != "Minecraft" || len(got.Ports) != 1 || got.Ports[0] != 25565 {
		t.Fatalf("a failed save changed the service: %+v", got)
	}
}

func TestServiceChangesOnlyUpdateTheLiveGroup(t *testing.T) {
	a := newTestApp(t)
	settings := a.Settings()
	settings.AppPort = freeUDPPort(t)
	if err := a.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	live, err := a.CreateGroup("Live", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	idle, err := a.CreateGroup("Idle", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	liveService := group.Service{ID: "live", Name: "Live Service", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25565}}
	idleService := group.Service{ID: "idle", Name: "Idle Service", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25566}}
	if err := a.AddService(live.ID, liveService); err != nil {
		t.Fatal(err)
	}
	if err := a.AddService(idle.ID, idleService); err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(live.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.StopSession() })

	a.mu.Lock()
	host := a.host
	a.mu.Unlock()
	if host == nil {
		t.Fatal("the live host is missing")
	}
	checkLiveServices := func(label string) {
		t.Helper()
		services := host.Services()
		if len(services) != 1 || services[0].ID != "live" {
			t.Fatalf("%s: live services = %+v", label, services)
		}
	}
	checkLiveServices("before idle changes")

	updated := idleService
	updated.Name = "Idle Updated"
	updated.Ports = []int{25567}
	if err := a.UpdateService(idle.ID, updated); err != nil {
		t.Fatal(err)
	}
	checkLiveServices("after an idle update")

	if err := a.AddService(idle.ID, group.Service{ID: "idle2", Name: "Idle Two", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25568}}); err != nil {
		t.Fatal(err)
	}
	checkLiveServices("after an idle add")

	if err := a.RemoveService(idle.ID, "idle2"); err != nil {
		t.Fatal(err)
	}
	checkLiveServices("after an idle remove")
}
