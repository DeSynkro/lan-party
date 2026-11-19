package gtkui

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/secure"
)

func TestInviteDialogStateFor(t *testing.T) {
	cases := []struct {
		name       string
		liveID     string
		groupID    string
		liveName   string
		wantWarn   string
		wantAction string
		wantBlock  bool
	}{
		{
			name: "this group live", liveID: "g1", groupID: "g1",
			wantAction: "Create invite",
		},
		{
			name: "session off", liveID: "", groupID: "g1",
			wantWarn:   "Session is off. It will start when you create this invite.",
			wantAction: "Start session and invite",
		},
		{
			name: "other group live", liveID: "g2", groupID: "g1", liveName: "Friday Night",
			wantWarn:   "The session for Friday Night is live. End it before inviting from this group.",
			wantAction: "Create invite", wantBlock: true,
		},
		{
			name: "other group unnamed", liveID: "g2", groupID: "g1",
			wantWarn:   "The session for another group is live. End it before inviting from this group.",
			wantAction: "Create invite", wantBlock: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inviteDialogStateFor(tc.liveID, tc.groupID, tc.liveName)
			if got.Warning != tc.wantWarn || got.Action != tc.wantAction || got.Blocked != tc.wantBlock {
				t.Fatalf("state = %+v, want warning %q, action %q, blocked %t", got, tc.wantWarn, tc.wantAction, tc.wantBlock)
			}
		})
	}
}

func TestEnsureInviteSessionStartsThisGroupOnly(t *testing.T) {
	state := newInviteTestApp(t)
	settings := state.Settings()
	settings.AppPort = freeInviteUDPPort(t)
	if err := state.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	first, err := state.CreateGroup("First", "Host")
	if err != nil {
		t.Fatal(err)
	}
	second, err := state.CreateGroup("Second", "Host")
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureInviteSession(state, first.ID); err != nil {
		t.Fatal(err)
	}
	if got := state.LiveGroupID(); got != first.ID {
		t.Fatalf("live group = %q, want %q", got, first.ID)
	}

	err = ensureInviteSession(state, second.ID)
	if err == nil {
		t.Fatal("a second group's session was started")
	}
	if !strings.Contains(err.Error(), "First") {
		t.Fatalf("error = %q, want the live group name", err)
	}
	if got := state.LiveGroupID(); got != first.ID {
		t.Fatalf("live group changed to %q", got)
	}
}

func TestCreateInviteDoesNotCreateWhenSessionStartFails(t *testing.T) {
	state := newInviteTestApp(t)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	settings := state.Settings()
	settings.PublicAddress = "home.example.net"
	settings.AppPort = conn.LocalAddr().(*net.UDPAddr).Port
	if err := state.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	g, err := state.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}

	u := &probeUI{state: state}
	if _, err := u.createInvite(g, "Ash"); err == nil {
		t.Fatal("an invite was created without a session")
	}
	if got := state.LiveGroupID(); got != "" {
		t.Fatalf("live group = %q, want no session", got)
	}
	saved, ok := state.Group(g.ID)
	if !ok {
		t.Fatal("group disappeared")
	}
	if len(saved.OpenInvites(time.Now())) != 0 {
		t.Fatal("an invite was created after the session start failed")
	}
}

func newInviteTestApp(t *testing.T) *apppkg.App {
	t.Helper()
	base := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(base, "config", "lan-party"),
		DataDir:   filepath.Join(base, "data", "lan-party"),
	}
	state, err := apppkg.NewWithStore(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })
	return state
}

func freeInviteUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
