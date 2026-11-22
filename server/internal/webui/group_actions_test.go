package webui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestCreateGroup(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/groups", url.Values{"name": {"Friday"}, "hostName": {"Server"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if len(srv.fake.groups) != 1 || srv.fake.groups[0].Name != "Friday" {
		t.Fatal("group not created")
	}
	if rr.Header().Get("Location") != "/groups/"+srv.fake.groups[0].ID {
		t.Fatalf("location = %q", rr.Header().Get("Location"))
	}
}

func TestCreateGroupValidationError(t *testing.T) {
	srv := newTestServer(t)
	srv.fake.groupErr = errors.New("give the group a name")
	rr := srv.post("/groups", url.Values{"name": {""}, "hostName": {"Server"}})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "give the group a name") {
		t.Fatal("validation message missing")
	}
}

func TestDeleteGroupRequiresConfirmation(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.get("/groups/g1/delete")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Delete") {
		t.Fatalf("confirmation missing: %d", rr.Code)
	}
	rr = srv.post("/groups/g1/delete", url.Values{})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d", rr.Code)
	}
	if len(srv.fake.groups) != 0 {
		t.Fatal("group not deleted")
	}
}

func TestStartAndStop(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	if rr := srv.post("/groups/g1/start", url.Values{}); rr.Code != http.StatusSeeOther {
		t.Fatalf("start = %d", rr.Code)
	}
	if !srv.fake.started || srv.fake.live != "g1" {
		t.Fatal("session not started")
	}
	if rr := srv.post("/groups/g1/stop", url.Values{}); rr.Code != http.StatusSeeOther {
		t.Fatalf("stop = %d", rr.Code)
	}
	if srv.fake.started || srv.fake.live != "" {
		t.Fatal("session not stopped")
	}
}

func TestStartErrorReRendersGroup(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	srv.fake.sessionErr = errors.New("a session is already running")
	rr := srv.post("/groups/g1/start", url.Values{})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "a session is already running") {
		t.Fatal("error message missing")
	}
}

func TestGroupActionsRequireCSRF(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	req := httptest.NewRequest("POST", "http://lan.local/groups/g1/start", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://lan.local")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.started {
		t.Fatal("session started without a CSRF token")
	}
}
