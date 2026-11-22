package webui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

func TestGroupViewHidesFriendAndRelayFields(t *testing.T) {
	g := group.Group{
		ID: "g1", Name: "Test", Role: group.RoleHost,
		UseRelay: true, RelayHost: "secret.example", RelayID: strings.Repeat("a", 64),
		HostPin: "pin", HostGroupID: "host-group",
	}
	v := NewGroupView(g, nil, true, 0, "192.168.1.20:51820")
	if v.Name != "Test" || !v.Live || v.Endpoint != "192.168.1.20:51820" {
		t.Fatalf("unexpected view: %+v", v)
	}
	typ := reflect.TypeOf(v)
	for _, forbidden := range []string{"UseRelay", "RelayHost", "RelayPort", "RelayID", "RelayCustom", "HostPin", "HostGroupID", "Role"} {
		if _, ok := typ.FieldByName(forbidden); ok {
			t.Fatalf("view exposes %s", forbidden)
		}
	}
}

func TestGroupViewCountsConnectedMembers(t *testing.T) {
	g := group.Group{ID: "g1", Name: "Test", Role: group.RoleHost,
		Members: []group.Member{{ID: "m1"}, {ID: "m2"}},
	}
	presence := map[string]session.Presence{"m1": {Live: true}, "m2": {Live: false}}
	v := NewGroupView(g, presence, true, 0, "")
	if v.Connected != 1 || v.Members != 2 {
		t.Fatalf("counts: connected=%d members=%d", v.Connected, v.Members)
	}
}

func TestMemberViewHidesKeyMaterial(t *testing.T) {
	m := group.Member{
		ID: "m1", Name: "Ash", DeviceName: "Laptop", PublicKey: "secret-key",
		Fingerprint: "ABCD", LastAddress: "10.0.0.5", State: group.MemberApproved,
	}
	v := NewMemberView(m, nil)
	if !v.Approved || v.Fingerprint != "ABCD" || v.Address != "10.0.0.5" {
		t.Fatalf("unexpected view: %+v", v)
	}
	typ := reflect.TypeOf(v)
	for _, forbidden := range []string{"PublicKey", "RelayReconnectToken", "RelayToken"} {
		if _, ok := typ.FieldByName(forbidden); ok {
			t.Fatalf("view exposes %s", forbidden)
		}
	}
}

func TestInviteViewHidesTokenHash(t *testing.T) {
	inv := group.Invite{ID: "i1", Name: "Ash", TokenHash: "secret-hash", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	v := NewInviteView(inv, "code", time.Now())
	if !v.Open || v.Code != "code" {
		t.Fatalf("unexpected view: %+v", v)
	}
	typ := reflect.TypeOf(v)
	for _, forbidden := range []string{"TokenHash", "Relay"} {
		if _, ok := typ.FieldByName(forbidden); ok {
			t.Fatalf("view exposes %s", forbidden)
		}
	}
}

func TestServiceViewPortsText(t *testing.T) {
	v := NewServiceView(group.Service{ID: "s1", Name: "Game", Proto: "tcp", Target: "127.0.0.1:25565", Ports: []int{25565, 25566}})
	if v.PortsText != "25565, 25566" {
		t.Fatalf("ports text = %q", v.PortsText)
	}
}

func TestErrorViewCarriesStatus(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.get("/does-not-exist")
	if rr.Code != 404 || !strings.Contains(rr.Body.String(), "Error 404") {
		t.Fatalf("error page: %d", rr.Code)
	}
}
