package session

import (
	"net"
	"testing"
	"time"
)

func TestBindTCPFallback(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	ln, got, err := BindTCP(port)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got == port {
		t.Fatalf("BindTCP returned the busy port %d", port)
	}

	accepted := make(chan bool, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
			accepted <- true
		}
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("the listener did not accept a connection")
	}
}

func TestBindUDPFallback(t *testing.T) {
	busy, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.LocalAddr().(*net.UDPAddr).Port

	conn, got, err := BindUDP(port)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if got == port {
		t.Fatalf("BindUDP returned the busy port %d", port)
	}

	client, err := net.DialUDP("udp", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		t.Fatal(err)
	}
}
