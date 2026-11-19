package app

import (
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
)

func TestReconnectDelay(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		elapsed time.Duration
		want    time.Duration
		ok      bool
	}{
		{"first", 0, 0, time.Second, true},
		{"second", 1, 0, 2 * time.Second, true},
		{"third", 2, 0, 5 * time.Second, true},
		{"fourth", 3, 0, 10 * time.Second, true},
		{"fifth", 4, 0, 15 * time.Second, true},
		{"sixth", 5, 0, 30 * time.Second, true},
		{"fast cap", 6, 2 * time.Minute, 30 * time.Second, true},
		{"fast until ten minutes", 6, 9*time.Minute + 59*time.Second, 30 * time.Second, true},
		{"slow phase", 100, 10 * time.Minute, time.Minute, true},
		{"clamped to cutoff", 100, 29*time.Minute + 30*time.Second, 30 * time.Second, true},
		{"stopped at cutoff", 100, 30 * time.Minute, 0, false},
		{"stopped after cutoff", 100, 31 * time.Minute, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := reconnectDelay(tc.attempt, tc.elapsed)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("reconnectDelay(%d, %v) = %v, %t; want %v, %t", tc.attempt, tc.elapsed, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestAutoReconnectRespectsSetting(t *testing.T) {
	a := newTestApp(t)
	g := group.CreateFriendGroup("Friends", "Host", "127.0.0.1:1", "pin", "self", "You", nil)
	a.mu.Lock()
	a.groups = append(a.groups, g)
	a.liveGroupID = g.ID
	a.settings.AutoReconnect = false
	a.startAutoReconnectLocked(g.ID, time.Now())
	state := a.autoReconnect
	a.mu.Unlock()
	if state != nil {
		t.Fatal("auto-reconnect started while the setting was off")
	}
}

func TestStopSessionStopsAutoReconnect(t *testing.T) {
	a := newTestApp(t)
	cancelled := false
	a.mu.Lock()
	a.autoReconnect = &reconnectState{groupID: "group", cancel: func() { cancelled = true }}
	epoch := a.sessionEpoch
	a.mu.Unlock()

	if err := a.StopSession(); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("the retry loop was not cancelled")
	}
	a.mu.Lock()
	state := a.autoReconnect
	after := a.sessionEpoch
	a.mu.Unlock()
	if state != nil {
		t.Fatal("the retry state survived StopSession")
	}
	if after != epoch+1 {
		t.Fatalf("session epoch = %d, want %d", after, epoch+1)
	}
}

func TestDeleteGroupStopsAutoReconnect(t *testing.T) {
	a := newTestApp(t)
	g := group.CreateFriendGroup("Friends", "Host", "127.0.0.1:1", "pin", "self", "You", nil)
	cancelled := false
	a.mu.Lock()
	a.groups = append(a.groups, g)
	a.autoReconnect = &reconnectState{groupID: g.ID, cancel: func() { cancelled = true }}
	a.mu.Unlock()

	if err := a.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("the retry loop was not cancelled")
	}
	a.mu.Lock()
	state := a.autoReconnect
	a.mu.Unlock()
	if state != nil {
		t.Fatal("the retry state survived DeleteGroup")
	}
}

func TestSaveSettingsStopsAutoReconnect(t *testing.T) {
	a := newTestApp(t)
	cancelled := false
	a.mu.Lock()
	a.autoReconnect = &reconnectState{groupID: "group", cancel: func() { cancelled = true }}
	a.mu.Unlock()

	settings := a.Settings()
	settings.AutoReconnect = false
	if err := a.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("the retry loop was not cancelled")
	}
	a.mu.Lock()
	state := a.autoReconnect
	a.mu.Unlock()
	if state != nil {
		t.Fatal("the retry state survived disabling the setting")
	}
}

func TestReconnectStatusReportsStopReason(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	state := &reconnectState{groupID: "group", cancel: func() {}}
	a.autoReconnect = state
	a.finishAutoReconnectLocked(state, "the host denied the join")
	a.mu.Unlock()

	status := a.ReconnectStatus("group")
	if !status.Stopped || status.Reason != "the host denied the join" {
		t.Fatalf("status = %+v", status)
	}
}
