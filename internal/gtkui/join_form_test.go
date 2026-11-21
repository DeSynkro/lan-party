package gtkui

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/relay"
)

func TestJoinInvitePreview(t *testing.T) {
	token, _, err := invite.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	code, err := invite.Encode(invite.Invite{
		Version:   invite.CurrentVersion,
		GroupID:   "group-1",
		GroupName: "Friday Night",
		HostName:  "Desynkro",
		Endpoint:  "127.0.0.1:51820",
		HostPin:   base64.RawURLEncoding.EncodeToString(make([]byte, sha256.Size)),
		Token:     token,
		PeerName:  "Ash",
		Expires:   time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	nameLine, groupLine := joinInvitePreview(code)
	if nameLine != "The host has assigned you the name Ash." {
		t.Fatalf("name line = %q", nameLine)
	}
	if groupLine != "Joining Friday Night · hosted by Desynkro." {
		t.Fatalf("group line = %q", groupLine)
	}

	if name, group := joinInvitePreview("not an invite"); name != "" || group != "" {
		t.Fatalf("invalid preview = %q, %q", name, group)
	}
}

func TestJoinRelayInvitePreview(t *testing.T) {
	token, _, err := invite.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	_, relayToken, err := relay.NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	code, err := invite.Encode(invite.Invite{
		Version:    invite.RelayVersion,
		GroupID:    "group-1",
		GroupName:  "Friday Night",
		HostName:   "Desynkro",
		HostPin:    base64.RawURLEncoding.EncodeToString(make([]byte, sha256.Size)),
		Token:      token,
		RelayHost:  "relay.example.net",
		RelayPort:  7443,
		RelayID:    strings.Repeat("ab", 32),
		RelayToken: relayToken,
		PeerName:   "Ash",
		Expires:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	nameLine, groupLine := joinInvitePreview(code)
	if nameLine != "The host has assigned you the name Ash." {
		t.Fatalf("name line = %q", nameLine)
	}
	if groupLine != "Joining Friday Night · hosted by Desynkro. · via relay" {
		t.Fatalf("group line = %q", groupLine)
	}
}
