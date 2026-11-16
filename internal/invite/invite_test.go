package invite

import (
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func sample(t *testing.T) Invite {
	t.Helper()

	host, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	peer, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	return Invite{
		Version:   1,
		GroupID:   "abc123",
		GroupName: "Friday Night",
		HostName:  "Desynkro",
		Endpoint:  "lanparty.example.com:51820",
		HostKey:   host.PublicKey().String(),
		PeerName:  "Ash",
		PeerKey:   peer.String(),
		PeerBox:   "100.64.0.2",
		Expires:   time.Now().Add(24 * time.Hour),
		Services: []Service{
			{Name: "Minecraft", Proto: "tcp", Ports: []int{25565}, Hint: "Direct Connect"},
		},
	}
}

func TestRoundTrip(t *testing.T) {
	inv := sample(t)

	code, err := Encode(inv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, Prefix) {
		t.Fatalf("code does not start with %q", Prefix)
	}

	got, err := Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if got.GroupName != inv.GroupName || got.PeerName != inv.PeerName {
		t.Fatalf("got %+v, want %+v", got, inv)
	}
	if len(got.Services) != 1 || got.Services[0].Ports[0] != 25565 {
		t.Fatalf("services did not survive: %+v", got.Services)
	}

	host, peer, err := got.Keys()
	if err != nil {
		t.Fatal(err)
	}
	if host.String() != inv.HostKey || peer.String() != inv.PeerKey {
		t.Fatal("keys did not survive the round trip")
	}
}

func TestDecodeRejects(t *testing.T) {
	valid := sample(t)

	expired := valid
	expired.Expires = time.Now().Add(-time.Minute)

	cases := map[string]string{
		"empty":         "",
		"not an invite": "hello",
		"damaged":       Prefix + "not-base-64!",
	}

	for name, code := range cases {
		if _, err := Decode(code); err == nil {
			t.Errorf("%s: decoded anyway", name)
		}
	}

	code, err := Encode(expired)
	if err != nil {
		t.Fatalf("an expired invite should still encode: %v", err)
	}
	if _, err := Decode(code); err == nil {
		t.Fatal("an expired invite was accepted")
	}
}

func TestEncodeRejectsIncomplete(t *testing.T) {
	inv := sample(t)
	inv.PeerBox = ""
	if _, err := Encode(inv); err == nil {
		t.Fatal("encoded an invite with no tunnel address")
	}

	inv = sample(t)
	inv.Endpoint = "nonsense"
	if _, err := Encode(inv); err == nil {
		t.Fatal("encoded an invite with a bad endpoint")
	}

	inv = sample(t)
	inv.HostKey = "not-a-key"
	if _, err := Encode(inv); err == nil {
		t.Fatal("encoded an invite with a bad host key")
	}
}
