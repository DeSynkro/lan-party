package group

import (
	"strings"
	"testing"
	"time"
)

func TestCreateHostGroup(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	if g.Role != RoleHost || !g.IsHost() {
		t.Fatalf("role = %q", g.Role)
	}
	if g.Name != "Friday Night" || g.HostName != "Desynkro" {
		t.Fatalf("group = %+v", g)
	}
	if len(g.ID) != 16 {
		t.Fatalf("id = %q", g.ID)
	}
	if g.CreatedAt.IsZero() {
		t.Fatal("created at is zero")
	}
}

func TestAddServiceAndConflict(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	minecraft := Service{ID: "mc", Name: "Minecraft", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25565}}
	if err := g.AddService(minecraft); err != nil {
		t.Fatal(err)
	}
	if err := g.AddService(Service{Name: "Another TCP", Proto: "tcp", Target: "192.168.1.51", Ports: []int{25565}}); err == nil {
		t.Fatal("a TCP port conflict was accepted")
	}
	if err := g.AddService(Service{Name: "Same port, UDP", Proto: "udp", Target: "192.168.1.51", Ports: []int{25565}}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddService(Service{Name: "Both", Proto: "both", Target: "192.168.1.52", Ports: []int{25565}}); err == nil {
		t.Fatal("a both-protocol conflict was accepted")
	}
	if !g.RemoveService("mc") {
		t.Fatal("service was not removed")
	}
	if err := g.AddService(minecraft); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateService(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	minecraft := Service{ID: "mc", Name: "Minecraft", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25565}}
	voice := Service{ID: "voice", Name: "Voice", Proto: "udp", Target: "192.168.1.50", Ports: []int{25565}}
	if err := g.AddService(minecraft); err != nil {
		t.Fatal(err)
	}
	if err := g.AddService(voice); err != nil {
		t.Fatal(err)
	}

	updated := minecraft
	updated.Name = "Minecraft Modded"
	updated.Target = "localhost"
	updated.Ports = []int{25566}
	if err := g.UpdateService(updated); err != nil {
		t.Fatal(err)
	}
	got, ok := g.FindService("mc")
	if !ok {
		t.Fatal("the service disappeared")
	}
	if got.Name != "Minecraft Modded" || got.Target != "127.0.0.1" ||
		len(got.Ports) != 1 || got.Ports[0] != 25566 {
		t.Fatalf("updated service = %+v", got)
	}

	web := Service{ID: "web", Name: "Web", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25565}}
	if err := g.AddService(web); err != nil {
		t.Fatal(err)
	}
	conflict := minecraft
	conflict.Ports = []int{25565}
	if err := g.UpdateService(conflict); err == nil {
		t.Fatal("a conflict with another service was accepted")
	}
	got, _ = g.FindService("mc")
	if len(got.Ports) != 1 || got.Ports[0] != 25566 {
		t.Fatalf("a failed update changed the service: %+v", got)
	}

	if err := g.UpdateService(Service{ID: "missing", Name: "Missing", Proto: "tcp", Target: "192.168.1.50", Ports: []int{25567}}); err == nil {
		t.Fatal("an unknown service was updated")
	}
}

func TestServiceLimit(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	for i := 0; i < MaxServices; i++ {
		s := Service{
			Name:   "Service " + strings.Repeat("x", i),
			Proto:  "tcp",
			Target: "192.168.1.50",
			Ports:  []int{1000 + i},
		}
		if err := g.AddService(s); err != nil {
			t.Fatalf("service %d: %v", i, err)
		}
	}
	if err := g.AddService(Service{Name: "One more", Proto: "tcp", Target: "192.168.1.50", Ports: []int{2000}}); err == nil {
		t.Fatal("the service limit was not enforced")
	}
}

