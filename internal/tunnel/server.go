package tunnel

import (
	"bufio"
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
	Identity             secure.Identity
	ListenAddr           string
	PacketConn           net.PacketConn // optional; the server takes ownership
	Relay                bool
	Authenticate         Authenticator
	MaxStreams           int
	ApprovalTimeout      time.Duration
	MaxPendingHandshakes int
	HandshakeTimeout     time.Duration
}

// Server accepts friends and delivers authenticated connections.
type Server struct {
	listener         *quic.Listener
	transport        *quic.Transport
	packetConn       net.PacketConn
	relay            bool
	auth             Authenticator
	approvalTimeout  time.Duration
	handshakeTimeout time.Duration
	pendingSlots     chan struct{}
	conns            chan *Conn
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	mu               sync.Mutex
	active           map[*Conn]struct{}
	pending          map[*quic.Conn]struct{}
	closing          bool
	closeOnce        sync.Once
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

	var listener *quic.Listener
	var transport *quic.Transport
	if cfg.PacketConn != nil {
		transport = &quic.Transport{Conn: cfg.PacketConn}
		listener, err = transport.Listen(tlsConf, serverQUICConfig(cfg.MaxStreams))
		if err != nil {
			_ = transport.Close()
			_ = cfg.PacketConn.Close()
			return nil, fmt.Errorf("listen: %w", err)
		}
	} else {
		addr := cfg.ListenAddr
		if addr == "" {
			addr = "0.0.0.0:0"
		}
		listener, err = quic.ListenAddr(addr, tlsConf, serverQUICConfig(cfg.MaxStreams))
		if err != nil {
			return nil, fmt.Errorf("listen: %w", err)
		}
	}

	timeout := cfg.ApprovalTimeout
	if timeout <= 0 {
		timeout = defaultApprovalTimeout
	}
	handshakeTimeout := cfg.HandshakeTimeout
	if handshakeTimeout <= 0 {
		handshakeTimeout = defaultHandshakeTimeout
	}
	maxPending := cfg.MaxPendingHandshakes
	if maxPending <= 0 {
		maxPending = defaultMaxPendingHandshakes
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		listener:         listener,
		transport:        transport,
		packetConn:       cfg.PacketConn,
		relay:            cfg.Relay,
		auth:             cfg.Authenticate,
		approvalTimeout:  timeout,
		handshakeTimeout: handshakeTimeout,
		pendingSlots:     make(chan struct{}, maxPending),
		conns:            make(chan *Conn, 16),
		ctx:              ctx,
		cancel:           cancel,
		active:           map[*Conn]struct{}{},
		pending:          map[*quic.Conn]struct{}{},
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
		s.mu.Lock()
		s.closing = true
		s.mu.Unlock()

		s.cancel()
		_ = s.listener.Close()
		if s.transport != nil {
			_ = s.transport.Close()
		}
		if s.packetConn != nil {
			_ = s.packetConn.Close()
		}

		s.mu.Lock()
		conns := make([]*Conn, 0, len(s.active))
		for c := range s.active {
			conns = append(conns, c)
		}
		pending := make([]*quic.Conn, 0, len(s.pending))
		for qc := range s.pending {
			pending = append(pending, qc)
		}
		s.mu.Unlock()

		for _, qc := range pending {
			_ = qc.CloseWithError(0, "server closed")
		}
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

		select {
		case s.pendingSlots <- struct{}{}:
		default:
			_ = qc.CloseWithError(2, "server busy")
			continue
		}

		s.mu.Lock()
		if s.closing {
			s.mu.Unlock()
			<-s.pendingSlots
			_ = qc.CloseWithError(0, "server closed")
			continue
		}
		s.pending[qc] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()

		go func() {
			defer s.wg.Done()
			defer func() { <-s.pendingSlots }()
			defer func() {
				s.mu.Lock()
				delete(s.pending, qc)
				s.mu.Unlock()
			}()

			c, err := s.handshake(qc)
			if err != nil {
				_ = qc.CloseWithError(1, "handshake failed")
				return
			}

			s.mu.Lock()
			if s.closing {
				s.mu.Unlock()
				_ = c.Close()
				return
			}
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
	if err := stream.SetReadDeadline(time.Now().Add(s.handshakeTimeout)); err != nil {
		return nil, fmt.Errorf("set hello deadline: %w", err)
	}
	reader := bufio.NewReader(stream)
	enc := json.NewEncoder(stream)

	hello, err := readControl(reader)
	if err != nil {
		return nil, fmt.Errorf("read hello: %w", err)
	}
	if err := stream.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear hello deadline: %w", err)
	}
	if hello.Type != MsgHello || hello.Version != ProtocolVersion {
		return nil, errors.New("the peer speaks a different protocol")
	}
	if len(hello.GroupID) > 128 || len(hello.MemberID) > 128 || len(hello.DeviceName) > 512 || len(hello.PeerName) > 128 || len(hello.Token) > 512 {
		return nil, errors.New("the hello is too large")
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
		GroupID:       hello.GroupID,
		MemberID:      clipText(hello.MemberID, 128),
		PeerName:      clipText(hello.PeerName, 60),
		DeviceName:    clipText(hello.DeviceName, 120),
		Token:         clipText(hello.Token, 512),
		HasRelayToken: hello.HasRelayToken,
		RemoteAddr:    qc.RemoteAddr().String(),
		Relay:         s.relay,
		PublicKey:     pub,
		Fingerprint:   secure.Fingerprint(pub),
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
		_ = writeControl(enc, stream, ControlMessage{Type: MsgDenied, Reason: reason})
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

	if err := writeControl(enc, stream, ControlMessage{
		Type:       MsgWelcome,
		MemberID:   memberID,
		Services:   decision.Services,
		RelayToken: decision.RelayToken,
	}); err != nil {
		return nil, fmt.Errorf("send welcome: %w", err)
	}

	c := newConn(qc, stream, reader, enc, nil)
	c.onClose = func() { s.removeConn(c) }
	c.MemberID = memberID
	c.Services = decision.Services
	c.start()
	return c, nil
}

func (s *Server) removeConn(c *Conn) {
	s.mu.Lock()
	delete(s.active, c)
	s.mu.Unlock()
}
