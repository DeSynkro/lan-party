package webui

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

func TestSnapshotIncludesOpenInviteIDs(t *testing.T) {
	fake := newFakeApp()
	fake.groups = []group.Group{{ID: "g1", Role: group.RoleHost,
		Invites: []group.Invite{
			{ID: "i1", TokenHash: "h1", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
			{ID: "i2", TokenHash: "h2", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), RedeemedAt: time.Now()},
		},
	}}
	snap := NewSnapshot(fake)
	if len(snap.Groups) != 1 || len(snap.Groups[0].Invites) != 1 || snap.Groups[0].Invites[0] != "i1" {
		t.Fatalf("unexpected snapshot invites: %+v", snap.Groups)
	}
}

func TestSnapshotChangesWhenPresenceChanges(t *testing.T) {
	fake := newFakeApp()
	s := NewSnapshot(fake)
	first := s.Hash()
	fake.presence = map[string]session.Presence{"m1": {Live: true}}
	second := NewSnapshot(fake).Hash()
	if first == second {
		t.Fatal("snapshot did not change")
	}
}

func TestSnapshotStableWithoutChanges(t *testing.T) {
	fake := newFakeApp()
	if NewSnapshot(fake).Hash() != NewSnapshot(fake).Hash() {
		t.Fatal("snapshot hash is not stable")
	}
}

func TestSnapshotCountsConnected(t *testing.T) {
	fake := newFakeApp()
	fake.groups = []group.Group{{ID: "g1", Role: group.RoleHost, Members: []group.Member{{ID: "m1"}, {ID: "m2"}}}}
	fake.live = "g1"
	fake.presence = map[string]session.Presence{"m1": {Live: true}, "m2": {Live: false}}
	snap := NewSnapshot(fake)
	if len(snap.Groups) != 1 || snap.Groups[0].Connected != 1 || !snap.Groups[0].Live {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestEventsStreamsInitialSnapshot(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "http://lan.local/events", nil).WithContext(ctx)
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(rr, req)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("events handler did not stop")
	}
	scanner := bufio.NewScanner(strings.NewReader(rr.Body.String()))
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "data: ") {
		t.Fatalf("no data event: %q", rr.Body.String())
	}
}

func TestEventsRequireAuth(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/events", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestEventsCloseWhenSessionIsDestroyed(t *testing.T) {
	srv := newTestServer(t)
	srv.eventInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "http://lan.local/events", nil).WithContext(ctx)
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(rr, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	srv.sessions.Destroy(srv.session.ID)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event stream survived session destruction")
	}
}

func TestEventsCloseOnServerShutdown(t *testing.T) {
	srv := newTestServer(t)
	srv.eventInterval = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "http://lan.local/events", nil).WithContext(ctx)
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: srv.session.ID})
	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(rr, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	srv.closeStreams()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event stream survived server shutdown")
	}
}

func TestSessionAliveDoesNotTouch(t *testing.T) {
	store := NewSessionStore(20*time.Millisecond, time.Hour)
	session, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if !store.Alive(session.ID) {
		t.Fatal("live session reported dead")
	}
	time.Sleep(15 * time.Millisecond)
	if _, ok := store.Get(session.ID); ok {
		t.Fatal("Alive extended the idle timeout")
	}
	if store.Alive("missing") {
		t.Fatal("missing session reported alive")
	}
}
