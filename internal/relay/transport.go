package relay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// HostPacketConn adapts the host's single relay UDP socket to a net.PacketConn.
// It presents a stable synthetic address per relay friend ID so quic-go treats
// each friend as a separate client.
type HostPacketConn struct {
	conn      *net.UDPConn
	relay     *net.UDPAddr
	dataToken [tokenLength]byte
	readBuf   []byte
	readMu    sync.Mutex

	mu       sync.Mutex
	friends  map[uint16]*net.UDPAddr
	byAddr   map[string]uint16
	nextPort int

	closeOnce sync.Once
	closeErr  error
}

// NewHostPacketConn opens a local UDP socket for relay data. local may be nil
// to bind all interfaces on an ephemeral port. dataToken is the token from the
// relay welcome message.
func NewHostPacketConn(local, relay *net.UDPAddr, dataToken []byte) (*HostPacketConn, error) {
	if relay == nil || relay.IP == nil || relay.Port == 0 {
		return nil, errors.New("a complete relay address is required")
	}
	if len(dataToken) != tokenLength {
		return nil, errors.New("the relay data token is invalid")
	}
	network := "udp"
	if local == nil {
		if relay.IP.To4() != nil {
			local = &net.UDPAddr{IP: net.IPv4zero}
			network = "udp4"
		} else {
			local = &net.UDPAddr{IP: net.IPv6unspecified}
			network = "udp6"
		}
	} else if local.IP != nil {
		if local.IP.To4() != nil {
			network = "udp4"
		} else {
			network = "udp6"
		}
	}
	conn, err := net.ListenUDP(network, local)
	if err != nil {
		return nil, fmt.Errorf("listen for relay data: %w", err)
	}
	c := &HostPacketConn{
		conn:     conn,
		relay:    cloneUDPAddr(relay),
		readBuf:  make([]byte, maxRelayPayload+1+friendIDLength),
		friends:  map[uint16]*net.UDPAddr{},
		byAddr:   map[string]uint16{},
		nextPort: 29999,
	}
	copy(c.dataToken[:], dataToken)
	return c, nil
}

// SendKeepalive registers the host data path and refreshes its NAT mapping.
func (c *HostPacketConn) SendKeepalive() error {
	packet := make([]byte, 1+tokenLength)
	packet[0] = packetHost
	copy(packet[1:], c.dataToken[:])
	_, err := c.conn.WriteToUDP(packet, c.relay)
	return err
}

// FriendAddress returns the stable synthetic address for a relay friend ID.
func (c *HostPacketConn) FriendAddress(id uint16) *net.UDPAddr {
	c.mu.Lock()
	defer c.mu.Unlock()
	if addr := c.friends[id]; addr != nil {
		return addr
	}
	for {
		c.nextPort++
		if c.nextPort < 30000 || c.nextPort > 65000 {
			c.nextPort = 30000
		}
		addr := &net.UDPAddr{IP: append(net.IP(nil), c.relay.IP...), Port: c.nextPort}
		key := addr.String()
		if _, used := c.byAddr[key]; used || addr.Port == c.relay.Port {
			continue
		}
		c.friends[id] = addr
		c.byAddr[key] = id
		return addr
	}
}

// RemoveFriend drops the synthetic address for a friend that left.
func (c *HostPacketConn) RemoveFriend(id uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	addr := c.friends[id]
	if addr == nil {
		return
	}
	delete(c.friends, id)
	delete(c.byAddr, addr.String())
}

func (c *HostPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		n, addr, err := c.conn.ReadFromUDP(c.readBuf)
		if err != nil {
			return 0, nil, err
		}
		if !sameUDPAddr(addr, c.relay) {
			continue
		}
		if n < 1+friendIDLength || c.readBuf[0] != packetRelay {
			continue
		}
		id := binary.BigEndian.Uint16(c.readBuf[1 : 1+friendIDLength])
		payload := c.readBuf[1+friendIDLength : n]
		if len(payload) == 0 {
			continue
		}
		return copy(p, payload), c.FriendAddress(id), nil
	}
}

