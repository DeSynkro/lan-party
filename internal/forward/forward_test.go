package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/tunnel"
)

// The whole path in one test: a game connects to loopback on the friend's
// machine, crosses the tunnel, and reaches a server on the host's LAN.
func TestForwardThroughTunnel(t *testing.T) {
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go echoOnce(echoLn)

	host, friend := startPair(t)

	hostLn, err := host.Listen("100.64.0.1:9000")
	if err != nil {
		t.Fatalf("host listen: %v", err)
	}
	defer hostLn.Close()

	friendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer friendLn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dialer net.Dialer
	go func() { _ = ServeTCP(ctx, hostLn, echoLn.Addr().String(), dialer.DialContext) }()
	go func() { _ = ServeTCP(ctx, friendLn, "100.64.0.1:9000", friend.DialContext) }()

	conn, err := net.DialTimeout("tcp", friendLn.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatalf("connect to friend's loopback: %v", err)
	}
	defer conn.Close()

	msg := []byte("minecraft handshake")
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

func echoOnce(ln net.Listener) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = io.Copy(conn, conn)
}

func startPair(t *testing.T) (*tunnel.Device, *tunnel.Device) {
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

	host, err := tunnel.Start(tunnel.Config{
		PrivateKey: hostKey,
		ListenPort: hostPort,
		Addresses:  []net.IPNet{mustCIDR(t, "100.64.0.1/32")},
		Peers: []tunnel.Peer{{
			PublicKey:  friendKey.PublicKey(),
			Endpoint:   endpoint(friendPort),
			AllowedIPs: []net.IPNet{mustCIDR(t, "100.64.0.2/32")},
		}},
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	t.Cleanup(func() { host.Close() })

	friend, err := tunnel.Start(tunnel.Config{
		PrivateKey: friendKey,
		ListenPort: friendPort,
		Addresses:  []net.IPNet{mustCIDR(t, "100.64.0.2/32")},
		Peers: []tunnel.Peer{{
			PublicKey:  hostKey.PublicKey(),
			Endpoint:   endpoint(hostPort),
			AllowedIPs: []net.IPNet{mustCIDR(t, "100.64.0.1/32")},
		}},
	})
	if err != nil {
		t.Fatalf("start friend: %v", err)
	}
	t.Cleanup(func() { friend.Close() })

	return host, friend
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

func endpoint(port uint16) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}