func TestMemberLifecycle(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	m := Member{Name: "Ash", DeviceName: "ash-desktop", PublicKey: "key-1", Fingerprint: "AAAA-BBBB-CCCC-DDDD"}
	if err := g.AddMember(m); err != nil {
		t.Fatal(err)
	}
	if len(g.Members) != 1 {
		t.Fatalf("members = %d", len(g.Members))
	}
	if g.Members[0].State != MemberPending {
		t.Fatalf("state = %q", g.Members[0].State)
	}
	if _, ok := g.FindMemberByKey("key-1"); !ok {
		t.Fatal("member was not found by key")
	}
	if !g.ApproveMember(g.Members[0].ID) {
		t.Fatal("member was not approved")
	}
	if !g.Members[0].IsApproved() {
		t.Fatal("member is still pending")
	}
	if !g.RemoveMember(g.Members[0].ID) {
		t.Fatal("member was not removed")
	}
	if len(g.Members) != 0 {
		t.Fatal("members remain after removal")
	}
}

func TestMemberLimits(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	for i := 0; i < MaxMembers; i++ {
		if err := g.AddMember(Member{Name: "Friend " + strings.Repeat("x", i)}); err != nil {
			t.Fatalf("member %d: %v", i, err)
		}
	}
	if err := g.AddMember(Member{Name: "One more"}); err == nil {
		t.Fatal("the member limit was not enforced")
	}
}

func TestInviteLifecycle(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	now := time.Now().UTC()
	inv := Invite{
		Name:      "Ash",
		TokenHash: "hash",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := g.AddInvite(inv); err != nil {
		t.Fatal(err)
	}
	if len(g.OpenInvites(now)) != 1 {
		t.Fatal("open invite was not listed")
	}
	if !g.RedeemInvite(g.Invites[0].ID, now) {
		t.Fatal("invite was not redeemed")
	}
	if len(g.OpenInvites(now)) != 0 {
		t.Fatal("redeemed invite is still open")
	}
	if g.RedeemInvite(g.Invites[0].ID, now) {
		t.Fatal("invite was redeemed twice")
	}
}

func TestExpiredInvite(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	now := time.Now().UTC()
	inv := Invite{
		Name:      "Ash",
		TokenHash: "hash",
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}
	if err := g.AddInvite(inv); err != nil {
		t.Fatal(err)
	}
	if len(g.OpenInvites(now)) != 0 {
		t.Fatal("expired invite is open")
	}
	if g.RedeemInvite(g.Invites[0].ID, now) {
		t.Fatal("expired invite was redeemed")
	}
}

func TestInviteNeedsFields(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	if err := g.AddInvite(Invite{TokenHash: "hash"}); err == nil {
		t.Fatal("invite without a name was accepted")
	}
	if err := g.AddInvite(Invite{Name: "Ash"}); err == nil {
		t.Fatal("invite without a hash was accepted")
	}
}

func TestAddInviteUsesOneHourDefault(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	created := time.Now().UTC()
	if err := g.AddInvite(Invite{Name: "Ash", TokenHash: "hash", CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
	inv := g.Invites[0]
	if got := inv.ExpiresAt.Sub(created); got != InviteTTL {
		t.Fatalf("expiry = %v, want %v", got, InviteTTL)
	}
	if !inv.Open(created.Add(59 * time.Minute)) {
		t.Fatal("a 59-minute-old invite was closed")
	}
	if inv.Open(created.Add(61 * time.Minute)) {
		t.Fatal("a 61-minute-old invite was open")
	}
}

func TestClearOpenInvites(t *testing.T) {
	g := CreateHostGroup("Friday Night", "Desynkro")
	now := time.Now().UTC()
	if err := g.AddInvite(Invite{
		Name:      "Open",
		TokenHash: "open",
		CreatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	openID := g.Invites[0].ID
	if err := g.AddInvite(Invite{
		Name:      "Expired",
		TokenHash: "expired",
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddInvite(Invite{
		Name:       "Redeemed",
		TokenHash:  "redeemed",
		CreatedAt:  now.Add(-time.Minute),
		ExpiresAt:  now.Add(time.Hour),
		RedeemedAt: now.Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	removed := g.ClearOpenInvites(now)
	if len(removed) != 1 || removed[0] != openID {
		t.Fatalf("removed = %v, want [%s]", removed, openID)
	}
	if len(g.Invites) != 2 {
		t.Fatalf("kept %d invites, want 2", len(g.Invites))
	}
	for _, inv := range g.Invites {
		if inv.ID == openID {
			t.Fatal("the open invite was kept")
		}
	}
}
