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
