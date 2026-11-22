package webui

import (
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestDashboardRendersGroups(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "<script>alert(1)</script>", Role: group.RoleHost, HostName: "Server"})
	rr := srv.get("/")
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("unescaped group name")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("escaped name missing")
	}
}

func TestDashboardEmptyState(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.get("/")
	body := rr.Body.String()
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(body, "Create your first group") {
		t.Fatal("empty state missing")
	}
	if strings.Contains(body, "Join a group") {
		t.Fatal("join flow leaked into the host UI")
	}
}

func TestDashboardHidesFriendGroups(t *testing.T) {
	srv := newTestServerWithGroups(t,
		group.Group{ID: "h1", Name: "Hosted", Role: group.RoleHost},
		group.Group{ID: "f1", Name: "Friend group", Role: group.RoleFriend},
	)
	rr := srv.get("/")
	body := rr.Body.String()
	if !strings.Contains(body, "Hosted") {
		t.Fatal("host group missing")
	}
	if strings.Contains(body, "Friend group") {
		t.Fatal("friend group shown")
	}
}

func TestDashboardShowsLiveState(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost, HostName: "Server"})
	srv.fake.live = "g1"
	rr := srv.get("/")
	body := rr.Body.String()
	if !strings.Contains(body, "Live") || !strings.Contains(body, "Session running") {
		t.Fatal("live state missing")
	}
}
