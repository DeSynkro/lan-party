package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/secure"
	"github.com/quic-go/quic-go"
)

func TestRelayQuicTransportEndToEnd(t *testing.T) {
	relay := startTestRelay(t, nil)
	relayAddr := relayDataAddr(t, relay)

	host := dialTestHost(t, relay, testRelayPassword)
	_, tokenText, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	friendToken, err := DecodeFriendToken(tokenText)
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, tokenText)

	hostConn, err := NewHostPacketConn(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, relayAddr, host.dataToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hostConn.Close() })
	if err := hostConn.SendKeepalive(); err != nil {
		t.Fatal(err)
	}

	friendConn, err := NewFriendPacketConn(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, relayAddr, friendToken)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = friendConn.Close() })

	hostIdentity := newTransportIdentity(t)
	hostCert, err := hostIdentity.Certificate(x509.ExtKeyUsageServerAuth)
	if err != nil {
		t.Fatal(err)
	}
	hostPin := secure.Pin(hostIdentity.Public)
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{hostCert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"lan-party-relay-test"},
	}
	clientTLS := &tls.Config{
		InsecureSkipVerify: true, // the host pin below replaces CA checks
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{"lan-party-relay-test"},
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("host sent no certificate")
			}
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			pub, err := secure.PublicKeyFromCertificate(cert)
			if err != nil {
				return err
			}
			if !secure.PinMatches(pub, hostPin) {
				return errors.New("host pin mismatch")
			}
			return nil
		},
	}

	quicConfig := &quic.Config{
		HandshakeIdleTimeout: 5 * time.Second,
		MaxIdleTimeout:       10 * time.Second,
		KeepAlivePeriod:      2 * time.Second,
	}
	hostTransport := &quic.Transport{Conn: hostConn}
	t.Cleanup(func() { _ = hostTransport.Close() })
	listener, err := hostTransport.Listen(serverTLS, quicConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	friendTransport := &quic.Transport{Conn: friendConn}
	t.Cleanup(func() { _ = friendTransport.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type acceptResult struct {
		conn *quic.Conn
		err  error
	}
	accepted := make(chan acceptResult, 1)
	go func() {
		conn, err := listener.Accept(ctx)
		accepted <- acceptResult{conn: conn, err: err}
	}()

	friendQuic, err := friendTransport.Dial(ctx, relayAddr, clientTLS, quicConfig)
	if err != nil {
		t.Fatalf("friend dial: %v", err)
	}
	defer friendQuic.CloseWithError(0, "done")

	result := <-accepted
	if result.err != nil {
		t.Fatalf("host accept: %v", result.err)
	}
	hostQuic := result.conn
	defer hostQuic.CloseWithError(0, "done")

	friendStream, err := friendQuic.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("friend stream: %v", err)
	}
	_ = friendStream.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := friendStream.Write([]byte("hello through the relay")); err != nil {
		t.Fatalf("friend write: %v", err)
	}
	if err := friendStream.Close(); err != nil {
		t.Fatalf("friend close write: %v", err)
	}

	hostStream, err := hostQuic.AcceptStream(ctx)
	if err != nil {
		t.Fatalf("host stream: %v", err)
	}
	_ = hostStream.SetDeadline(time.Now().Add(5 * time.Second))
	data, err := io.ReadAll(hostStream)
	if err != nil {
		t.Fatalf("host read: %v", err)
	}
	if string(data) != "hello through the relay" {
		t.Fatalf("host received %q", data)
	}
	if _, err := hostStream.Write([]byte("reply through the relay")); err != nil {
		t.Fatalf("host write: %v", err)
	}
	if err := hostStream.Close(); err != nil {
		t.Fatalf("host close write: %v", err)
	}

	reply, err := io.ReadAll(friendStream)
	if err != nil {
		t.Fatalf("friend read: %v", err)
	}
	if string(reply) != "reply through the relay" {
		t.Fatalf("friend received %q", reply)
	}
}

func newTransportIdentity(t *testing.T) secure.Identity {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return secure.Identity{Private: priv, Public: pub}
}

func TestHostPacketConnRemoveFriend(t *testing.T) {
	relay := startTestRelay(t, nil)
	conn, err := NewHostPacketConn(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}, relayDataAddr(t, relay), make([]byte, tokenLength))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	addr := conn.FriendAddress(42)
	conn.RemoveFriend(42)
	if _, err := conn.WriteTo([]byte("late"), addr); err == nil {
		t.Fatal("a removed friend address was still accepted")
	}
}
