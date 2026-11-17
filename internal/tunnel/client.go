package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/desynkro/lan-party/internal/secure"
	"github.com/quic-go/quic-go"
)

// ClientConfig is the friend's half of the handshake.
type ClientConfig struct {
	Identity   secure.Identity
	Endpoint   string
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
		return nil, errors.New("the host address is missing")
	}
	if cfg.GroupID == "" {
		return nil, errors.New("the group id is missing")
	}

	tlsConf, err := clientTLS(cfg.Identity, cfg.HostPin)
	if err != nil {
		return nil, err
	}
	qc, err := quic.DialAddr(ctx, cfg.Endpoint, tlsConf, clientQUICConfig())
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	stream, err := qc.OpenStreamSync(ctx)
	if err != nil {
		_ = qc.CloseWithError(1, "control stream failed")
		return nil, fmt.Errorf("control stream: %w", err)
	}
	enc := json.NewEncoder(stream)
	dec := json.NewDecoder(stream)

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
		return nil, fmt.Errorf("send hello: %w", err)
	}

	reply, err := readReply(ctx, dec)
	if err != nil {
		_ = qc.CloseWithError(1, "handshake failed")
		return nil, err
	}

	switch reply.Type {
	case MsgDenied:
		_ = qc.CloseWithError(0, "denied")
		return nil, fmt.Errorf("%w: %s", ErrDenied, reply.Reason)
	case MsgWelcome:
		if reply.MemberID == "" {
			_ = qc.CloseWithError(1, "missing member id")
			return nil, errors.New("the host did not assign a member id")
		}
	default:
		_ = qc.CloseWithError(1, "unexpected reply")
		return nil, errors.New("the host sent an unexpected reply")
	}

	c := newConn(qc, stream, dec, enc, nil)
	c.MemberID = reply.MemberID
	c.Services = reply.Services
	return c, nil
}

type replyResult struct {
	msg ControlMessage
	err error
}

func readReply(ctx context.Context, dec *json.Decoder) (ControlMessage, error) {
	ch := make(chan replyResult, 1)
	go func() {
		var msg ControlMessage
		err := dec.Decode(&msg)
		ch <- replyResult{msg: msg, err: err}
	}()

	select {
	case r := <-ch:
		return r.msg, r.err
	case <-ctx.Done():
		return ControlMessage{}, ctx.Err()
	}
}
