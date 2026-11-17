package forward

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestFlowTable(t *testing.T) {
	table := NewFlowTable(2, time.Minute)
	first := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1000}
	second := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1001}
	third := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1002}

	id, ok := table.New(first)
	if !ok {
		t.Fatal("first flow was refused")
	}
	again, ok := table.New(first)
	if !ok || again != id {
		t.Fatalf("same address got %d, want %d", again, id)
	}
	secondID, ok := table.New(second)
	if !ok || secondID == id {
		t.Fatalf("second flow = %d", secondID)
	}
	if _, ok := table.New(third); ok {
		t.Fatal("the flow limit was not enforced")
	}

	addr, ok := table.Lookup(id)
	if !ok || addr.String() != first.String() {
		t.Fatalf("lookup = %v, %v", addr, ok)
	}

	expired := table.Expire(time.Now().Add(2 * time.Minute))
	if len(expired) != 2 || table.Len() != 0 {
		t.Fatalf("expired = %v, len = %d", expired, table.Len())
	}
}

func TestDatagramFraming(t *testing.T) {
	packet, err := EncodeDatagram(25565, 7, []byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	port, id, payload, err := DecodeDatagram(packet)
	if err != nil {
		t.Fatal(err)
	}
	if port != 25565 || id != 7 || string(payload) != "ping" {
		t.Fatalf("decoded = %d %d %q", port, id, payload)
	}

	if _, _, _, err := DecodeDatagram([]byte("short")); err == nil {
		t.Fatal("short datagram was accepted")
	}
	if _, err := EncodeDatagram(0, 1, nil); err == nil {
		t.Fatal("bad port was accepted")
	}
	if _, err := EncodeDatagram(25565, 1, make([]byte, MaxUDPPayload+1)); err == nil {
		t.Fatal("oversized payload was accepted")
	}
}

func TestServeHostUDP(t *testing.T) {
	echo, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := echo.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = echo.WriteTo(buf[:n], addr)
		}
	}()

	recv := make(chan []byte, 4)
	send := make(chan []byte, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = ServeHostUDP(ctx, 25565, func(b []byte) error {
			send <- b
			return nil
		}, recv, echo.LocalAddr().String())
	}()

	packet, err := EncodeDatagram(25565, 7, []byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	recv <- packet

	select {
	case out := <-send:
		port, id, payload, err := DecodeDatagram(out)
		if err != nil {
			t.Fatal(err)
		}
		if port != 25565 || id != 7 || string(payload) != "ping" {
			t.Fatalf("reply = %d %d %q", port, id, payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the host did not echo the reply")
	}
}

func TestServeFriendUDP(t *testing.T) {
	local, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()

	send := make(chan []byte, 4)
	recv := make(chan []byte, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = ServeFriendUDP(ctx, local, 25565, func(b []byte) error {
			send <- b
			return nil
		}, recv)
	}()

	game, err := net.DialUDP("udp", nil, local.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer game.Close()
	if _, err := game.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}

	var id FlowID
	select {
	case out := <-send:
		port, gotID, payload, err := DecodeDatagram(out)
		if err != nil {
			t.Fatal(err)
		}
		if port != 25565 || string(payload) != "ping" {
			t.Fatalf("outgoing = %d %d %q", port, gotID, payload)
		}
		id = gotID
	case <-time.After(2 * time.Second):
		t.Fatal("the friend did not send the game packet")
	}

	reply, err := EncodeDatagram(25565, id, []byte("pong"))
	if err != nil {
		t.Fatal(err)
	}
	recv <- reply

	_ = game.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 32)
	n, err := game.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "pong" {
		t.Fatalf("game got %q", buf[:n])
	}
}
