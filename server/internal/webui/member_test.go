package webui

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
)

func TestApprovalAllowAndDeny(t *testing.T) {
	srv := newTestServer(t)
	srv.fake.approval = &app.Approval{ID: "a1", GroupID: "g1", PeerName: "Ash", DeviceName: "Laptop", Address: "10.0.0.5", Fingerprint: "ABCD"}
	rr := srv.post("/approvals/a1", url.Values{"allow": {"yes"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.fake.resolved || !srv.fake.allowed {
		t.Fatal("approval not resolved")
	}
}

func TestApprovalDeny(t *testing.T) {
	srv := newTestServer(t)
	srv.fake.approval = &app.Approval{ID: "a1", GroupID: "g1", PeerName: "Ash"}
	rr := srv.post("/approvals/a1", url.Values{"allow": {"no"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.fake.resolved || srv.fake.allowed {
		t.Fatal("denial not recorded")
	}
}

func TestApprovalUnknownIsConflict(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/approvals/nope", url.Values{"allow": {"yes"}})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestApprovalBannerShown(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	srv.fake.approval = &app.Approval{ID: "a1", GroupID: "g1", PeerName: "Ash", DeviceName: "Laptop"}
	rr := srv.get("/")
	body := rr.Body.String()
	if !strings.Contains(body, "Ash") || !strings.Contains(body, "Review the request") {
		t.Fatal("approval banner missing")
	}
}

func TestRemoveMember(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Members: []group.Member{{ID: "m1", Name: "Ash", State: group.MemberApproved}},
	})
	rr := srv.post("/groups/g1/members/m1/remove", url.Values{})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if len(srv.fake.groups[0].Members) != 0 {
		t.Fatal("member not removed")
	}
}

func TestMemberPageShowsDetailsWithoutKey(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Members: []group.Member{{
			ID: "m1", Name: "Ash", DeviceName: "Laptop", PublicKey: "private-key-material",
			Fingerprint: "ABCD", LastAddress: "10.0.0.5", State: group.MemberApproved,
			FirstSeen: time.Now().Add(-time.Hour), LastSeen: time.Now(),
		}},
	})
	rr := srv.get("/groups/g1/members/m1")
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	for _, want := range []string{"Ash", "Laptop", "ABCD", "10.0.0.5"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(body, "private-key-material") {
		t.Fatal("public key leaked into the page")
	}
	if strings.Contains(body, "relay") {
		t.Fatal("relay fields leaked into the member page")
	}
}

func TestMemberRemoveConfirmation(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Members: []group.Member{{ID: "m1", Name: "Ash"}},
	})
	rr := srv.get("/groups/g1/members/m1/remove")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Remove Ash") {
		t.Fatalf("confirmation: %d", rr.Code)
	}
}
