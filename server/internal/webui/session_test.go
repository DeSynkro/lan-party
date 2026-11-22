package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	store := NewSessionStore(time.Hour, 24*time.Hour)
	s, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ID) < 32 || s.CSRF == "" {
		t.Fatal("weak session")
	}
	if got, ok := store.Get(s.ID); !ok || got != s {
		t.Fatal("session not found")
	}
	store.Destroy(s.ID)
	if _, ok := store.Get(s.ID); ok {
		t.Fatal("session survived destroy")
	}
}

func TestSessionIdleExpiry(t *testing.T) {
	store := NewSessionStore(time.Millisecond, time.Hour)
	s, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, ok := store.Get(s.ID); ok {
		t.Fatal("idle session survived")
	}
}

func TestSessionAbsoluteExpiry(t *testing.T) {
	store := NewSessionStore(time.Hour, time.Millisecond)
	s, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, ok := store.Get(s.ID); ok {
		t.Fatal("expired session survived")
	}
}

func TestSessionTouchExtendsIdle(t *testing.T) {
	store := NewSessionStore(50*time.Millisecond, time.Hour)
	s, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := store.Get(s.ID); !ok {
		t.Fatal("session expired early")
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := store.Get(s.ID); !ok {
		t.Fatal("touch did not extend the session")
	}
}

func TestDestroyAll(t *testing.T) {
	store := NewSessionStore(time.Hour, time.Hour)
	if _, err := store.Create(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(); err != nil {
		t.Fatal(err)
	}
	store.DestroyAll()
	if !store.Empty() {
		t.Fatal("sessions survived DestroyAll")
	}
}

func TestCookieFlags(t *testing.T) {
	rr := httptest.NewRecorder()
	SetSessionCookie(rr, "abc", true)
	c := rr.Result().Cookies()[0]
	if c.Name != sessionCookieName || c.Value != "abc" {
		t.Fatalf("cookie: %+v", c)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags: %+v", c)
	}
	if c.Path != "/" || c.Domain != "" {
		t.Fatalf("cookie scope: %+v", c)
	}
}

func TestClearSessionCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	ClearSessionCookie(rr, false)
	c := rr.Result().Cookies()[0]
	if c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("cookie not cleared: %+v", c)
	}
	if c.Secure {
		t.Fatal("insecure request produced a Secure cookie")
	}
}

func TestSessionIDFromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if SessionID(r) != "" {
		t.Fatal("empty request had a session")
	}
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "abc"})
	if SessionID(r) != "abc" {
		t.Fatal("cookie not read")
	}
}
