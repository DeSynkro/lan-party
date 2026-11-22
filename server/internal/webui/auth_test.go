package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLoginRequired(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("redirect: %d %q", rr.Code, rr.Header().Get("Location"))
	}
}

func TestLoginPageRenders(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/login", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `name="password"`) || !strings.Contains(body, "autocomplete=\"current-password\"") {
		t.Fatal("password field missing")
	}
	if strings.Contains(body, "username") {
		t.Fatal("username field present")
	}
}

func TestLoginSuccessAndLogout(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"password": {testPassword}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://example.com/login", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/" {
		t.Fatalf("login: %d %q", rr.Code, rr.Header().Get("Location"))
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookie")
	}
	id := cookies[0].Value
	session, ok := srv.sessions.Get(id)
	if !ok {
		t.Fatal("session not stored")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "http://example.com/logout", strings.NewReader(url.Values{"csrf": {session.CSRF}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: id})
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("logout: %d %q", rr.Code, rr.Header().Get("Location"))
	}
	if _, ok := srv.sessions.Get(id); ok {
		t.Fatal("session survived logout")
	}
}

func TestLoginFailureIsGeneric(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://example.com/login", strings.NewReader("password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "wrong password") {
		t.Fatal("error leaked detail")
	}
}

func TestLoginRotatesSession(t *testing.T) {
	srv := newTestServer(t)
	old := srv.session
	body := url.Values{"password": {testPassword}}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://lan.local/login", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://lan.local")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: old.ID})
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if _, ok := srv.sessions.Get(old.ID); ok {
		t.Fatal("old session survived login")
	}
}

func TestLoginRateLimited(t *testing.T) {
	srv := newTestServer(t)
	for i := 0; i < maxLoginFailures; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "http://lan.local/login", strings.NewReader("password=wrong"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "http://lan.local")
		req.Host = "lan.local"
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", i, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://lan.local/login", strings.NewReader("password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://lan.local")
	req.Host = "lan.local"
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit status = %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestLogoutRequiresCSRF(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "http://lan.local/logout", strings.NewReader("csrf=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://lan.local")
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	if _, ok := srv.sessions.Get(srv.session.ID); !ok {
		t.Fatal("session destroyed by a forged logout")
	}
}
