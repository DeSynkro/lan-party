// Package forward copies bytes between a listener and a target. The same code
// runs on both sides: the host forwards a published box port to its LAN, and
// the friend forwards a loopback port through the tunnel.
package forward

import (
	"context"
	"io"
	"net"
	"sync"
	"time"
)

// MaxConns caps how many connections one service will carry at once, so a
// single peer cannot exhaust the host.
const MaxConns = 64

// DialFunc opens the far side. On the host it is a normal net.Dialer, on the
// friend it is the tunnel.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ServeTCP accepts connections until the listener closes or the context ends.
func ServeTCP(ctx context.Context, ln net.Listener, target string, dial DialFunc) error {
	limit := make(chan struct{}, MaxConns)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		select {
		case limit <- struct{}{}:
		default:
			// Too many open connections for this service.
			conn.Close()
			continue
		}

		go func() {
			defer func() { <-limit }()
			handle(ctx, conn, target, dial)
		}()
	}
}

func handle(ctx context.Context, client net.Conn, target string, dial DialFunc) {
	defer client.Close()

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	upstream, err := dial(dialCtx, "tcp", target)
	if err != nil {
		return
	}
	defer upstream.Close()

	pipe(client, upstream)
}

// pipe copies both ways and closes each half as it finishes, so a game that
// halts one direction still sees the other close cleanly.
func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		closeWrite(a)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		closeWrite(b)
	}()

	wg.Wait()
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
