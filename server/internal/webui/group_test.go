package webui

import (
	"html"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
)

func TestGroupPageShowsSessionAndServices(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{
		ID: "g1", Name: "Game night", Role: group.RoleHost, HostName: "Server",
		Services: []group.Service{{ID: "s1", Name: "Minecraft", Proto: "tcp", Target: "127.0.0.1:25565", Ports: []int{25565}}},
	})
	rr := srv.get("/groups/g1")
	body := rr.Body.String()
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(body, "Game night") || !strings.Contains(body, "Minecraft") {
		t.Fatal("missing content")
	}
	if !strings.Contains(body, "Session off") {
		t.Fatal("missing session state")
	}
	if !strings.Contains(body, "managed by this deployment") {
		t.Fatal("missing app port note")
	}
}

func TestRedeemedInviteDoesNotShowCode(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Invites: []group.Invite{{ID: "i1", Name: "Ash", TokenHash: "hash", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour), RedeemedAt: time.Now()}},
	})
	srv.fake.lastInvite = "lp1_secret-code"
	rr := srv.get("/groups/g1")
	body := rr.Body.String()
	if strings.Contains(body, "lp1_secret-code") {
		t.Fatal("redeemed invite still shows its code")
	}
	if strings.Contains(body, "Used") {
		t.Fatal("redeemed invite is still listed")
	}
}

func TestSettingsExplainsApprovalSemantics(t *testing.T) {
	srv := newTestServer(t)
	body := srv.get("/settings").Body.String()
	if !strings.Contains(body, "Ask before a new device joins") {
		t.Fatal("approval setting is not explained")
	}
	if strings.Contains(body, "Approve new devices automatically") {
		t.Fatal("approval setting still claims automatic approval")
	}
}

func TestGroupPagePointsAtTheEnvFile(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	body := srv.get("/groups/g1").Body.String()
	if strings.Contains(body, "docker-compose.yml") {
		t.Fatal("group page names a file that does not exist")
	}
	if !strings.Contains(body, ".env") || !strings.Contains(body, "setup.sh --ports") {
		t.Fatal("group page does not say where to change the port")
	}
}

func TestGroupPageExposesStructureForLiveUpdates(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Members: []group.Member{{ID: "m1", Name: "Ash"}},
		Invites: []group.Invite{
			{ID: "i1", Name: "Open", TokenHash: "hash", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
			{ID: "i2", Name: "Used", TokenHash: "hash", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), RedeemedAt: time.Now()},
		},
	})
	body := srv.get("/groups/g1").Body.String()
	for _, want := range []string{`data-group-id="g1"`, `data-members="m1"`, `data-invites="i1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("group page is missing %s", want)
		}
	}
	if strings.Contains(body, `data-invites="i1,i2"`) {
		t.Fatal("redeemed invite is in the live structure signature")
	}
}

func TestDockerFirewallHintOnGroupPage(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Minecraft", Proto: "tcp", Target: "192.168.0.10", Ports: []int{25565}}},
	})
	srv.cfg.InContainer = true
	body := srv.get("/groups/g1").Body.String()
	for _, want := range []string{
		"Docker firewall reminder",
		"sudo ufw allow from 172.16.0.0/12 to any port 25565 proto tcp",
		"sudo ufw delete allow from 172.16.0.0/12 to any port 25565 proto tcp",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("group page is missing %q", want)
		}
	}

	srv.cfg.InContainer = false
	if strings.Contains(srv.get("/groups/g1").Body.String(), "Docker firewall reminder") {
		t.Fatal("Docker hint shown outside a container")
	}
}

func TestDockerFirewallCommandsAreCombined(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{
			{ID: "s1", Name: "Minecraft", Proto: "tcp", Target: "192.168.0.10", Ports: []int{25565}},
			{ID: "s2", Name: "Valheim", Proto: "udp", Target: "192.168.0.10", Ports: []int{2456, 2457}},
		},
	})
	srv.cfg.InContainer = true
	body := html.UnescapeString(srv.get("/groups/g1").Body.String())
	if strings.Count(body, `id="docker-allow"`) != 1 || strings.Count(body, `id="docker-delete"`) != 1 {
		t.Fatal("commands are split across multiple blocks")
	}
	for _, want := range []string{
		"comment 'LAN Party -> Minecraft'",
		"comment 'LAN Party -> Valheim'",
		"to any port 2456 proto udp",
		"to any port 2457 proto udp",
		"# LAN Party -> Minecraft",
		"# LAN Party -> Valheim",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("combined block is missing %q", want)
		}
	}
}

func TestDockerHintOnServiceForm(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	srv.cfg.InContainer = true
	body := srv.get("/groups/g1/services/new").Body.String()
	if !strings.Contains(body, "means the container itself") {
		t.Fatal("service form does not warn about 127.0.0.1 in Docker")
	}
	if !strings.Contains(body, "sudo ufw allow from 172.16.0.0/12") || !strings.Contains(body, "sudo ufw delete allow from 172.16.0.0/12") {
		t.Fatal("service form is missing the add/delete firewall commands")
	}

	srv.cfg.InContainer = false
	if strings.Contains(srv.get("/groups/g1/services/new").Body.String(), "Docker firewall reminder") {
		t.Fatal("Docker hint shown outside a container")
	}
}

func TestGroupPageMissingIs404(t *testing.T) {
	srv := newTestServer(t)
	if rr := srv.get("/groups/nope"); rr.Code != 404 {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGroupPageHidesFriendGroup(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "f1", Name: "Friend group", Role: group.RoleFriend})
	if rr := srv.get("/groups/f1"); rr.Code != 404 {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestGroupPageShowsInviteCode(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	srv.fake.lastInvite = "invite-code-1"
	srv.fake.groups[0].Invites = append(srv.fake.groups[0].Invites, group.Invite{ID: "i1", Name: "Ash", TokenHash: "hash", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	rr := srv.get("/groups/g1")
	body := rr.Body.String()
	if !strings.Contains(body, "invite-code-1") {
		t.Fatal("invite code missing")
	}
	if !strings.Contains(body, `data-copy="invite-i1"`) {
		t.Fatal("copy control missing")
	}
}
