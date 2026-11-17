package invite

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testPin() string {
	return base64.RawURLEncoding.EncodeToString(make([]byte, 32))
}

func testInvite(now time.Time) Invite {
	token, _, err := NewToken()
	if err != nil {
		panic(err)
	}
	return Invite{
		Version:   CurrentVersion,
		GroupID:   "group-1",
		GroupName: "Friday Night",
		HostName:  "Desynkro",
		Endpoint:  "home.example.net:51820",
		HostPin:   testPin(),
		Token:     token,
		PeerName:  "Ash",
		Expires:   now.Add(24 * time.Hour),
	}
}

func rawCode(t *testing.T, inv Invite) string {
	t.Helper()
	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	return codePrefix + base64.RawURLEncoding.EncodeToString(data)
}

func TestNewToken(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "LP1-") {
		t.Fatalf("token = %q", token)
	}
	if len(token) != 43 {
		t.Fatalf("token length = %d", len(token))
	}
	if len(hash) != 43 {
		t.Fatalf("hash length = %d", len(hash))
	}

	token2, _, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == token2 {
		t.Fatal("two tokens are identical")
	}
	if HashToken(token) != hash {
		t.Fatal("hash is not stable")
	}
}

func TestNormalizeToken(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	messy := " " + strings.ToLower(strings.ReplaceAll(token, "-", " ")) + "\n"
	if NormalizeToken(messy) != token {
		t.Fatalf("normalized = %q, want %q", NormalizeToken(messy), token)
	}
	if HashToken(messy) != hash {
		t.Fatal("the normalized token hashes differently")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	now := time.Now().UTC()
	inv := testInvite(now)
	code, err := Encode(inv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, codePrefix) {
		t.Fatalf("code = %q", code)
	}

	got, err := Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if got.GroupID != inv.GroupID || got.GroupName != inv.GroupName || got.HostName != inv.HostName {
		t.Fatalf("decoded = %+v", got)
	}
	if got.Endpoint != inv.Endpoint || got.HostPin != inv.HostPin || got.Token != inv.Token {
		t.Fatalf("decoded = %+v", got)
	}
	if got.PeerName != inv.PeerName || !got.Expires.Equal(inv.Expires) {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestEncodeRejectsExpired(t *testing.T) {
	inv := testInvite(time.Now().UTC())
	inv.Expires = time.Now().UTC().Add(-time.Hour)
	if _, err := Encode(inv); err == nil {
		t.Fatal("an expired invite was encoded")
	}
}

func TestDecodeExpired(t *testing.T) {
	now := time.Now().UTC()
	inv := testInvite(now)
	inv.Expires = now.Add(-time.Hour)
	if _, err := Decode(rawCode(t, inv)); err == nil {
		t.Fatal("an expired invite was decoded")
	}
}

func TestDecodeNormalizesCode(t *testing.T) {
	code, err := Encode(testInvite(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	messy := "  " + strings.ToUpper(code[:4]) + code[4:] + "\n"
	if _, err := Decode(messy); err != nil {
		t.Fatalf("messy code was rejected: %v", err)
	}
}

func TestDecodeRejectsBadCodes(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"lp1_",
		"lp1_!!!!",
	}
	for _, code := range cases {
		if _, err := Decode(code); err == nil {
			t.Errorf("Decode(%q) accepted a bad code", code)
		}
	}
}

func TestDecodeRejectsMissingFields(t *testing.T) {
	now := time.Now().UTC()
	inv := testInvite(now)
	inv.PeerName = ""
	if _, err := Decode(rawCode(t, inv)); err == nil {
		t.Fatal("an invite without a peer name was decoded")
	}

	inv = testInvite(now)
	inv.Endpoint = "home.example.net"
	if _, err := Decode(rawCode(t, inv)); err == nil {
		t.Fatal("an invite without an endpoint port was decoded")
	}

	inv = testInvite(now)
	inv.HostPin = "short"
	if _, err := Decode(rawCode(t, inv)); err == nil {
		t.Fatal("an invite with a bad pin was decoded")
	}
}

func TestInviteContainsNoPrivateKey(t *testing.T) {
	code, err := Encode(testInvite(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.RawURLEncoding.DecodeString(code[len(codePrefix):])
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	if strings.Contains(text, "private") || strings.Contains(text, "secret") {
		t.Fatalf("invite payload contains a secret field: %s", data)
	}
}

func TestValidateEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", "host", "host:0", "host:70000", ":51820"} {
		if err := validateEndpoint(endpoint); err == nil {
			t.Errorf("endpoint %q was accepted", endpoint)
		}
	}
}
