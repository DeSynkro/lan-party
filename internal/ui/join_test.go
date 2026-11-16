package ui

import (
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/session"
	"github.com/desynkro/lan-party/internal/tunnel"
)

// Joining a group should connect, open the loopback port, and be forgotten
// cleanly when the friend leaves.
func TestJoinAndForget(t *testing.T) {
	inv := liveInvite(t)

	a := test.NewApp()
	a.Settings().SetTheme(NewTheme())
	app := &App{
		fyneApp:     a,
		win:         a.NewWindow("LAN Party"),
		inviteStore: invite.OpenStore(filepath.Join(t.TempDir(), "joined.json")),
	}
	app.win.Resize(fyne.NewSize(420, 620))

	app.openJoined(inv)
	if app.friend == nil {
		t.Fatal("join did not start a session")
	}
	if len(app.joined) != 1 {
		t.Fatalf("got %d remembered groups, want 1", len(app.joined))
	}
	if got := app.friend.Bound(); len(got) != 1 {
		t.Fatalf("got %d bound ports, want 1", len(got))
	}
	capture(t, app.win, "friend.png")

	app.forget(inv)
	if app.friend != nil {
		t.Fatal("forget left the session running")
	}
	if len(app.joined) != 0 {
		t.Fatalf("got %d remembered groups, want 0", len(app.joined))
	}
}

// liveInvite starts a host with one service and returns an invite for a friend.
func liveInvite(t *testing.T) invite.Invite {
	t.Helper()

	echo := tcpEcho(t)

	hostKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	peerKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	port := freeUDPPort(t)
	host, err := session.StartHost(session.HostConfig{
		PrivateKey: hostKey,
		ListenPort: port,
		Peers: []tunnel.Peer{{
			PublicKey:  peerKey.PublicKey(),
			AllowedIPs: []net.IPNet{mustNet(t, "100.64.0.2/32")},
		}},
		Services: []group.Service{{
			ID:     "svc",
			Name:   "Minecraft",
			Proto:  "tcp",
			Ports:  []int{25565},
			Target: echo,
		}},
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	t.Cleanup(func() { host.Close() })

	return invite.Invite{
		Version:   1,
		GroupID:   "grp",
		GroupName: "Friday Night",
		HostName:  "Desynkro",
		Endpoint:  fmt.Sprintf("127.0.0.1:%d", port),
		HostKey:   hostKey.PublicKey().String(),
		PeerName:  "Ash",
		PeerKey:   peerKey.String(),
		PeerBox:   "100.64.0.2",
		Expires:   time.Now().Add(time.Hour),
		Services: []invite.Service{{
			Name:  "Minecraft",
			Proto: "tcp",
			Ports: []int{25565},
		}},
	}
}

func tcpEcho(t *testing.T) string {
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

func mustNet(t *testing.T, s string) net.IPNet {
	t.Helper()
	ip, network, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatal(err)
	}
	return net.IPNet{IP: ip, Mask: network.Mask}
}
