package app

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/relay"
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
	if got.UseRelay {
		t.Fatalf("connection uses the relay by default: %+v", got)
	}
}

func TestEffectiveConnectionInheritsRelayFromSettings(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.UseRelay = true
	s.RelayHost = "relay.example.net"
	s.RelayPort = 7443
	s.RelayID = strings.Repeat("ab", 32)
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UseRelay || got.RelayCustom {
		t.Fatalf("relay flags = %+v", got)
	}
	if got.RelayHost != "relay.example.net" || got.RelayPort != 7443 || got.RelayID != s.RelayID {
		t.Fatalf("relay connection = %+v", got)
	}
	if got.Custom {
		t.Fatalf("an inherited connection is marked custom: %+v", got)
	}
	endpoint, err := a.GroupEndpoint(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "relay.example.net:7443" {
		t.Fatalf("endpoint = %q", endpoint)
	}
}

func TestEffectiveConnectionUsesGroupRelayOverride(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.UseRelay = true
	s.RelayHost = "relay.example.net"
	s.RelayPort = 7443
	s.RelayID = strings.Repeat("ab", 32)
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	groupRelayID := strings.Repeat("cd", 32)

	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: true,
		RelayHost:   "private.example.net",
		RelayPort:   9443,
		RelayID:     groupRelayID,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UseRelay || !got.RelayCustom || !got.Custom {
		t.Fatalf("relay flags = %+v", got)
	}
	if got.RelayHost != "private.example.net" || got.RelayPort != 9443 || got.RelayID != groupRelayID {
		t.Fatalf("relay connection = %+v", got)
	}

	// Unchecking the custom relay falls back to the relay in Settings but
	// keeps the group's custom values saved.
	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: false,
		RelayHost:   "private.example.net",
		RelayPort:   9443,
		RelayID:     groupRelayID,
	}); err != nil {
		t.Fatal(err)
	}
	got, err = a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RelayCustom || got.RelayHost != "relay.example.net" || got.RelayID != s.RelayID {
		t.Fatalf("inherited relay = %+v", got)
	}
	saved, ok := a.Group(g.ID)
	if !ok || saved.RelayHost != "private.example.net" || saved.RelayID != groupRelayID {
		t.Fatalf("saved custom relay = %+v", saved)
	}

	// Turning the override off uses Settings and ignores the saved values.
	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{}); err != nil {
		t.Fatal(err)
	}
	got, err = a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Custom || !got.UseRelay || got.RelayHost != "relay.example.net" {
		t.Fatalf("inherited connection = %+v", got)
	}
}

func TestEffectiveConnectionIgnoresValuesWithoutOverride(t *testing.T) {
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

	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:      false,
		PublicAddress: "group.example.net",
		AppPort:       41000,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Custom || got.PublicAddress != "home.example.net" || got.AppPort != 51820 {
		t.Fatalf("connection = %+v", got)
	}
	saved, ok := a.Group(g.ID)
	if !ok || saved.PublicAddress != "group.example.net" || saved.AppPort != 41000 {
		t.Fatalf("inactive values were not kept: %+v", saved)
	}

	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:      true,
		PublicAddress: saved.PublicAddress,
		AppPort:       saved.AppPort,
	}); err != nil {
		t.Fatal(err)
	}
	got, err = a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Custom || got.PublicAddress != "group.example.net" || got.AppPort != 41000 {
		t.Fatalf("connection after override = %+v", got)
	}
}

