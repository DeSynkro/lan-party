package webui

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestUnauthenticatedGetRedirects(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Game"}},
		Members:  []group.Member{{ID: "m1", Name: "Ash"}},
		Invites:  []group.Invite{{ID: "i1", Name: "Ash"}},
	})
	paths := []string{
		"/", "/groups/g1", "/groups/new", "/groups/g1/delete",
		"/groups/g1/services/new", "/groups/g1/services/s1", "/groups/g1/services/s1/remove",
		"/groups/g1/invites/new", "/groups/g1/invites/i1/cancel",
		"/groups/g1/members/m1", "/groups/g1/members/m1/remove",
		"/settings", "/settings/password", "/guide", "/events",
	}
	for _, path := range paths {
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
			t.Fatalf("%s: %d %q", path, rr.Code, rr.Header().Get("Location"))
		}
		if strings.Contains(rr.Body.String(), "Friday") {
			t.Fatalf("%s leaked content to a logged-out request", path)
		}
	}
}

func TestUnauthenticatedPostRejected(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	paths := []string{
		"/groups", "/groups/g1/start", "/groups/g1/stop", "/groups/g1/delete",
		"/groups/g1/services", "/groups/g1/invites", "/approvals/a1",
		"/settings", "/settings/password", "/logout",
	}
	for _, path := range paths {
		req := httptest.NewRequest("POST", path, strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "http://lan.local")
		req.Host = "lan.local"
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("%s: %d", path, rr.Code)
		}
	}
	if srv.fake.started {
		t.Fatal("an unauthenticated request started a session")
	}
}

func TestForeignOriginRejected(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "http://lan.local/settings", strings.NewReader("publicAddress=evil.example"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://evil.example")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress == "evil.example" {
		t.Fatal("foreign origin changed settings")
	}
}

func TestSameOriginPostWithoutOriginIsAllowed(t *testing.T) {
	srv := newTestServer(t)
	values := url.Values{"publicAddress": {"192.168.1.20"}, "csrf": {srv.session.CSRF}}
	req := httptest.NewRequest("POST", "http://lan.local/settings", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress != "192.168.1.20" {
		t.Fatal("same-origin post was not saved")
	}
}

func TestCrossSitePostWithoutOriginRejected(t *testing.T) {
	srv := newTestServer(t)
	values := url.Values{"publicAddress": {"evil.example"}, "csrf": {srv.session.CSRF}}
	req := httptest.NewRequest("POST", "http://lan.local/settings", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress == "evil.example" {
		t.Fatal("cross-site post changed settings")
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	srv := newTestServer(t)
	big := "password=" + strings.Repeat("a", maxFormBytes+1024)
	req := httptest.NewRequest("POST", "http://lan.local/login", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://lan.local")
	req.Host = "lan.local"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestLoginOverTLSGetsSecureCookie(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"password": {testPassword}}
	req := httptest.NewRequest("POST", "https://lan.local/login", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://lan.local")
	req.Host = "lan.local"
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].Secure {
		t.Fatalf("cookie: %+v", cookies)
	}
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("missing HSTS on TLS")
	}
}

func TestStaticPathTraversal(t *testing.T) {
	srv := newTestServer(t)
	for _, path := range []string{"/static/../go.mod", "/static/%2e%2e/go.mod", "/static/../../go.mod"} {
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if rr.Code == http.StatusOK && strings.Contains(rr.Body.String(), "module github.com") {
			t.Fatalf("%s served go.mod", path)
		}
	}
}

func TestGroupPageEscapesServiceAndInviteNames(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "<img src=x onerror=alert(1)>", Proto: "tcp", Target: "127.0.0.1", Ports: []int{1}, Hint: "<b>hint</b>"}},
		Invites:  []group.Invite{{ID: "i1", Name: "<script>alert(1)</script>"}},
	})
	rr := srv.get("/groups/g1")
	body := rr.Body.String()
	if strings.Contains(body, "<img src=x onerror=alert(1)>") || strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("unescaped value in group page")
	}
	if !strings.Contains(body, "&lt;img src=x") {
		t.Fatal("expected escaped service name")
	}
}

func TestMemberPageEscapesName(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Members: []group.Member{{ID: "m1", Name: "<script>alert(1)</script>"}},
	})
	rr := srv.get("/groups/g1/members/m1")
	if strings.Contains(rr.Body.String(), "<script>alert(1)</script>") {
		t.Fatal("unescaped member name")
	}
}

func TestClientIPTrustsOnlyConfiguredProxies(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	if got := srv.clientIP(req); got != "127.0.0.1" {
		t.Fatalf("untrusted proxy honoured: %s", got)
	}
	srv.cfg.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	if got := srv.clientIP(req); got != "203.0.113.5" {
		t.Fatalf("trusted proxy ignored: %s", got)
	}
}
