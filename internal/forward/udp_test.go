package forward

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestUDPForwardThroughTunnel(t *testing.T) {
	echoAddr := startUDPEcho(t)

	host, friend := startPair(t)

	hostPC, err := host.ListenPacket("100.64.0.1:9001")
	if err != nil {
		t.Fatalf("host listen: %v", err)
	}
	defer hostPC.Close()

	friendPC, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer friendPC.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dialer net.Dialer
	go func() { _ = ServeUDP(ctx, hostPC, echoAddr, dialer.DialContext) }()
	go func() { _ = ServeUDP(ctx, friendPC, "100.64.0.1:9001", friend.DialContext) }()

	client, err := net.Dial("udp", friendPC.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// The first datagram can be lost while the handshake completes, which is
	// also true in a real session, so send until a reply comes back.
	deadline := time.Now().Add(10 * time.Second)
	msg := []byte("valheim")
	buf := make([]byte, 64)

	for time.Now().Before(deadline) {
		if _, err := client.Write(msg); err != nil {
			t.Fatalf("write: %v", err)
		}
		_ = client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := client.Read(buf)
		if err != nil {
			continue
		}
		if string(buf[:n]) != string(msg) {
			t.Fatalf("got %q, want %q", buf[:n], msg)
		}
		return
	}

	t.Fatal("no reply through the tunnel")
}

func startUDPEcho(t *testing.T) string {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
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
