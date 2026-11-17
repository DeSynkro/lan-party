// Package forward moves bytes between a game, the tunnel and a LAN target.
package forward

import (
	"context"
	"io"
	"net"
	"sync"
)

// ServeTCP accepts game connections until ctx ends. open supplies the other
// half, usually a tunnel data stream.
func ServeTCP(ctx context.Context, ln net.Listener, open func(context.Context) (io.ReadWriteCloser, error)) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-done:
		}
	}()
	defer close(done)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			stream, err := open(ctx)
			if err != nil {
				return
			}
			defer stream.Close()
			_ = Pipe(conn, stream)
		}(conn)
	}
}

// HandleTCP connects a tunnel stream to the LAN target.
func HandleTCP(stream io.ReadWriteCloser, dial func(context.Context) (net.Conn, error), ctx context.Context) error {
	conn, err := dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer stream.Close()
	return Pipe(conn, stream)
}

// Pipe copies both directions and half-closes each side when its copy ends.
func Pipe(a, b io.ReadWriteCloser) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(a, b)
		closeWrite(a)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(b, a)
		closeWrite(b)
		errs <- err
	}()
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func closeWrite(v io.ReadWriteCloser) {
	if cw, ok := v.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = v.Close()
}
