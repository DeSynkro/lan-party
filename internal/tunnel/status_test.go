package tunnel

import (
	"encoding/hex"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestParseStatus(t *testing.T) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	pub := key.PublicKey()
	uapi := "public_key=" + hex.EncodeToString(pub[:]) + "\n" +
		"last_handshake_time_sec=1763000000\n" +
		"last_handshake_time_nsec=0\n" +
		"rx_bytes=1024\n" +
		"tx_bytes=2048\n"

	peers := parseStatus(uapi)
	if len(peers) != 1 {
		t.Fatalf("got %d peers, want 1", len(peers))
	}
	peer := peers[0]
	if peer.PublicKey != key.PublicKey() {
		t.Fatal("public key did not survive")
	}
	if !peer.LastHandshake.Equal(time.Unix(1763000000, 0)) {
		t.Fatalf("last handshake = %v", peer.LastHandshake)
	}
	if peer.RxBytes != 1024 || peer.TxBytes != 2048 {
		t.Fatalf("counters = %d/%d", peer.RxBytes, peer.TxBytes)
	}
}

func TestParseStatusIgnoresNoise(t *testing.T) {
	uapi := "private_key=abc\n" +
		"listen_port=51820\n" +
		"public_key=not-hex\n" +
		"last_handshake_time_sec=5\n"

	if peers := parseStatus(uapi); len(peers) != 0 {
		t.Fatalf("got %d peers, want 0", len(peers))
	}
}
