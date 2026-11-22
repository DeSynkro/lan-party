package webui

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestCreateInviteShowsCodeOnce(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.post("/groups/g1/invites", url.Values{"peerName": {"Ash"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.lastInvite == "" {
		t.Fatal("no invite code")
	}
	rr = srv.get("/groups/g1")
	if !strings.Contains(rr.Body.String(), srv.fake.lastInvite) {
		t.Fatal("code not shown")
	}
}

func TestInviteRequiresSession(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.post("/groups/g1/invites", url.Values{"peerName": {"Ash"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.fake.started {
		t.Fatal("session not started")
	}
}

func TestInviteFormWarnsWhenSessionOff(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.get("/groups/g1/invites/new")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Creating the invite starts it") {
		t.Fatal("session warning missing")
	}
}

func TestInviteBlockedByOtherLiveGroup(t *testing.T) {
	srv := newTestServerWithGroups(t,
		group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost},
		group.Group{ID: "g2", Name: "Saturday", Role: group.RoleHost},
	)
	srv.fake.live = "g2"
	rr := srv.post("/groups/g1/invites", url.Values{"peerName": {"Ash"}})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "already running") {
		t.Fatal("conflict message missing")
	}
}

func TestInviteCancel(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Invites: []group.Invite{{ID: "i1", Name: "Ash"}},
	})
	rr := srv.get("/groups/g1/invites/i1/cancel")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Ash") {
		t.Fatalf("confirmation: %d", rr.Code)
	}
	rr = srv.post("/groups/g1/invites/i1/cancel", url.Values{})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("cancel: %d", rr.Code)
	}
	if len(srv.fake.groups[0].Invites) != 0 {
		t.Fatal("invite not cancelled")
	}
}

func TestInviteRequiresName(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.post("/groups/g1/invites", url.Values{"peerName": {" "}})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.started {
		t.Fatal("session started for an invalid invite")
	}
}
