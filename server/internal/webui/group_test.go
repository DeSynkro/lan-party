package webui

import (
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestGroupPageShowsSessionAndServices(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{
		ID: "g1", Name: "Game night", Role: group.RoleHost, HostName: "Server",
		Services: []group.Service{{ID: "s1", Name: "Minecraft", Proto: "tcp", Target: "127.0.0.1:25565", Ports: []int{25565}}},
	})
	rr := srv.get("/groups/g1")
	body := rr.Body.String()
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(body, "Game night") || !strings.Contains(body, "Minecraft") {
		t.Fatal("missing content")
	}
	if !strings.Contains(body, "Session off") {
		t.Fatal("missing session state")
	}
	if !strings.Contains(body, "managed by this deployment") {
		t.Fatal("missing app port note")
	}
}

func TestGroupPageMissingIs404(t *testing.T) {
	srv := newTestServer(t)
	if rr := srv.get("/groups/nope"); rr.Code != 404 {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGroupPageHidesFriendGroup(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "f1", Name: "Friend group", Role: group.RoleFriend})
	if rr := srv.get("/groups/f1"); rr.Code != 404 {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGroupPageShowsInviteCode(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	srv.fake.lastInvite = "invite-code-1"
	srv.fake.groups[0].Invites = append(srv.fake.groups[0].Invites, group.Invite{ID: "i1", Name: "Ash"})
	rr := srv.get("/groups/g1")
	body := rr.Body.String()
	if !strings.Contains(body, "invite-code-1") {
		t.Fatal("invite code missing")
	}
	if !strings.Contains(body, `data-copy="invite-i1"`) {
		t.Fatal("copy control missing")
	}
}
