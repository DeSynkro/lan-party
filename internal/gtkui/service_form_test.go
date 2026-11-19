package gtkui

import (
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestServiceWarningText(t *testing.T) {
	if got := serviceWarningText("127.0.0.1", "25565", "", "tcp"); !strings.Contains(got, "on this PC") {
		t.Fatalf("loopback warning = %q", got)
	}
	if got := serviceWarningText("192.168.0.10", "22", "", "tcp"); !strings.Contains(got, "administrative") {
		t.Fatalf("admin port warning = %q", got)
	}
	if got := serviceWarningText("192.168.0.10", "25565", "", "tcp"); got != "" {
		t.Fatalf("normal service warning = %q", got)
	}
}

func TestServiceRoutePreviewRangeWithFriendPort(t *testing.T) {
	got := serviceRoutePreview("192.168.0.10", "2456-2458", "25565")
	want := "Leave the friend port blank to use the game port range."
	if got != want {
		t.Fatalf("preview = %q, want %q", got, want)
	}
}

func TestServiceRoutePreview(t *testing.T) {
	cases := []struct {
		name   string
		ip     string
		game   string
		friend string
		want   string
	}{
		{
			name: "same single port",
			ip:   "192.168.0.10", game: "25565",
			want: "Friends open 127.0.0.1:25565 → game server 192.168.0.10:25565",
		},
		{
			name: "remapped single port",
			ip:   "192.168.0.10", game: "25566", friend: "25565",
			want: "Friends open 127.0.0.1:25565 → game server 192.168.0.10:25566",
		},
		{
			name: "same range",
			ip:   "192.168.0.10", game: "2456-2458",
			want: "Friends open 127.0.0.1:2456-2458 → game server 192.168.0.10:2456-2458",
		},
		{
			name: "same list",
			ip:   "192.168.0.10", game: "2456,2600",
			want: "Friends open 127.0.0.1:2456,2600 → game server 192.168.0.10:2456,2600",
		},
		{
			name: "loopback game server",
			ip:   "127.0.0.1", game: "25565",
			want: "Friends open 127.0.0.1:25565 → game server 127.0.0.1:25565",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceRoutePreview(tc.ip, tc.game, tc.friend); got != tc.want {
				t.Fatalf("preview = %q, want %q", got, tc.want)
			}
		})
	}

	invalid := []struct {
		name   string
		ip     string
		game   string
		friend string
	}{
		{"empty ip", "", "25565", ""},
		{"empty game ports", "192.168.0.10", "", ""},
		{"range with one friend port", "192.168.0.10", "2456-2458", "25565"},
		{"ip contains a port", "192.168.0.10:25565", "25565", ""},
		{"invalid friend port", "192.168.0.10", "25565", "0"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceRoutePreview(tc.ip, tc.game, tc.friend); strings.Contains(got, "Friends open") {
				t.Fatalf("preview accepted invalid input: %q", got)
			}
		})
	}
}

func TestServiceFormValues(t *testing.T) {
	cases := []struct {
		name       string
		service    group.Service
		wantIP     string
		wantGame   string
		wantFriend string
	}{
		{
			name:     "same port",
			service:  group.Service{Target: "192.168.1.50", Ports: []int{25565}},
			wantIP:   "192.168.1.50",
			wantGame: "25565",
		},
		{
			name:       "remapped port",
			service:    group.Service{Target: "192.168.1.50:25566", Ports: []int{25565}},
			wantIP:     "192.168.1.50",
			wantGame:   "25566",
			wantFriend: "25565",
		},
		{
			name:     "range",
			service:  group.Service{Target: "192.168.1.50", Ports: []int{2456, 2457, 2458}},
			wantIP:   "192.168.1.50",
			wantGame: "2456-2458",
		},
		{
			name:     "localhost",
			service:  group.Service{Target: "localhost", Ports: []int{25565}},
			wantIP:   "127.0.0.1",
			wantGame: "25565",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip, game, friend, _, err := serviceFormValues(tc.service)
			if err != nil {
				t.Fatal(err)
			}
			if ip != tc.wantIP || game != tc.wantGame || friend != tc.wantFriend {
				t.Fatalf("serviceFormValues = %q, %q, %q; want %q, %q, %q", ip, game, friend, tc.wantIP, tc.wantGame, tc.wantFriend)
			}
		})
	}
	if _, _, _, _, err := serviceFormValues(group.Service{Target: "example.com", Ports: []int{25565}}); err == nil {
		t.Fatal("an invalid target was accepted")
	}
}

func TestServiceTargetHelpFor(t *testing.T) {
	got := serviceTargetHelpFor("192.168.1.50")
	if !strings.Contains(got, "Use 127.0.0.1") || !strings.Contains(got, "192.168.1.50") {
		t.Fatalf("help = %q", got)
	}
	if strings.Contains(serviceTargetHelpFor("this PC"), "this PC: this PC") {
		t.Fatal("the fallback host was repeated")
	}
}
