package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestEveryAuthenticatedPostFormHasCSRF(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{
		ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Game", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25565}}},
		Members:  []group.Member{{ID: "m1", Name: "Ash"}},
		Invites:  []group.Invite{{ID: "i1", Name: "Ash"}},
	})
	paths := []string{
		"/groups/new",
		"/groups/g1",
		"/groups/g1/delete",
		"/groups/g1/services/new",
		"/groups/g1/services/s1",
		"/groups/g1/services/s1/remove",
		"/groups/g1/invites/new",
		"/groups/g1/invites/i1/cancel",
		"/groups/g1/members/m1/remove",
		"/settings",
		"/settings/password",
	}
	for _, path := range paths {
		rr := srv.get(path)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", path, rr.Code)
		}
		body := rr.Body.String()
		forms := strings.Count(body, `<form method="post"`)
		tokens := strings.Count(body, `name="csrf"`)
		if forms != tokens {
			t.Fatalf("%s: %d POST forms but %d CSRF fields", path, forms, tokens)
		}
	}
}

func TestNamedFormsCarryCSRF(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{
		ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Game", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25565}}},
	})
	cases := []struct{ page, action string }{
		{"/groups/new", "/groups"},
		{"/settings", "/settings"},
		{"/settings/password", "/settings/password"},
		{"/groups/g1/services/new", "/groups/g1/services"},
		{"/groups/g1/services/s1", "/groups/g1/services/s1"},
		{"/groups/g1/invites/new", "/groups/g1/invites"},
	}
	for _, c := range cases {
		page := srv.get(c.page).Body.String()
		if !formHasCSRF(page, c.action) {
			t.Fatalf("%s: form %q has no CSRF field", c.page, c.action)
		}
	}
}

func TestPasswordChangeWithRenderedToken(t *testing.T) {
	srv := newTestServer(t)
	page := srv.get("/settings/password").Body.String()
	token := extractFormCSRF(page, "/settings/password")
	if token == "" {
		t.Fatal("password form has no CSRF token")
	}
	values := url.Values{
		"csrf":    {token},
		"current": {testPassword},
		"new":     {"a new long password"},
		"confirm": {"a new long password"},
	}
	req := httptest.NewRequest("POST", "http://lan.local/settings/password", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "null")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.creds.Verify("a new long password") {
		t.Fatal("password not changed")
	}
}

func TestSettingsSaveWithRenderedToken(t *testing.T) {
	srv := newTestServer(t)
	page := srv.get("/settings").Body.String()
	token := extractFormCSRF(page, "/settings")
	if token == "" {
		t.Fatal("settings form has no CSRF token")
	}
	values := url.Values{"csrf": {token}, "publicAddress": {"192.168.1.20"}, "forgetDays": {"5"}}
	req := httptest.NewRequest("POST", "http://lan.local/settings", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "null")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress != "192.168.1.20" {
		t.Fatal("settings not saved")
	}
}

func formHasCSRF(page, action string) bool {
	return extractFormCSRF(page, action) != ""
}

func extractFormCSRF(page, action string) string {
	re := regexp.MustCompile(`(?s)<form[^>]*action="` + regexp.QuoteMeta(action) + `"[^>]*>(.*?)</form>`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	marker := `name="csrf" value="`
	start := strings.Index(m[1], marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(m[1][start:], `"`)
	if end < 0 {
		return ""
	}
	return m[1][start : start+end]
}
