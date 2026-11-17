package forward

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func echoServer(t *testing.T) string {
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

func TestHandleTCPEcho(t *testing.T) {
	target := echoServer(t)

	gameLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer gameLn.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := gameLn.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	game, err := net.Dial("tcp", gameLn.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer game.Close()
	app := <-accepted
	defer app.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		_ = HandleTCP(app, func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", target)
		}, ctx)
	}()

	if _, err := game.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(game, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("echo = %q", got)
	}

	if tcp, ok := game.(*net.TCPConn); ok {
		if err := tcp.CloseWrite(); err != nil {
			t.Fatal(err)
		}
	}
	_ = game.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := game.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("read after half-close = %v", err)
	}
}

func TestServeTCPEcho(t *testing.T) {
	target := echoServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ServeTCP(ctx, ln, func(ctx context.Context) (io.ReadWriteCloser, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", target)
	})

	game, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer game.Close()

	if _, err := game.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 2)
	if _, err := io.ReadFull(game, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Fatalf("echo = %q", got)
	}
}
