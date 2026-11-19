package app

import (
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func tcpEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestHostAndFriendEndToEnd(t *testing.T) {
	host := newTestApp(t)
	friend := newTestApp(t)

	echoAddr := tcpEchoServer(t)
	published := freeTCPPort(t)
	appPort := freeUDPPort(t)

	g, err := host.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	settings := host.Settings()
	settings.PublicAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(appPort))
	settings.AppPort = appPort
	if err := host.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := host.AddService(g.ID, group.Service{
		Name:   "Echo",
		Proto:  "tcp",
		Target: echoAddr,
		Ports:  []int{published},
		Hint:   "Connect to the address shown",
	}); err != nil {
		t.Fatal(err)
	}
	if err := host.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.StopSession() })

	code, err := host.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	joined := make(chan error, 1)
	go func() {
		_, err := friend.JoinInvite(ctx, code, "ash-pc")
		joined <- err
	}()

	var approval *Approval
	waitFor(t, 3*time.Second, func() bool {
		approval = host.Approval()
		return approval != nil
	})
	if approval.Kind != "join" || approval.PeerName != "Ash" {
		t.Fatalf("approval = %+v", approval)
	}
	if !host.ResolveApproval(approval.ID, true) {
		t.Fatal("could not approve the device")
	}
	if err := <-joined; err != nil {
		t.Fatal(err)
	}

	friendGroups := friend.Groups()
	if len(friendGroups) != 1 {
		t.Fatalf("friend groups = %d", len(friendGroups))
	}
	fg := friendGroups[0]
	if !friend.FriendPresence().Live {
		t.Fatal("the friend does not see the host as live")
	}
	waitFor(t, 3*time.Second, func() bool {
		p, ok := host.HostPresence()[fg.SelfID]
		return ok && p.Live
	})

	var boundAddress string
	for _, bound := range friend.FriendBounds() {
		if bound.Proto == "tcp" && bound.Error == "" {
			boundAddress = bound.Address
			break
		}
	}
	if boundAddress == "" {
		t.Fatalf("bounds = %+v", friend.FriendBounds())
	}

	conn, err := net.Dial("tcp", boundAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("echo = %q", got)
	}

	if err := host.RemoveMember(g.ID, fg.SelfID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, func() bool { return !friend.FriendPresence().Live })
}

func TestFriendReconnectAfterDisconnectAndHostRestart(t *testing.T) {
	host := newTestApp(t)
	friend := newTestApp(t)

	echoAddr := tcpEchoServer(t)
	published := freeTCPPort(t)
	appPort := freeUDPPort(t)

	g, err := host.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	settings := host.Settings()
	settings.PublicAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(appPort))
	settings.AppPort = appPort
	if err := host.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := host.AddService(g.ID, group.Service{
		Name:   "Echo",
		Proto:  "tcp",
		Target: echoAddr,
		Ports:  []int{published},
	}); err != nil {
		t.Fatal(err)
	}
	if err := host.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.StopSession() })

	code, err := host.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	joined := make(chan error, 1)
	go func() {
		_, err := friend.JoinInvite(ctx, code, "ash-pc")
		joined <- err
	}()

	var approval *Approval
	waitFor(t, 3*time.Second, func() bool {
		approval = host.Approval()
		return approval != nil
	})
	if !host.ResolveApproval(approval.ID, true) {
		t.Fatal("could not approve the device")
	}
	if err := <-joined; err != nil {
		t.Fatal(err)
	}

	fg := friend.Groups()[0]
	if fg.HostGroupID != g.ID {
		t.Fatalf("host group id = %q, want %q", fg.HostGroupID, g.ID)
	}

	if err := friend.StopSession(); err != nil {
		t.Fatal(err)
	}
	reconnectCtx, reconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := friend.Reconnect(reconnectCtx, fg.ID); err != nil {
		t.Fatal(err)
	}
	reconnectCancel()
	waitFor(t, 3*time.Second, func() bool { return friend.FriendPresence().Live })

	if err := host.StopSession(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, func() bool { return !friend.FriendPresence().Live })

	if err := host.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	reconnectCtx, reconnectCancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := friend.Reconnect(reconnectCtx, fg.ID); err != nil {
		t.Fatal(err)
	}
	reconnectCancel()
	waitFor(t, 3*time.Second, func() bool { return friend.FriendPresence().Live })
}

func TestFriendAutoReconnectsAfterHostRestart(t *testing.T) {
	host := newTestApp(t)
	friend := newTestApp(t)

	appPort := freeUDPPort(t)
	g, err := host.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	settings := host.Settings()
	settings.PublicAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(appPort))
	settings.AppPort = appPort
	if err := host.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := host.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.StopSession() })

	code, err := host.CreateInvite(g.ID, "Ash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	joined := make(chan error, 1)
	go func() {
		_, err := friend.JoinInvite(ctx, code, "ash-pc")
		joined <- err
	}()

	var approval *Approval
	waitFor(t, 3*time.Second, func() bool {
		approval = host.Approval()
		return approval != nil
	})
	if !host.ResolveApproval(approval.ID, true) {
		t.Fatal("could not approve the device")
	}
	if err := <-joined; err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, func() bool { return friend.FriendPresence().Live })
	fg := friend.Groups()[0]

	if err := host.StopSession(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, func() bool { return !friend.FriendPresence().Live })

	if err := host.StartSession(g.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 8*time.Second, func() bool { return friend.FriendPresence().Live })

	if status := friend.ReconnectStatus(fg.ID); status.Active || status.Stopped {
		t.Fatalf("reconnect status after success = %+v", status)
	}
}
