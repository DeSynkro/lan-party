package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	h := rr.Header()
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing CSP")
	}
	if strings.Contains(h.Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatal("CSP allows inline script")
	}
	if h.Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
	if h.Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing frame denial")
	}
	if h.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("missing referrer policy")
	}
}

func TestRequireAuthRedirectsAndAllows(t *testing.T) {
	store := NewSessionStore(time.Hour, time.Hour)
	session, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	handler := RequireAuth(store, func(w http.ResponseWriter, r *http.Request) {
		if SessionFrom(r) == nil {
			t.Error("session missing from context")
		}
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/groups", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("unauth: %d %q", rr.Code, rr.Header().Get("Location"))
	}

	req := httptest.NewRequest("GET", "/groups", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.ID})
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("auth: %d", rr.Code)
	}
}

func TestCheckOrigin(t *testing.T) {
	r := httptest.NewRequest("POST", "http://lan.local/settings", nil)
	r.Host = "lan.local"
	if err := CheckOrigin(r, ""); err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", "http://lan.local")
	if err := CheckOrigin(r, ""); err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", "http://evil.example")
	if err := CheckOrigin(r, ""); err == nil {
		t.Fatal("foreign origin accepted")
	}
}

func TestCheckOriginBrowserRules(t *testing.T) {
	r := httptest.NewRequest("POST", "http://lan.local/settings", nil)
	r.Host = "lan.local"

	// Same-origin form posts without an Origin are what Firefox/LibreWolf send
	// when the page sets Referrer-Policy: no-referrer.
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if err := CheckOrigin(r, ""); err != nil {
		t.Fatalf("same-origin without Origin: %v", err)
	}
	r.Header.Set("Origin", "null")
	if err := CheckOrigin(r, ""); err != nil {
		t.Fatalf("same-origin with null Origin: %v", err)
	}
	r.Header.Set("Sec-Fetch-Site", "none")
	if err := CheckOrigin(r, ""); err != nil {
		t.Fatalf("user navigation with null Origin: %v", err)
	}

	r.Header.Set("Sec-Fetch-Site", "cross-site")
	if err := CheckOrigin(r, ""); err == nil {
		t.Fatal("cross-site null Origin accepted")
	}
	r.Header.Set("Origin", "http://evil.example")
	if err := CheckOrigin(r, ""); err == nil {
		t.Fatal("foreign origin accepted")
	}
}

func TestCheckCSRF(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("csrf=wrong"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s := &Session{CSRF: "right"}
	if err := CheckCSRF(r, s); err == nil {
		t.Fatal("wrong token accepted")
	}

	r = httptest.NewRequest("POST", "/", strings.NewReader("csrf=right"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := CheckCSRF(r, s); err != nil {
		t.Fatal(err)
	}
}

func TestLimitBody(t *testing.T) {
	var readErr error
	handler := LimitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}), 8)
	req := httptest.NewRequest("POST", "/", strings.NewReader("more than eight bytes"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if readErr == nil {
		t.Fatal("oversized body accepted")
	}
}
