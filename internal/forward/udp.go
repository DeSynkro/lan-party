package forward

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// MaxSessions caps how many UDP conversations one service will carry.
const MaxSessions = 64

// udpIdle is how long a conversation with no packets in either direction is
// kept before its socket is closed.
var udpIdle = 2 * time.Minute

// ServeUDP forwards datagrams between a listener and a target, giving each
// client its own outbound socket so replies go back to the right place.
func ServeUDP(ctx context.Context, pc net.PacketConn, target string, dial DialFunc) error {
	f := &udpForwarder{
		listen:   pc,
		target:   target,
		dial:     dial,
		sessions: map[string]*udpSession{},
	}

	sweepDone := make(chan struct{})
	go func() {
		defer close(sweepDone)
		f.sweep(ctx)
	}()
	defer func() {
		f.closeAll()
		<-sweepDone
	}()

	buf := make([]byte, 65535)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if n == 0 {
			continue
		}
		if err := f.deliver(ctx, buf[:n], from); err != nil {
			// A broken conversation should not take the listener with it.
			continue
		}
	}
}

type udpForwarder struct {
	listen net.PacketConn
	target string
	dial   DialFunc

	mu       sync.Mutex
	sessions map[string]*udpSession
}

type udpSession struct {
	conn   net.Conn
	client net.Addr
	last   time.Time
	once   sync.Once
}

func (f *udpForwarder) deliver(ctx context.Context, data []byte, from net.Addr) error {
	s, err := f.session(ctx, from)
	if err != nil {
		return err
	}

	f.mu.Lock()
	s.last = time.Now()
	f.mu.Unlock()

	_, err = s.conn.Write(data)
	return err
}

func (f *udpForwarder) session(ctx context.Context, from net.Addr) (*udpSession, error) {
	key := from.String()

	f.mu.Lock()
	s, ok := f.sessions[key]
	if ok {
		f.mu.Unlock()
		return s, nil
	}
	if len(f.sessions) >= MaxSessions {
		f.mu.Unlock()
		return nil, errTooManySessions
	}
	f.mu.Unlock()

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := f.dial(dialCtx, "udp", f.target)
	if err != nil {
		return nil, err
	}

	s = &udpSession{conn: conn, client: from, last: time.Now()}

	f.mu.Lock()
	if existing, ok := f.sessions[key]; ok {
		// Another goroutine got there first.
		f.mu.Unlock()
		conn.Close()
		return existing, nil
	}
	f.sessions[key] = s
	f.mu.Unlock()

	go f.reply(s)
	return s, nil
}

func (f *udpForwarder) reply(s *udpSession) {
	buf := make([]byte, 65535)

	for {
		n, err := s.conn.Read(buf)
		if err != nil {
			f.forget(s)
			return
		}

		f.mu.Lock()
		s.last = time.Now()
		f.mu.Unlock()

		if _, err := f.listen.WriteTo(buf[:n], s.client); err != nil {
			f.forget(s)
			return
		}
	}
}

func (f *udpForwarder) forget(s *udpSession) {
	f.mu.Lock()
	defer f.mu.Unlock()

	s.once.Do(func() { s.conn.Close() })
	for key, existing := range f.sessions {
		if existing == s {
			delete(f.sessions, key)
			return
		}
	}
}

func (f *udpForwarder) sweep(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-udpIdle)
			f.mu.Lock()
			for key, s := range f.sessions {
				if s.last.Before(cutoff) {
					s.once.Do(func() { s.conn.Close() })
					delete(f.sessions, key)
				}
			}
			f.mu.Unlock()
		}
	}
}

func (f *udpForwarder) closeAll() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for key, s := range f.sessions {
		s.once.Do(func() { s.conn.Close() })
		delete(f.sessions, key)
	}
}

var errTooManySessions = errors.New("too many udp conversations for this service")
