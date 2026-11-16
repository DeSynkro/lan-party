package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type pair struct {
	host   *Device
	friend *Device
}

// startPair brings up two devices talking over the loopback interface. Inside
// the tunnel they are 100.64.0.1 and 100.64.0.2, exactly like a real session.
func startPair(t *testing.T) *pair {
	t.Helper()

	hostPort := freeUDPPort(t)
	friendPort := freeUDPPort(t)

	hostKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	friendKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	host, err := Start(Config{
		PrivateKey: hostKey,
		ListenPort: hostPort,
		Addresses:  []net.IPNet{mustCIDR(t, "100.64.0.1/32")},
		Peers: []Peer{{
			PublicKey:  friendKey.PublicKey(),
			Endpoint:   loopback(friendPort),
			AllowedIPs: []net.IPNet{mustCIDR(t, "100.64.0.2/32")},
		}},
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	t.Cleanup(func() { host.Close() })

	friend, err := Start(Config{
		PrivateKey: friendKey,
		ListenPort: friendPort,
		Addresses:  []net.IPNet{mustCIDR(t, "100.64.0.2/32")},
		Peers: []Peer{{
			PublicKey:  hostKey.PublicKey(),
			Endpoint:   loopback(hostPort),
			AllowedIPs: []net.IPNet{mustCIDR(t, "100.64.0.1/32")},
		}},
	})
	if err != nil {
		t.Fatalf("start friend: %v", err)
	}
	t.Cleanup(func() { friend.Close() })

	return &pair{host: host, friend: friend}
}

func TestDialThroughTunnel(t *testing.T) {
	p := startPair(t)

	ln, err := p.host.Listen("100.64.0.1:9000")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := p.friend.DialContext(ctx, "tcp", "100.64.0.1:9000")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello from the other side")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("got %q, want %q", got, msg)
	}
}

// A friend must not reach an address that is not in their allowed set. This is
// the property that makes the box a box.
func TestPeerCannotReachOtherAddresses(t *testing.T) {
	p := startPair(t)

	ln, err := p.host.Listen("100.64.0.9:9000")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := p.friend.DialContext(ctx, "tcp", "100.64.0.9:9000")
	if err == nil {
		conn.Close()
		t.Fatal("friend reached an address outside its allowed set")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Logf("dial failed with %v", err)
	}
}

func TestAddAndRemovePeer(t *testing.T) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	d, err := Start(Config{
		PrivateKey: key,
		ListenPort: freeUDPPort(t),
		Addresses:  []net.IPNet{mustCIDR(t, "100.64.0.1/32")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	peerKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	if err := d.AddPeer(Peer{
		PublicKey:  peerKey.PublicKey(),
		AllowedIPs: []net.IPNet{mustCIDR(t, "100.64.0.2/32")},
	}); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	if err := d.RemovePeer(peerKey.PublicKey()); err != nil {
		t.Fatalf("remove peer: %v", err)
	}
}

func TestStartRejectsBadConfig(t *testing.T) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Start(Config{Addresses: []net.IPNet{mustCIDR(t, "100.64.0.1/32")}}); err == nil {
		t.Fatal("started without a private key")
	}
	if _, err := Start(Config{PrivateKey: key}); err == nil {
		t.Fatal("started without an address")
	}
	if _, err := Start(Config{
		PrivateKey: key,
		Addresses:  []net.IPNet{mustCIDR(t, "100.64.0.1/32")},
		Peers:      []Peer{{PublicKey: mustKey(t)}},
	}); err == nil {
		t.Fatal("started with a peer that has no allowed addresses")
	}
}

func freeUDPPort(t *testing.T) uint16 {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return uint16(conn.LocalAddr().(*net.UDPAddr).Port)
}

func mustCIDR(t *testing.T, s string) net.IPNet {
	t.Helper()
	ip, network, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return net.IPNet{IP: ip, Mask: network.Mask}
}

func mustKey(t *testing.T) wgtypes.Key {
	t.Helper()
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key.PublicKey()
}

func loopback(port uint16) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}
