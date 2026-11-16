package session

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/tunnel"
)

// The whole product in one test: a service on the host's LAN, published on the
// box address, reached from a loopback port on the friend's machine.
func TestFriendReachesPublishedService(t *testing.T) {
	echoAddr := startTCPEcho(t)

	_, inv := startSessionPair(t, group.Service{
		ID:     "svc-tcp",
		Name:   "Echo",
		Proto:  "tcp",
		Ports:  []int{25565},
		Target: echoAddr,
	})

	friend, err := StartFriend(FriendConfig{Invite: inv})
	if err != nil {
		t.Fatalf("start friend: %v", err)
	}
	defer friend.Close()

	bounds := friend.Bound()
	if len(bounds) != 1 {
		t.Fatalf("got %d bound ports, want 1", len(bounds))
	}

	conn, err := net.DialTimeout("tcp", bounds[0].Address, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", bounds[0].Address, err)
	}
	defer conn.Close()

	msg := []byte("hello minecraft")
	if _, err := conn.Write(msg); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("got %q, want %q", got, msg)
	}
}

func TestFriendReachesPublishedUDP(t *testing.T) {
	echoAddr := startUDPEcho(t)

	_, inv := startSessionPair(t, group.Service{
		ID:     "svc-udp",
		Name:   "Echo",
		Proto:  "udp",
		Ports:  []int{2456},
		Target: echoAddr,
	})

	friend, err := StartFriend(FriendConfig{Invite: inv})
	if err != nil {
		t.Fatalf("start friend: %v", err)
	}
	defer friend.Close()

	bounds := friend.Bound()
	if len(bounds) != 1 {
		t.Fatalf("got %d bound ports, want 1", len(bounds))
	}

	conn, err := net.Dial("udp", bounds[0].Address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	msg := []byte("valheim")
	buf := make([]byte, 64)
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		if _, err := conn.Write(msg); err != nil {
			t.Fatal(err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}
		if string(buf[:n]) != string(msg) {
			t.Fatalf("got %q, want %q", buf[:n], msg)
		}
		return
	}
	t.Fatal("no reply through the session")
}

// startSessionPair publishes one service and returns the host and an invite.
func startSessionPair(t *testing.T, svc group.Service) (*Host, invite.Invite) {
	t.Helper()

	hostKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	peerKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	listenPort := freeUDPPort(t)

	host, err := StartHost(HostConfig{
		PrivateKey: hostKey,
		ListenPort: listenPort,
		Peers: []tunnel.Peer{{
			PublicKey:  peerKey.PublicKey(),
			AllowedIPs: []net.IPNet{testCIDR(t, "100.64.0.2/32")},
		}},
		Services: []group.Service{svc},
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	t.Cleanup(func() { host.Close() })

	return host, invite.Invite{
		Version:   1,
		GroupID:   "grp",
		GroupName: "Friday Night",
		HostName:  "Desynkro",
		Endpoint:  fmt.Sprintf("127.0.0.1:%d", listenPort),
		HostKey:   hostKey.PublicKey().String(),
		PeerName:  "Ash",
		PeerKey:   peerKey.String(),
		PeerBox:   "100.64.0.2",
		Expires:   time.Now().Add(time.Hour),
		Services: []invite.Service{{
			Name:  svc.Name,
			Proto: svc.Proto,
			Ports: svc.Ports,
		}},
	}
}

func startTCPEcho(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	return ln.Addr().String()
}

func startUDPEcho(t *testing.T) string {
	t.Helper()

	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })

	go func() {
		buf := make([]byte, 65535)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(buf[:n], from)
		}
	}()

	return pc.LocalAddr().String()
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

func testCIDR(t *testing.T, s string) net.IPNet {
	t.Helper()
	ip, network, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return net.IPNet{IP: ip, Mask: network.Mask}
}

// The host has to be able to tell who is actually in the session, otherwise the
// roster is decoration.
func TestHostSeesFriendConnect(t *testing.T) {
	echoAddr := startTCPEcho(t)

	host, inv := startSessionPair(t, group.Service{
		ID:     "svc-tcp",
		Name:   "Echo",
		Proto:  "tcp",
		Ports:  []int{25565},
		Target: echoAddr,
	})

	friend, err := StartFriend(FriendConfig{Invite: inv})
	if err != nil {
		t.Fatalf("start friend: %v", err)
	}
	defer friend.Close()

	conn, err := net.DialTimeout("tcp", friend.Bound()[0].Address, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	peerKey, err := wgtypes.ParseKey(inv.PeerKey)
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if last := host.Presence()[peerKey.PublicKey().String()]; !last.IsZero() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("the host never saw the friend connect")
}
