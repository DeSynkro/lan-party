package relay

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const defaultRelayPingInterval = 30 * time.Second

// ClientConfig is the host's relay control connection.
type ClientConfig struct {
	Address     string
	Fingerprint string
	Password    string
}

// FriendEvent reports a friend joining or leaving the relay session.
type FriendEvent struct {
	Type     string
	FriendID uint16
}

// Client is an authenticated host control connection to a relay.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader

	SessionID string
	DataToken []byte

	events chan FriendEvent

	writeMu     sync.Mutex
	authorizeMu sync.Mutex
	replyMu     sync.Mutex
	replyCh     chan controlMessage

	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	finishOnce sync.Once
	errMu      sync.Mutex
	err        error

	pingInterval time.Duration
}

// DialClient opens and authenticates a host control connection.
func DialClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.Address == "" {
		return nil, errors.New("the relay address is missing")
	}
	if cfg.Fingerprint == "" {
		return nil, errors.New("the Relay ID is missing")
	}
	if cfg.Password == "" {
		return nil, errors.New("the relay password is missing")
	}
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    ClientTLS(cfg.Fingerprint),
	}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("connect to relay: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	if err := writeControl(conn, controlMessage{
		Type:     "hello",
		Version:  Version,
		Role:     "host",
		Password: cfg.Password,
	}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send relay hello: %w", err)
	}
	welcome, err := readControl(reader)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read relay welcome: %w", err)
	}
	if welcome.Type != "welcome" {
		_ = conn.Close()
		return nil, fmt.Errorf("relay rejected the host: %s", welcome.Message)
	}
	dataToken, err := base64.RawURLEncoding.DecodeString(welcome.DataToken)
	if err != nil || len(dataToken) != tokenLength {
		_ = conn.Close()
		return nil, errors.New("relay sent an invalid data token")
	}
	_ = conn.SetDeadline(time.Time{})

	clientCtx, cancel := context.WithCancel(context.Background())
	c := &Client{
		conn:         conn,
		reader:       reader,
		SessionID:    welcome.SessionID,
		DataToken:    dataToken,
		events:       make(chan FriendEvent, 32),
		ctx:          clientCtx,
		cancel:       cancel,
		done:         make(chan struct{}),
		pingInterval: defaultRelayPingInterval,
	}
	go c.readLoop()
	go c.pingLoop()
	return c, nil
}

// Events reports friend joins and leaves. It is not closed; stop on Done.
func (c *Client) Events() <-chan FriendEvent { return c.events }

// Done closes when the control connection ends.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err is the last control connection error, if any.
func (c *Client) Err() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.err
}

// Ping keeps the control connection alive.
func (c *Client) Ping() error {
	return c.send(controlMessage{Type: "ping"})
}

// Authorize registers a one-time friend token with the relay. The expiry is
// capped by the relay. The token text is the value that goes into the invite.
func (c *Client) Authorize(ctx context.Context, token string, expires time.Time) error {
	if token == "" {
		return errors.New("the friend token is empty")
	}
	return c.request(ctx, controlMessage{Type: "authorize", Token: token, Expires: expires})
}

// AuthorizeHash registers a reusable reconnect token by its SHA-256 hash. The
// host keeps only the hash and never needs the raw token after issuing it.
func (c *Client) AuthorizeHash(ctx context.Context, hash string, reusable bool, expires time.Time) error {
	if hash == "" {
		return errors.New("the reconnect token hash is empty")
	}
	return c.request(ctx, controlMessage{
		Type:      "authorize",
		TokenHash: hash,
		Reusable:  reusable,
		Expires:   expires,
	})
}

// RevokeHash removes a token by its SHA-256 hash.
func (c *Client) RevokeHash(ctx context.Context, hash string) error {
	if hash == "" {
		return errors.New("the reconnect token hash is empty")
	}
	return c.request(ctx, controlMessage{Type: "revoke", TokenHash: hash})
}

func (c *Client) request(ctx context.Context, message controlMessage) error {
	c.authorizeMu.Lock()
	defer c.authorizeMu.Unlock()

	reply := make(chan controlMessage, 1)
	c.replyMu.Lock()
	c.replyCh = reply
	c.replyMu.Unlock()
	defer func() {
		c.replyMu.Lock()
		if c.replyCh == reply {
			c.replyCh = nil
		}
		c.replyMu.Unlock()
	}()

	if err := c.send(message); err != nil {
		return err
	}
	select {
	case msg := <-reply:
		if msg.Type != "ok" {
			return fmt.Errorf("relay: %s", msg.Message)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		if err := c.Err(); err != nil {
			return err
		}
		return errors.New("relay control connection closed")
	}
}

// Close says goodbye and closes the control connection.
func (c *Client) Close() error {
	c.writeMu.Lock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_ = writeControl(c.conn, controlMessage{Type: "bye"})
	c.writeMu.Unlock()
	c.finish(net.ErrClosed)
	return nil
}

func (c *Client) send(msg controlMessage) error {
	select {
	case <-c.done:
		if err := c.Err(); err != nil {
			return err
		}
		return errors.New("relay control connection closed")
	default:
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(defaultControlWrite))
	err := writeControl(c.conn, msg)
	_ = c.conn.SetWriteDeadline(time.Time{})
	if err != nil {
		c.finish(err)
	}
	return err
}

func (c *Client) readLoop() {
	for {
		msg, err := readControl(c.reader)
		if err != nil {
			c.finish(err)
			return
		}
		switch msg.Type {
		case "ok", "error":
			c.replyMu.Lock()
			reply := c.replyCh
			c.replyMu.Unlock()
			if reply != nil {
				select {
				case reply <- msg:
				default:
				}
			}
		case "friend":
			select {
			case c.events <- FriendEvent{Type: "friend", FriendID: msg.FriendID}:
			case <-c.ctx.Done():
				return
			}
		case "friendLeft":
			select {
			case c.events <- FriendEvent{Type: "friendLeft", FriendID: msg.FriendID}:
			case <-c.ctx.Done():
				return
			}
		case "pong":
			// Keepalive reply.
		}
	}
}

func (c *Client) pingLoop() {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.Ping(); err != nil {
				return
			}
		}
	}
}

func (c *Client) finish(err error) {
	c.finishOnce.Do(func() {
		c.errMu.Lock()
		if c.err == nil {
			c.err = err
		}
		c.errMu.Unlock()
		c.cancel()
		_ = c.conn.Close()
		close(c.done)
	})
}
