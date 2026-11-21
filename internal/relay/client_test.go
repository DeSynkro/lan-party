package relay

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestRelayClientAuthorizeAndFriendEvent(t *testing.T) {
	relay := startTestRelay(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := DialClient(ctx, ClientConfig{
		Address:     relay.address,
		Fingerprint: relay.fingerprint,
		Password:    testRelayPassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.SessionID == "" || len(client.DataToken) != tokenLength {
		t.Fatalf("session = %q token length = %d", client.SessionID, len(client.DataToken))
	}

	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Authorize(ctx, token, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	friend := newTestFriend(t, relay)
	sendFriend(t, friend, token, []byte("hello"))
	select {
	case event := <-client.Events():
		if event.Type != "friend" || event.FriendID == 0 {
			t.Fatalf("friend event = %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("relay did not report the friend")
	}
	if err := client.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRelayClientRejectsWrongPassword(t *testing.T) {
	relay := startTestRelay(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DialClient(ctx, ClientConfig{
		Address:     relay.address,
		Fingerprint: relay.fingerprint,
		Password:    "not the password",
	})
	if err == nil {
		t.Fatal("wrong relay password was accepted")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Fatalf("error = %v", err)
	}
}

func TestRelayClientReusableHash(t *testing.T) {
	relay := startTestRelay(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := DialClient(ctx, ClientConfig{
		Address:     relay.address,
		Fingerprint: relay.fingerprint,
		Password:    testRelayPassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	raw, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	hash := TokenHash(raw)
	if err := client.AuthorizeHash(ctx, hex.EncodeToString(hash[:]), true, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := client.RevokeHash(ctx, hex.EncodeToString(hash[:])); err != nil {
		t.Fatal(err)
	}

	friend := newTestFriend(t, relay)
	sendFriend(t, friend, token, []byte("revoked"))
	select {
	case event := <-client.Events():
		t.Fatalf("revoked token produced event %+v", event)
	case <-time.After(300 * time.Millisecond):
	}
}
