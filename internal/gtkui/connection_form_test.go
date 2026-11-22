package gtkui

import (
	"strings"
	"testing"
	"time"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
)

func TestConnectionEndpointNote(t *testing.T) {
	if got := connectionEndpointNote(true, false, 1, false); !strings.Contains(got, "cancels 1 open invite") {
		t.Fatalf("note = %q", got)
	}
	if got := connectionEndpointNote(true, true, 2, true); !strings.Contains(got, "cancels 2 open invites") || !strings.Contains(got, "after the restart") {
		t.Fatalf("note = %q", got)
	}
	if got := connectionEndpointNote(true, false, 0, false); got != "" {
		t.Fatalf("note = %q", got)
	}
	if got := connectionEndpointNote(false, true, 0, false); !strings.Contains(got, "router forward") {
		t.Fatalf("note = %q", got)
	}
	if got := connectionEndpointNote(false, false, 0, false); got != "" {
		t.Fatalf("note = %q", got)
	}
}

func TestConnectionChangeNoteForRelay(t *testing.T) {
	got := connectionChangeNote(true, true, true, 2, true)
	if !strings.Contains(got, "relay changes") || !strings.Contains(got, "cancels 2 open invites") || !strings.Contains(got, "after the restart") {
		t.Fatalf("note = %q", got)
	}
	if got := connectionChangeNote(true, false, false, 0, false); got != "" {
		t.Fatalf("note = %q", got)
	}
}

func TestLiveConnectionChange(t *testing.T) {
	direct := apppkg.GroupConnection{PublicAddress: "home.example.net", AppPort: 51820}
	sameAddress := direct
	sameAddress.PublicAddress = "other.example.net"
	if liveConnectionChange(direct, sameAddress) {
		t.Fatal("an address-only change asked for a restart")
	}
	newPort := direct
	newPort.AppPort = 51821
	if !liveConnectionChange(direct, newPort) {
		t.Fatal("a direct port change did not ask for a restart")
	}

	relay := apppkg.GroupConnection{UseRelay: true, RelayHost: "relay.example.net", RelayPort: 7443, RelayID: strings.Repeat("ab", 32)}
	sameRelay := relay
	if liveConnectionChange(relay, sameRelay) {
		t.Fatal("an unchanged relay asked for a restart")
	}
	newRelay := relay
	newRelay.RelayHost = "other.example.net"
	if !liveConnectionChange(relay, newRelay) {
		t.Fatal("a relay host change did not ask for a restart")
	}
	if !liveConnectionChange(direct, relay) {
		t.Fatal("switching to the relay did not ask for a restart")
	}
}

func TestFriendOfflineServiceNote(t *testing.T) {
	if got := friendOfflineServiceNote(false, false, false); !strings.Contains(got, "Reconnect when the host is back") {
		t.Fatalf("direct note = %q", got)
	}
	if got := friendOfflineServiceNote(true, false, false); !strings.Contains(got, "new invite") {
		t.Fatalf("no-token note = %q", got)
	}
	if got := friendOfflineServiceNote(true, true, true); !strings.Contains(got, "Reconnecting through the relay") {
		t.Fatalf("reconnecting note = %q", got)
	}
	if got := friendOfflineServiceNote(true, true, false); !strings.Contains(got, "Reconnect when the host is back") {
		t.Fatalf("idle note = %q", got)
	}
}

func TestFriendForgetText(t *testing.T) {
	if got := friendForgetText(group.Group{}); got != "" {
		t.Fatalf("note = %q", got)
	}
	g := group.Group{ForgetAfterDays: 5, LastConnected: time.Now().Add(-2 * 24 * time.Hour)}
	if got := friendForgetText(g); !strings.Contains(got, "3 days") {
		t.Fatalf("note = %q", got)
	}
	g.LastConnected = time.Now().Add(-6 * 24 * time.Hour)
	if got := friendForgetText(g); !strings.Contains(got, "ended") {
		t.Fatalf("expired note = %q", got)
	}
	g.LastConnected = time.Now().Add(-5*24*time.Hour + time.Hour)
	if got := friendForgetText(g); !strings.Contains(got, "less than a day") {
		t.Fatalf("last-day note = %q", got)
	}
}

func TestConnectionDialogHeight(t *testing.T) {
	if got := connectionDialogHeight(false, false, false); got != 330 {
		t.Fatalf("inherited height = %d", got)
	}
	if got := connectionDialogHeight(true, false, false); got != 480 {
		t.Fatalf("direct height = %d", got)
	}
	if got := connectionDialogHeight(true, true, false); got != 520 {
		t.Fatalf("inherited relay height = %d", got)
	}
	if got := connectionDialogHeight(true, true, true); got != 620 {
		t.Fatalf("custom relay height = %d", got)
	}
}

func TestShortRelayID(t *testing.T) {
	id := strings.Repeat("ab", 32)
	if got := shortRelayID(id); got != id[:16]+"…" {
		t.Fatalf("short ID = %q", got)
	}
	if got := shortRelayID("short"); got != "short" {
		t.Fatalf("short ID = %q", got)
	}
	route := relayRoute("relay.example.net", 7443, id)
	if !strings.Contains(route, "relay.example.net") || !strings.Contains(route, "port 7443") || !strings.Contains(route, "Relay ID abab") {
		t.Fatalf("route = %q", route)
	}
}
