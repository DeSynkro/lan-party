package tunnel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/desynkro/lan-party/internal/secure"
	"github.com/quic-go/quic-go"
)

// ClientConfig is the friend's half of the handshake.
type ClientConfig struct {
	Identity   secure.Identity
	Endpoint   string
	PacketConn net.PacketConn // optional; the client takes ownership
	HostPin    string
	GroupID    string
	MemberID   string
	PeerName   string
	DeviceName string
	Token      string
}

// Dial connects, verifies the host pin, and waits for the host's answer.
func Dial(ctx context.Context, cfg ClientConfig) (*Conn, error) {
	if cfg.Endpoint == "" {
		if cfg.PacketConn != nil {
			_ = cfg.PacketConn.Close()
		}
		return nil, errors.New("the host address is missing")
	}
	if cfg.GroupID == "" {
		if cfg.PacketConn != nil {
			_ = cfg.PacketConn.Close()
		}
		return nil, errors.New("the group id is missing")
	}

	tlsConf, err := clientTLS(cfg.Identity, cfg.HostPin)
	if err != nil {
		if cfg.PacketConn != nil {
			_ = cfg.PacketConn.Close()
		}
		return nil, err
	}
	var qc *quic.Conn
	var transport *quic.Transport
	if cfg.PacketConn != nil {
		relayAddr, err := net.ResolveUDPAddr("udp", cfg.Endpoint)
		if err != nil {
			_ = cfg.PacketConn.Close()
			return nil, fmt.Errorf("relay address: %w", err)
		}
		transport = &quic.Transport{Conn: cfg.PacketConn}
		qc, err = transport.Dial(ctx, relayAddr, tlsConf, clientQUICConfig())
		if err != nil {
			_ = transport.Close()
			_ = cfg.PacketConn.Close()
			return nil, fmt.Errorf("connect: %w", err)
		}
	} else {
		qc, err = quic.DialAddr(ctx, cfg.Endpoint, tlsConf, clientQUICConfig())
		if err != nil {
			return nil, fmt.Errorf("connect: %w", err)
		}
	}
	closeTransport := func() {
		if transport != nil {
			_ = transport.Close()
		}
		if cfg.PacketConn != nil {
			_ = cfg.PacketConn.Close()
		}
	}

	stream, err := qc.OpenStreamSync(ctx)
	if err != nil {
		_ = qc.CloseWithError(1, "control stream failed")
		closeTransport()
		return nil, fmt.Errorf("control stream: %w", err)
	}
	enc := json.NewEncoder(stream)
	reader := bufio.NewReader(stream)

	hello := ControlMessage{
		Type:       MsgHello,
		Version:    ProtocolVersion,
		GroupID:    cfg.GroupID,
		MemberID:   cfg.MemberID,
		PeerName:   cfg.PeerName,
		DeviceName: cfg.DeviceName,
		Token:      cfg.Token,
	}
	if err := enc.Encode(hello); err != nil {
		_ = qc.CloseWithError(1, "hello failed")
		closeTransport()
		return nil, fmt.Errorf("send hello: %w", err)
	}

	reply, err := readReply(ctx, reader)
	if err != nil {
		_ = qc.CloseWithError(1, "handshake failed")
		closeTransport()
		return nil, err
	}

	switch reply.Type {
	case MsgDenied:
		_ = qc.CloseWithError(0, "denied")
		closeTransport()
		return nil, fmt.Errorf("%w: %s", ErrDenied, clipText(reply.Reason, 200))
	case MsgWelcome:
		if reply.MemberID == "" {
			_ = qc.CloseWithError(1, "missing member id")
			closeTransport()
			return nil, errors.New("the host did not assign a member id")
		}
	default:
		_ = qc.CloseWithError(1, "unexpected reply")
		closeTransport()
		return nil, errors.New("the host sent an unexpected reply")
	}

	c := newConn(qc, stream, reader, enc, nil)
	c.transport = transport
	c.packetConn = cfg.PacketConn
	c.MemberID = reply.MemberID
	c.Services = reply.Services
	c.start()
	return c, nil
}

type replyResult struct {
	msg ControlMessage
	err error
}

func readReply(ctx context.Context, reader *bufio.Reader) (ControlMessage, error) {
	ch := make(chan replyResult, 1)
	go func() {
		msg, err := readControl(reader)
		ch <- replyResult{msg: msg, err: err}
	}()

	select {
	case r := <-ch:
		return r.msg, r.err
	case <-ctx.Done():
		return ControlMessage{}, ctx.Err()
	}
}
