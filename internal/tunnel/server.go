package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/desynkro/lan-party/internal/secure"
	"github.com/quic-go/quic-go"
)

// ServerConfig is everything the tunnel needs from the host.
type ServerConfig struct {
	Identity        secure.Identity
	ListenAddr      string
	Authenticate    Authenticator
	MaxStreams      int
	ApprovalTimeout time.Duration
}

// Server accepts friends and delivers authenticated connections.
type Server struct {
	listener        *quic.Listener
	auth            Authenticator
	approvalTimeout time.Duration
	conns           chan *Conn
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.Mutex
	active          map[*Conn]struct{}
	closeOnce       sync.Once
}

// Listen opens the host's QUIC socket.
func Listen(cfg ServerConfig) (*Server, error) {
	if cfg.Authenticate == nil {
		return nil, errors.New("an authenticator is required")
	}
	tlsConf, err := serverTLS(cfg.Identity)
	if err != nil {
		return nil, fmt.Errorf("host certificate: %w", err)
	}

	addr := cfg.ListenAddr
	if addr == "" {
		addr = "0.0.0.0:0"
	}
	listener, err := quic.ListenAddr(addr, tlsConf, serverQUICConfig(cfg.MaxStreams))
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	timeout := cfg.ApprovalTimeout
	if timeout <= 0 {
		timeout = defaultApprovalTimeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		listener:        listener,
		auth:            cfg.Authenticate,
		approvalTimeout: timeout,
		conns:           make(chan *Conn, 16),
		ctx:             ctx,
		cancel:          cancel,
		active:          map[*Conn]struct{}{},
	}
	go s.acceptLoop()
	return s, nil
}

// Accept returns the next approved connection.
func (s *Server) Accept(ctx context.Context) (*Conn, error) {
	select {
	case c := <-s.conns:
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, quic.ErrServerClosed
	}
}

// Addr is the address the listener actually bound to.
func (s *Server) Addr() net.Addr { return s.listener.Addr() }

// Close stops the listener and every connection it accepted.
func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		_ = s.listener.Close()

		s.mu.Lock()
		conns := make([]*Conn, 0, len(s.active))
		for c := range s.active {
			conns = append(conns, c)
		}
		s.mu.Unlock()

		for _, c := range conns {
			_ = c.Close()
		}
		s.wg.Wait()
	})
	return nil
}

func (s *Server) acceptLoop() {
	for {
		qc, err := s.listener.Accept(s.ctx)
		if err != nil {
			return
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			c, err := s.handshake(qc)
			if err != nil {
				_ = qc.CloseWithError(1, "handshake failed")
				return
			}

			s.mu.Lock()
			s.active[c] = struct{}{}
			s.mu.Unlock()

			select {
			case s.conns <- c:
			case <-s.ctx.Done():
				_ = c.Close()
			}
		}()
	}
}

func (s *Server) handshake(qc *quic.Conn) (*Conn, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.approvalTimeout)
	defer cancel()

	stream, err := qc.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("control stream: %w", err)
	}
	dec := json.NewDecoder(stream)
	enc := json.NewEncoder(stream)

	var hello ControlMessage
	if err := dec.Decode(&hello); err != nil {
		return nil, fmt.Errorf("read hello: %w", err)
	}
	if hello.Type != MsgHello || hello.Version != ProtocolVersion {
		return nil, errors.New("the peer speaks a different protocol")
	}

	state := qc.ConnectionState()
	if len(state.TLS.PeerCertificates) == 0 {
		return nil, errors.New("the peer sent no client certificate")
	}
	pub, err := secure.PublicKeyFromCertificate(state.TLS.PeerCertificates[0])
	if err != nil {
		return nil, err
	}

	request := AuthRequest{
		GroupID:     hello.GroupID,
		MemberID:    hello.MemberID,
		PeerName:    hello.PeerName,
		DeviceName:  hello.DeviceName,
		Token:       hello.Token,
		RemoteAddr:  qc.RemoteAddr().String(),
		PublicKey:   pub,
		Fingerprint: secure.Fingerprint(pub),
	}

	decision, authErr := s.auth.Authenticate(ctx, request)
	if authErr != nil || !decision.Allow {
		reason := decision.Reason
		switch {
		case authErr != nil && errors.Is(authErr, context.DeadlineExceeded):
			reason = "the host did not answer in time"
		case authErr != nil:
			reason = "the host could not approve this device"
		case reason == "":
			reason = "the host denied the join"
		}
		_ = enc.Encode(ControlMessage{Type: MsgDenied, Reason: reason})
		time.Sleep(denialGrace)
		return nil, fmt.Errorf("%w: %s", ErrDenied, reason)
	}

	memberID := decision.MemberID
	if memberID == "" {
		memberID = hello.MemberID
	}
	if memberID == "" {
		return nil, errors.New("the authenticator did not assign a member id")
	}

	if err := enc.Encode(ControlMessage{
		Type:     MsgWelcome,
		MemberID: memberID,
		Services: decision.Services,
	}); err != nil {
		return nil, fmt.Errorf("send welcome: %w", err)
	}

	var c *Conn
	c = newConn(qc, stream, dec, enc, func() { s.removeConn(c) })
	c.MemberID = memberID
	c.Services = decision.Services
	return c, nil
}

func (s *Server) removeConn(c *Conn) {
	s.mu.Lock()
	delete(s.active, c)
	s.mu.Unlock()
}