func (c *HostPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if addr == nil {
		return 0, errors.New("relay friend address is missing")
	}
	if len(p) > maxRelayPayload {
		return 0, errors.New("relay packet is too large")
	}
	c.mu.Lock()
	id, ok := c.byAddr[addr.String()]
	c.mu.Unlock()
	if !ok {
		return 0, fmt.Errorf("unknown relay friend address %s", addr)
	}
	packet := make([]byte, 0, 1+tokenLength+friendIDLength+len(p))
	packet = append(packet, packetHost)
	packet = append(packet, c.dataToken[:]...)
	var friendID [friendIDLength]byte
	binary.BigEndian.PutUint16(friendID[:], id)
	packet = append(packet, friendID[:]...)
	packet = append(packet, p...)
	if _, err := c.conn.WriteToUDP(packet, c.relay); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *HostPacketConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

func (c *HostPacketConn) LocalAddr() net.Addr { return c.conn.LocalAddr() }

func (c *HostPacketConn) SetDeadline(t time.Time) error { return c.conn.SetDeadline(t) }

func (c *HostPacketConn) SetReadDeadline(t time.Time) error { return c.conn.SetReadDeadline(t) }

func (c *HostPacketConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

func (c *HostPacketConn) SetReadBuffer(bytes int) error { return c.conn.SetReadBuffer(bytes) }

func (c *HostPacketConn) SetWriteBuffer(bytes int) error { return c.conn.SetWriteBuffer(bytes) }

// FriendPacketConn adapts the friend's UDP socket to a net.PacketConn. It adds
// the friend token to every outgoing packet.
type FriendPacketConn struct {
	conn      *net.UDPConn
	relay     *net.UDPAddr
	token     [tokenLength]byte
	readBuf   []byte
	readMu    sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// NewFriendPacketConn opens a local UDP socket for relay data. local may be
// nil to bind all interfaces on an ephemeral port. token is the raw friend
// token from the invite.
func NewFriendPacketConn(local, relay *net.UDPAddr, token []byte) (*FriendPacketConn, error) {
	if relay == nil || relay.IP == nil || relay.Port == 0 {
		return nil, errors.New("a complete relay address is required")
	}
	if len(token) != tokenLength {
		return nil, errors.New("the friend token is invalid")
	}
	network := "udp"
	if local == nil {
		if relay.IP.To4() != nil {
			local = &net.UDPAddr{IP: net.IPv4zero}
			network = "udp4"
		} else {
			local = &net.UDPAddr{IP: net.IPv6unspecified}
			network = "udp6"
		}
	} else if local.IP != nil {
		if local.IP.To4() != nil {
			network = "udp4"
		} else {
			network = "udp6"
		}
	}
	conn, err := net.ListenUDP(network, local)
	if err != nil {
		return nil, fmt.Errorf("listen for relay data: %w", err)
	}
	c := &FriendPacketConn{
		conn:    conn,
		relay:   cloneUDPAddr(relay),
		readBuf: make([]byte, maxRelayPayload),
	}
	copy(c.token[:], token)
	return c, nil
}

func (c *FriendPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		n, addr, err := c.conn.ReadFromUDP(c.readBuf)
		if err != nil {
			return 0, nil, err
		}
		if !sameUDPAddr(addr, c.relay) {
			continue
		}
		return copy(p, c.readBuf[:n]), c.relay, nil
	}
}

func (c *FriendPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if addr == nil {
		return 0, errors.New("relay destination is missing")
	}
	udp, ok := addr.(*net.UDPAddr)
	if !ok || !sameUDPAddr(udp, c.relay) {
		return 0, fmt.Errorf("unexpected relay destination %s", addr)
	}
	if len(p) > maxRelayPayload {
		return 0, errors.New("relay packet is too large")
	}
	packet := make([]byte, 0, 1+tokenLength+len(p))
	packet = append(packet, packetFriend)
	packet = append(packet, c.token[:]...)
	packet = append(packet, p...)
	if _, err := c.conn.WriteToUDP(packet, c.relay); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *FriendPacketConn) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.conn.Close()
	})
	return c.closeErr
}

func (c *FriendPacketConn) LocalAddr() net.Addr { return c.conn.LocalAddr() }

func (c *FriendPacketConn) SetDeadline(t time.Time) error { return c.conn.SetDeadline(t) }

func (c *FriendPacketConn) SetReadDeadline(t time.Time) error { return c.conn.SetReadDeadline(t) }

func (c *FriendPacketConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

func (c *FriendPacketConn) SetReadBuffer(bytes int) error { return c.conn.SetReadBuffer(bytes) }

func (c *FriendPacketConn) SetWriteBuffer(bytes int) error { return c.conn.SetWriteBuffer(bytes) }

var (
	_ net.PacketConn = (*HostPacketConn)(nil)
	_ net.PacketConn = (*FriendPacketConn)(nil)
)
