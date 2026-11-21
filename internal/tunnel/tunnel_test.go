package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"io/fs"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/secure"
	"github.com/quic-go/quic-go"
)

type memStore map[string]string

func (m memStore) slot(service, user string) string { return service + "\x00" + user }

func (m memStore) Get(service, user string) (string, error) {
	value, ok := m[m.slot(service, user)]
	if !ok {
		return "", fs.ErrNotExist
	}
	return value, nil
}

func (m memStore) Set(service, user, secret string) error {
	m[m.slot(service, user)] = secret
	return nil
}

func (m memStore) Delete(service, user string) error {
	delete(m, m.slot(service, user))
	return nil
}

func testIdentity(t *testing.T, name string) secure.Identity {
	t.Helper()
	store := secure.NewStore(memStore{})
	identity, err := store.LoadOrCreate(name)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

type authFunc func(context.Context, AuthRequest) (AuthDecision, error)

func (f authFunc) Authenticate(ctx context.Context, req AuthRequest) (AuthDecision, error) {
	return f(ctx, req)
}

func startServer(t *testing.T, auth Authenticator, maxStreams int) (*Server, string) {
	t.Helper()
	return startServerConfig(t, auth, ServerConfig{MaxStreams: maxStreams})
}

func startServerConfig(t *testing.T, auth Authenticator, cfg ServerConfig) (*Server, string) {
	t.Helper()
	host := testIdentity(t, "host")
	cfg.Identity = host
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:0"
	}
	cfg.Authenticate = auth
	if cfg.ApprovalTimeout <= 0 {
		cfg.ApprovalTimeout = 3 * time.Second
	}
	srv, err := Listen(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, secure.Pin(host.Public)
}

func rawDial(t *testing.T, srv *Server, pin string) (*quic.Conn, *quic.Stream, error) {
	t.Helper()
	tlsConf, err := clientTLS(testIdentity(t, "raw-client"), pin)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	qc, err := quic.DialAddr(ctx, srv.Addr().String(), tlsConf, clientQUICConfig())
	if err != nil {
		return nil, nil, err
	}
	stream, err := qc.OpenStreamSync(ctx)
	if err != nil {
		_ = qc.CloseWithError(1, "stream failed")
		return nil, nil, err
	}
	return qc, stream, nil
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true in time")
}

func dial(t *testing.T, srv *Server, pin, token string) *Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, ClientConfig{
		Identity:   testIdentity(t, "client"),
		Endpoint:   srv.Addr().String(),
		HostPin:    pin,
		GroupID:    "group-1",
		PeerName:   "Ash",
		DeviceName: "ash-desktop",
		Token:      token,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func accept(t *testing.T, srv *Server) *Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, err := srv.Accept(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func allowAuth(services ...ServiceInfo) Authenticator {
	return authFunc(func(_ context.Context, req AuthRequest) (AuthDecision, error) {
		if req.Token != "good" {
			return AuthDecision{Allow: false, Reason: "that invite is not valid"}, nil
		}
		if len(req.PublicKey) != ed25519.PublicKeySize {
			return AuthDecision{Allow: false, Reason: "no device key"}, nil
		}
		return AuthDecision{Allow: true, MemberID: "member-1", Services: services}, nil
	})
}

func TestDialAndWelcome(t *testing.T) {
	services := []ServiceInfo{{ID: "svc-1", Name: "Minecraft", Proto: "tcp", Ports: []int{25565}}}
	srv, pin := startServer(t, allowAuth(services...), 8)

	client := dial(t, srv, pin, "good")
	if client.MemberID != "member-1" {
		t.Fatalf("member id = %q", client.MemberID)
	}
	if len(client.Services) != 1 || client.Services[0].Name != "Minecraft" {
		t.Fatalf("services = %+v", client.Services)
	}

	server := accept(t, srv)
	if server.MemberID != "member-1" {
		t.Fatalf("server member id = %q", server.MemberID)
	}
	if !strings.HasPrefix(server.RemoteAddr().String(), "127.0.0.1:") {
		t.Fatalf("remote address = %q", server.RemoteAddr())
	}
}

func TestWrongPinRejected(t *testing.T) {
	srv, _ := startServer(t, allowAuth(), 8)
	other := testIdentity(t, "other")
	wrongPin := secure.Pin(other.Public)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Dial(ctx, ClientConfig{
		Identity: testIdentity(t, "client"),
		Endpoint: srv.Addr().String(),
		HostPin:  wrongPin,
		GroupID:  "group-1",
		Token:    "good",
	})
	if err == nil {
		t.Fatal("a connection with the wrong pin succeeded")
	}
}

func TestDeniedEnrollment(t *testing.T) {
	srv, pin := startServer(t, authFunc(func(context.Context, AuthRequest) (AuthDecision, error) {
		return AuthDecision{Allow: false, Reason: "that invite is not valid"}, nil
	}), 8)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := Dial(ctx, ClientConfig{
		Identity: testIdentity(t, "client"),
		Endpoint: srv.Addr().String(),
		HostPin:  pin,
		GroupID:  "group-1",
		Token:    "bad",
	})
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("error = %v", err)
	}
}

func TestControlRoundTrip(t *testing.T) {
	srv, pin := startServer(t, allowAuth(), 8)
	client := dial(t, srv, pin, "good")
	server := accept(t, srv)

	if err := client.Send(ControlMessage{Type: MsgBye}); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-server.Messages():
		if msg.Type != MsgBye {
			t.Fatalf("server got %q", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive the control message")
	}

	if err := server.Send(ControlMessage{Type: MsgServices}); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-client.Messages():
		if msg.Type != MsgServices {
			t.Fatalf("client got %q", msg.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client did not receive the control message")
	}
}

func TestDataStreamEcho(t *testing.T) {
	srv, pin := startServer(t, allowAuth(), 8)
	client := dial(t, srv, pin, "good")
	server := accept(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := client.OpenData(ctx, DataHeader{ServiceID: "svc-1", Port: 25565})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := stream.CloseWrite(); err != nil {
		t.Fatal(err)
	}

	incoming, err := server.AcceptData(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if incoming.Header.ServiceID != "svc-1" || incoming.Header.Port != 25565 {
		t.Fatalf("header = %+v", incoming.Header)
	}
	got, err := io.ReadAll(incoming)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("server read %q", got)
	}
	if _, err := incoming.Write([]byte("world")); err != nil {
		t.Fatal(err)
	}
	if err := incoming.CloseWrite(); err != nil {
		t.Fatal(err)
	}

	reply, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if string(reply) != "world" {
		t.Fatalf("client read %q", reply)
	}
}

func TestDatagramEcho(t *testing.T) {
	srv, pin := startServer(t, allowAuth(), 8)
	client := dial(t, srv, pin, "good")
	server := accept(t, srv)

	if err := client.SendDatagram([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := server.ReceiveDatagram(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("ping")) {
		t.Fatalf("server got %q", got)
	}

	if err := server.SendDatagram([]byte("pong")); err != nil {
		t.Fatal(err)
	}
	got, err = client.ReceiveDatagram(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("pong")) {
		t.Fatalf("client got %q", got)
	}
}

func TestStreamLimit(t *testing.T) {
	srv, pin := startServer(t, allowAuth(), 1)
	client := dial(t, srv, pin, "good")
	_ = accept(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	first, err := client.OpenData(ctx, DataHeader{ServiceID: "svc-1", Port: 25565})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	defer first.Close()

	blocked, cancelBlocked := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelBlocked()
	if _, err := client.OpenData(blocked, DataHeader{ServiceID: "svc-1", Port: 25565}); err == nil {
		t.Fatal("the stream limit was not enforced")
	}
}

func TestControlMessageLimit(t *testing.T) {
	long := strings.Repeat("x", maxControlMessage+1) + "\n"
	if _, err := readControl(bufio.NewReader(strings.NewReader(long))); err == nil {
		t.Fatal("an oversized control message was accepted")
	}
}

func TestServerCloseEndsConnection(t *testing.T) {
	srv, pin := startServer(t, allowAuth(), 8)
	client := dial(t, srv, pin, "good")
	_ = accept(t, srv)

	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.Context().Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the client connection stayed open after the server closed")
	}
}

func TestHandshakeTimeoutDropsSilentPeer(t *testing.T) {
	srv, pin := startServerConfig(t, allowAuth(), ServerConfig{
		HandshakeTimeout:     150 * time.Millisecond,
		MaxPendingHandshakes: 1,
	})
	qc, stream, err := rawDial(t, srv, pin)
	if err != nil {
		t.Fatal(err)
	}
	defer qc.CloseWithError(0, "test done")

	_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := stream.Read(make([]byte, 1)); err == nil {
		t.Fatal("a silent handshake stayed open past the deadline")
	}
	waitFor(t, 2*time.Second, func() bool {
		srv.mu.Lock()
		defer srv.mu.Unlock()
		return len(srv.pending) == 0
	})
}

func TestHandshakeLimitRejectsExtraPeers(t *testing.T) {
	srv, pin := startServerConfig(t, allowAuth(), ServerConfig{
		HandshakeTimeout:     5 * time.Second,
		MaxPendingHandshakes: 1,
	})
	qc, _, err := rawDial(t, srv, pin)
	if err != nil {
		t.Fatal(err)
	}
	defer qc.CloseWithError(0, "test done")
	waitFor(t, 2*time.Second, func() bool {
		srv.mu.Lock()
		defer srv.mu.Unlock()
		return len(srv.pending) == 1
	})

	extra, extraStream, err := rawDial(t, srv, pin)
	if err == nil {
		defer extra.CloseWithError(0, "test done")
		_ = extraStream.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := extraStream.Read(make([]byte, 1)); err == nil {
			t.Fatal("an extra pending handshake stayed open")
		}
	}
	srv.mu.Lock()
	pending := len(srv.pending)
	srv.mu.Unlock()
	if pending != 1 {
		t.Fatalf("pending handshakes = %d, want 1", pending)
	}
}

func TestServerCloseUnblocksPendingHandshake(t *testing.T) {
	srv, pin := startServerConfig(t, allowAuth(), ServerConfig{
		HandshakeTimeout:     time.Minute,
		MaxPendingHandshakes: 1,
	})
	qc, _, err := rawDial(t, srv, pin)
	if err != nil {
		t.Fatal(err)
	}
	defer qc.CloseWithError(0, "test done")
	waitFor(t, 2*time.Second, func() bool {
		srv.mu.Lock()
		defer srv.mu.Unlock()
		return len(srv.pending) == 1
	})

	done := make(chan struct{})
	go func() {
		_ = srv.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server close blocked on a pending handshake")
	}
}

func TestTunnelWithPacketConn(t *testing.T) {
	host := testIdentity(t, "host")
	serverConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := Listen(ServerConfig{
		Identity:        host,
		PacketConn:      serverConn,
		ApprovalTimeout: 3 * time.Second,
		Authenticate: authFunc(func(context.Context, AuthRequest) (AuthDecision, error) {
			return AuthDecision{Allow: true, MemberID: "friend"}, nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	clientConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, ClientConfig{
		Identity:   testIdentity(t, "packet-conn-client"),
		Endpoint:   srv.Addr().String(),
		PacketConn: clientConn,
		HostPin:    secure.Pin(host.Public),
		GroupID:    "group",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.MemberID != "friend" {
		t.Fatalf("member id = %q", c.MemberID)
	}
	if err := c.Send(ControlMessage{Type: MsgBye}); err != nil {
		t.Fatal(err)
	}
}

func TestTunnelCarriesRelayReconnectToken(t *testing.T) {
	sawHasToken := make(chan bool, 1)
	srv, pin := startServer(t, authFunc(func(_ context.Context, req AuthRequest) (AuthDecision, error) {
		sawHasToken <- req.HasRelayToken
		return AuthDecision{Allow: true, MemberID: "friend", RelayToken: "reconnect-token"}, nil
	}), 4)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, ClientConfig{
		Identity:      testIdentity(t, "relay-token-client"),
		Endpoint:      srv.Addr().String(),
		HostPin:       pin,
		GroupID:       "group-1",
		PeerName:      "Ash",
		DeviceName:    "ash-desktop",
		HasRelayToken: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !<-sawHasToken {
		t.Fatal("the host did not see the friend's reconnect-token flag")
	}
	if c.RelayToken != "reconnect-token" {
		t.Fatalf("relay token = %q", c.RelayToken)
	}
}
