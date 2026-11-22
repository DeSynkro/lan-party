package webui

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestAddService(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.post("/groups/g1/services", url.Values{
		"name": {"Minecraft"}, "proto": {"tcp"}, "target": {"127.0.0.1:25565"},
		"ports": {"25565"}, "hint": {"Java edition"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if len(srv.fake.groups[0].Services) != 1 {
		t.Fatal("service not added")
	}
	got := srv.fake.groups[0].Services[0]
	if got.Name != "Minecraft" || got.Proto != "tcp" || len(got.Ports) != 1 || got.Ports[0] != 25565 {
		t.Fatalf("unexpected service: %+v", got)
	}
}

func TestServiceValidation(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.post("/groups/g1/services", url.Values{"name": {"Bad"}, "proto": {"icmp"}, "target": {"127.0.0.1"}, "ports": {"99999"}})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if len(srv.fake.groups[0].Services) != 0 {
		t.Fatal("invalid service added")
	}
}

func TestServicePresetPrefillsForm(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.get("/groups/g1/services/new?preset=minecraft-java")
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(body, `value="Minecraft: Java Edition"`) || !strings.Contains(body, `value="25565"`) {
		t.Fatal("preset not applied")
	}
}

func TestServiceFormDoesNotJoinLiveReloads(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Members: []group.Member{{ID: "m1", Name: "Ash"}},
	})
	body := srv.get("/groups/g1/services/new").Body.String()
	if strings.Contains(body, "data-group-id=") {
		t.Fatal("service form carries live group state and will reload in a loop")
	}
}

func TestServiceNewDefaultsToLoopback(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.get("/groups/g1/services/new")
	body := rr.Body.String()
	if !strings.Contains(body, `value="127.0.0.1"`) {
		t.Fatal("target does not default to a usable address")
	}
	if strings.Contains(body, `value="Server"`) {
		t.Fatal("target defaults to the device name")
	}
}

func TestServiceEditForm(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Terraria", Proto: "tcp", Target: "127.0.0.1", Ports: []int{7777}}},
	})
	rr := srv.get("/groups/g1/services/s1")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Terraria") {
		t.Fatalf("edit form: %d", rr.Code)
	}
	rr = srv.post("/groups/g1/services/s1", url.Values{"name": {"Terraria"}, "proto": {"tcp"}, "target": {"127.0.0.1"}, "ports": {"7778"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", rr.Code)
	}
	if len(srv.fake.groups[0].Services[0].Ports) != 1 || srv.fake.groups[0].Services[0].Ports[0] != 7778 {
		t.Fatal("service not updated")
	}
}

func TestServiceRemove(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Terraria", Proto: "tcp", Target: "127.0.0.1", Ports: []int{7777}}},
	})
	rr := srv.get("/groups/g1/services/s1/remove")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Remove") {
		t.Fatalf("confirmation: %d", rr.Code)
	}
	rr = srv.post("/groups/g1/services/s1/remove", url.Values{})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("remove: %d", rr.Code)
	}
	if len(srv.fake.groups[0].Services) != 0 {
		t.Fatal("service not removed")
	}
}

func TestServiceRemoveMissingIs409(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
	rr := srv.post("/groups/g1/services/nope/remove", url.Values{})
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d", rr.Code)
	}
}