func TestSetGroupConnectionOptionsValidates(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	validID := strings.Repeat("ab", 32)
	bad := []GroupConnectionInput{
		{Override: true, UseRelay: true, RelayCustom: true, RelayPort: 7443, RelayID: validID},
		{Override: true, UseRelay: true, RelayCustom: true, RelayHost: "relay.example.net", RelayPort: 7443, RelayID: "abcd"},
		{Override: true, UseRelay: true, RelayCustom: true, RelayHost: "https://relay.example.net", RelayPort: 7443, RelayID: validID},
		{Override: true, PublicAddress: "https://example.net"},
		{Override: true, AppPort: 70000},
	}
	for i, input := range bad {
		if err := a.SetGroupConnectionOptions(g.ID, input); err == nil {
			t.Errorf("case %d was accepted", i)
		}
	}

	// A blank custom relay port uses the relay default.
	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: true,
		RelayHost:   "relay.example.net",
		RelayID:     validID,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RelayPort != config.DefaultRelayPort {
		t.Fatalf("relay port = %d", got.RelayPort)
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

func TestRelayPasswordSecret(t *testing.T) {
	a := newTestApp(t)
	if a.RelayPasswordSet() {
		t.Fatal("a password exists on a fresh app")
	}
	if err := a.SetRelayPassword("   "); err == nil {
		t.Fatal("an empty password was accepted")
	}
	if err := a.SetRelayPassword(" relay secret "); err != nil {
		t.Fatal(err)
	}
	if !a.RelayPasswordSet() {
		t.Fatal("the password was not stored")
	}
	a.mu.Lock()
	password, ok, err := a.relayPasswordLocked(defaultRelaySecret)
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || password != "relay secret" {
		t.Fatalf("password = %q, ok = %v", password, ok)
	}
	if err := a.ClearRelayPassword(); err != nil {
		t.Fatal(err)
	}
	if a.RelayPasswordSet() {
		t.Fatal("the password was not cleared")
	}
}

func TestGroupRelayPasswordAndReset(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if a.GroupRelayPasswordSet(g.ID) {
		t.Fatal("a password exists on a fresh group")
	}
	if err := a.SetGroupRelayPassword(g.ID, "group relay secret"); err != nil {
		t.Fatal(err)
	}
	if !a.GroupRelayPasswordSet(g.ID) {
		t.Fatal("the group password was not stored")
	}
	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:      true,
		UseRelay:      true,
		RelayCustom:   true,
		RelayHost:     "relay.example.net",
		RelayID:       strings.Repeat("ab", 32),
		PublicAddress: "ignored.example.net",
		AppPort:       41000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.ResetGroupConnection(g.ID); err != nil {
		t.Fatal(err)
	}
	got, err := a.EffectiveConnection(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Custom || got.RelayCustom || got.UseRelay {
		t.Fatalf("reset connection = %+v", got)
	}
	saved, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group missing after reset")
	}
	if saved.OverrideSettings || saved.UseRelay || saved.RelayCustom || saved.RelayHost != "" || saved.RelayID != "" {
		t.Fatalf("saved connection after reset = %+v", saved)
	}
	if saved.PublicAddress != "" || saved.AppPort != 0 {
		t.Fatalf("saved direct values after reset = %+v", saved)
	}
	if a.GroupRelayPasswordSet(g.ID) {
		t.Fatal("the group password was not cleared")
	}
}

func TestDeleteGroupClearsRelayPassword(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupRelayPassword(g.ID, "group relay secret"); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if a.GroupRelayPasswordSet(g.ID) {
		t.Fatal("the group password survived deletion")
	}
}

func TestImportRelayProfile(t *testing.T) {
	a := newTestApp(t)
	id := strings.Repeat("ab", 32)
	profile := relay.HostProfile{
		Version:     relay.Version,
		Address:     "relay.example.net:9443",
		Fingerprint: strings.ToUpper(id),
		Password:    "relay secret",
	}
	path := filepath.Join(t.TempDir(), relay.ProfileFileName)
	if err := relay.SaveProfile(path, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.ImportRelayProfile(data)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UseRelay || got.RelayHost != "relay.example.net" || got.RelayPort != 9443 || got.RelayID != id {
		t.Fatalf("settings = %+v", got)
	}
	if !a.RelayPasswordSet() {
		t.Fatal("the relay password was not imported")
	}
	a.mu.Lock()
	password, ok, err := a.relayPasswordLocked(defaultRelaySecret)
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || password != "relay secret" {
		t.Fatalf("password = %q, ok = %v", password, ok)
	}
}

func TestImportRelayProfileRejectsBadData(t *testing.T) {
	a := newTestApp(t)
	before := a.Settings()
	if _, err := a.ImportRelayProfile([]byte("not a profile")); err == nil {
		t.Fatal("bad data was accepted")
	}
	if a.Settings() != before {
		t.Fatalf("settings changed: %+v", a.Settings())
	}
	if a.RelayPasswordSet() {
		t.Fatal("a password was stored")
	}
}

func freeRelayPort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err == nil {
			_ = udp.Close()
			return port
		}
	}
	t.Fatal("could not find a free relay port")
	return 0
}

type testRelayServer struct {
	profile relay.HostProfile
}

func startTestRelayServer(t *testing.T, password string) *testRelayServer {
	t.Helper()
	port := freeRelayPort(t)
	dir := t.TempDir()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	profile, err := relay.Setup(relay.SetupOptions{
		Dir:           dir,
		PublicAddress: address,
		Listen:        address,
		Password:      password,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := relay.LoadConfig(relay.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	cert, fingerprint, err := relay.LoadIdentity(relay.KeyPath(relay.ConfigPath(dir), cfg))
	if err != nil {
		t.Fatal(err)
	}
	server, err := relay.NewServer(cfg, cert, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return &testRelayServer{profile: profile}
}

func enableSettingsRelay(t *testing.T, a *App, relayServer *testRelayServer) {
	t.Helper()
	s := a.Settings()
	s.UseRelay = true
	s.RelayHost = relayServer.profile.Host
	s.RelayPort = relayServer.profile.Port
	s.RelayID = relayServer.profile.Fingerprint
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRelayPassword(relayServer.profile.Password); err != nil {
		t.Fatal(err)
	}
}

func TestTestRelayConnection(t *testing.T) {
	relayServer := startTestRelayServer(t, "relay test password")
	a := newTestApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.TestRelayConnection(ctx, relayServer.profile.Host, relayServer.profile.Port, relayServer.profile.Fingerprint, relayServer.profile.Password); err != nil {
		t.Fatalf("relay test failed: %v", err)
	}
	if err := a.TestRelayConnection(ctx, relayServer.profile.Host, relayServer.profile.Port, relayServer.profile.Fingerprint, "wrong password"); err == nil {
		t.Fatal("a wrong relay password passed the test")
	}
}

func TestAppRelayHostAndFriend(t *testing.T) {
	relayServer := startTestRelayServer(t, "relay test password")
	hostApp := newTestApp(t)
	s := hostApp.Settings()
	s.ApproveNewDevices = false
	if err := hostApp.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	enableSettingsRelay(t, hostApp, relayServer)
	g, err := hostApp.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if err := hostApp.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hostApp.StopSession() })

	code, err := hostApp.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := invite.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Version != invite.RelayVersion || inv.Endpoint != "" {
		t.Fatalf("invite = %+v", inv)
	}
	if inv.RelayHost != relayServer.profile.Host || inv.RelayPort != relayServer.profile.Port || inv.RelayID != relayServer.profile.Fingerprint {
		t.Fatalf("invite relay = %+v", inv)
	}
	if _, err := relay.DecodeFriendToken(inv.RelayToken); err != nil {
		t.Fatalf("relay token = %v", err)
	}

	friendApp := newTestApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	joined, err := friendApp.JoinInvite(ctx, code, "ash-desktop")
	if err != nil {
		t.Fatal(err)
	}
	if !joined.UseRelay || !joined.OverrideSettings || !joined.RelayCustom {
		t.Fatalf("joined group = %+v", joined)
	}
	if joined.RelayHost != inv.RelayHost || joined.RelayPort != inv.RelayPort || joined.RelayID != inv.RelayID {
		t.Fatalf("joined relay = %+v", joined)
	}
	if joined.Endpoint != "" {
		t.Fatalf("joined endpoint = %q", joined.Endpoint)
	}
	if !friendApp.FriendPresence().Live {
		t.Fatal("the friend is not live")
	}
	waitFor(t, 3*time.Second, func() bool {
		return len(hostApp.HostPresence()) > 0
	})

	if err := friendApp.StopSession(); err != nil {
		t.Fatal(err)
	}
	err = friendApp.Reconnect(ctx, joined.ID)
	if err == nil || !strings.Contains(err.Error(), "new invite") {
		t.Fatalf("reconnect error = %v", err)
	}
	if status := friendApp.ReconnectStatus(joined.ID); status.Active {
		t.Fatal("relay reconnect was scheduled")
	}
}

func TestCreateRelayInviteNeedsLiveSession(t *testing.T) {
	relayServer := startTestRelayServer(t, "relay test password")
	a := newTestApp(t)
	enableSettingsRelay(t, a, relayServer)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateInvite(g.ID, "Ash"); err == nil {
		t.Fatal("a relay invite was created without a live session")
	}
}

func TestStartRelaySessionNeedsPassword(t *testing.T) {
	relayServer := startTestRelayServer(t, "relay test password")
	a := newTestApp(t)
	s := a.Settings()
	s.UseRelay = true
	s.RelayHost = relayServer.profile.Host
	s.RelayPort = relayServer.profile.Port
	s.RelayID = relayServer.profile.Fingerprint
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	err = a.StartSession(g.ID)
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("start error = %v", err)
	}
	if a.LiveGroupID() != "" {
		t.Fatal("a relay session started without a password")
	}
}

func TestRelaySessionUsesGroupCustomRelay(t *testing.T) {
	relayServer := startTestRelayServer(t, "group relay password")
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: true,
		RelayHost:   relayServer.profile.Host,
		RelayPort:   relayServer.profile.Port,
		RelayID:     relayServer.profile.Fingerprint,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGroupRelayPassword(g.ID, relayServer.profile.Password); err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	defer a.StopSession()
	code, err := a.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := invite.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Version != invite.RelayVersion || inv.RelayHost != relayServer.profile.Host {
		t.Fatalf("invite = %+v", inv)
	}
}

func TestRelaySettingsCannotChangeWhileLive(t *testing.T) {
	relayServer := startTestRelayServer(t, "relay test password")
	a := newTestApp(t)
	enableSettingsRelay(t, a, relayServer)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	defer a.StopSession()

	s := a.Settings()
	s.RelayHost = "other.example.net"
	s.RelayID = strings.Repeat("cd", 32)
	if err := a.SaveSettings(s); err == nil || !strings.Contains(err.Error(), "relay") {
		t.Fatalf("settings change error = %v", err)
	}
	if err := a.SetGroupConnectionOptions(g.ID, GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: true,
		RelayHost:   "other.example.net",
		RelayPort:   relayServer.profile.Port,
		RelayID:     relayServer.profile.Fingerprint,
	}); err == nil || !strings.Contains(err.Error(), "relay") {
		t.Fatalf("group relay change error = %v", err)
	}
}

func TestRelaySettingsChangeCancelsInvitesAfterStop(t *testing.T) {
	relayServer := startTestRelayServer(t, "relay test password")
	a := newTestApp(t)
	enableSettingsRelay(t, a, relayServer)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateInvite(g.ID, "Ash"); err != nil {
		t.Fatal(err)
	}
	if err := a.StopSession(); err != nil {
		t.Fatal(err)
	}
	saved, ok := a.Group(g.ID)
	if !ok || len(saved.OpenInvites(time.Now())) != 1 {
		t.Fatalf("open invites = %+v", saved.Invites)
	}

	s := a.Settings()
	s.RelayHost = "other.example.net"
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	saved, ok = a.Group(g.ID)
	if !ok || len(saved.OpenInvites(time.Now())) != 0 {
		t.Fatalf("invites survived the relay change: %+v", saved.Invites)
	}
}

func TestSaveSettingsWithRelayPassword(t *testing.T) {
	a := newTestApp(t)
	s := a.Settings()
	s.UseRelay = true
	s.RelayHost = "relay.example.net"
	s.RelayPort = 7443
	s.RelayID = strings.Repeat("ab", 32)

	if err := a.SaveSettingsWithRelayPassword(s, ""); err == nil {
		t.Fatal("relay settings saved without a password")
	}
	if err := a.SaveSettingsWithRelayPassword(s, "relay secret"); err != nil {
		t.Fatal(err)
	}
	if got := a.Settings(); got != s {
		t.Fatalf("settings = %+v", got)
	}
	if err := a.SaveSettingsWithRelayPassword(s, ""); err != nil {
		t.Fatalf("a saved password was not reused: %v", err)
	}

	bad := s
	bad.RelayPort = 70000
	if err := a.SaveSettingsWithRelayPassword(bad, "new secret"); err == nil {
		t.Fatal("invalid settings were accepted")
	}
	a.mu.Lock()
	password, ok, err := a.relayPasswordLocked(defaultRelaySecret)
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || password != "relay secret" {
		t.Fatalf("password after a failed save = %q, ok = %v", password, ok)
	}
}

func TestSaveGroupConnectionOptionsPassword(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	in := GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: true,
		RelayHost:   "relay.example.net",
		RelayPort:   7443,
		RelayID:     strings.Repeat("ab", 32),
	}
	if err := a.SaveGroupConnectionOptions(g.ID, in, ""); err == nil {
		t.Fatal("a custom relay saved without a password")
	}
	if err := a.SaveGroupConnectionOptions(g.ID, in, "group secret"); err != nil {
		t.Fatal(err)
	}
	if !a.GroupRelayPasswordSet(g.ID) {
		t.Fatal("the group password was not stored")
	}
	if err := a.SaveGroupConnectionOptions(g.ID, in, ""); err != nil {
		t.Fatalf("a saved group password was not reused: %v", err)
	}

	bad := in
	bad.RelayID = "short"
	if err := a.SaveGroupConnectionOptions(g.ID, bad, "new secret"); err == nil {
		t.Fatal("invalid relay settings were accepted")
	}
	a.mu.Lock()
	password, ok, err := a.relayPasswordLocked(groupRelaySecret(g.ID))
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || password != "group secret" {
		t.Fatalf("password after a failed save = %q, ok = %v", password, ok)
	}

	// Switching to the Settings relay keeps the custom password for later.
	inherited := in
	inherited.RelayCustom = false
	if err := a.SaveGroupConnectionOptions(g.ID, inherited, ""); err != nil {
		t.Fatal(err)
	}
	if !a.GroupRelayPasswordSet(g.ID) {
		t.Fatal("the custom password was cleared while the values were kept")
	}
}

func TestPreviewGroupConnection(t *testing.T) {
	a := newTestApp(t)
	g, err := a.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	s := a.Settings()
	s.UseRelay = true
	s.RelayHost = "relay.example.net"
	s.RelayPort = 7443
	s.RelayID = strings.Repeat("ab", 32)
	if err := a.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	custom := GroupConnectionInput{
		Override:    true,
		UseRelay:    true,
		RelayCustom: true,
		RelayHost:   "private.example.net",
		RelayPort:   9443,
		RelayID:     strings.Repeat("cd", 32),
	}
	preview, err := a.PreviewGroupConnection(g.ID, custom)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Custom || !preview.RelayCustom || preview.RelayHost != "private.example.net" {
		t.Fatalf("custom preview = %+v", preview)
	}
	saved, ok := a.Group(g.ID)
	if !ok || saved.OverrideSettings || saved.RelayHost != "" {
		t.Fatalf("preview changed the group: %+v", saved)
	}
	inherited, err := a.PreviewGroupConnection(g.ID, GroupConnectionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if inherited.Custom || !inherited.UseRelay || inherited.RelayHost != "relay.example.net" {
		t.Fatalf("inherited preview = %+v", inherited)
	}
}
