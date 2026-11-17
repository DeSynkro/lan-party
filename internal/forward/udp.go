package forward

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// MaxUDPFlows is the per-service flow cap for one friend.
	MaxUDPFlows = 64
	// DefaultUDPIdle closes a quiet flow after a minute.
	DefaultUDPIdle = 60 * time.Second
	// MaxDatagram is the safe payload for a QUIC datagram on a 1280-byte path.
	MaxDatagram = 1200
	udpHeader   = 10 // published port (2) + flow id (8)
	// MaxUDPPayload is what is left for the game's packet.
	MaxUDPPayload = MaxDatagram - udpHeader
)

// FlowID identifies one game source address on the friend's side.
type FlowID uint64

type flowEntry struct {
	id   FlowID
	addr net.Addr
	last time.Time
}

// FlowTable maps game source addresses to flow ids and expires quiet ones.
type FlowTable struct {
	mu     sync.Mutex
	max    int
	idle   time.Duration
	next   FlowID
	byID   map[FlowID]*flowEntry
	byAddr map[string]*flowEntry
}

// NewFlowTable builds a table. Zero values get the defaults.
func NewFlowTable(max int, idle time.Duration) *FlowTable {
	if max <= 0 {
		max = MaxUDPFlows
	}
	if idle <= 0 {
		idle = DefaultUDPIdle
	}
	return &FlowTable{
		max:    max,
		idle:   idle,
		byID:   map[FlowID]*flowEntry{},
		byAddr: map[string]*flowEntry{},
	}
}

// New returns the flow for addr, creating one if there is room.
func (t *FlowTable) New(addr net.Addr) (FlowID, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := addr.String()
	if e, ok := t.byAddr[key]; ok {
		e.last = time.Now()
		return e.id, true
	}
	if len(t.byID) >= t.max {
		return 0, false
	}

	t.next++
	e := &flowEntry{id: t.next, addr: addr, last: time.Now()}
	t.byID[e.id] = e
	t.byAddr[key] = e
	return e.id, true
}

// Lookup returns the source address for a flow.
func (t *FlowTable) Lookup(id FlowID) (net.Addr, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.byID[id]
	if !ok {
		return nil, false
	}
	return e.addr, true
}

// Touch marks a flow as active.
func (t *FlowTable) Touch(id FlowID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.byID[id]; ok {
		e.last = time.Now()
	}
}

// Expire removes flows that have been quiet for the idle period.
func (t *FlowTable) Expire(now time.Time) []FlowID {
	t.mu.Lock()
	defer t.mu.Unlock()

	var expired []FlowID
	for id, e := range t.byID {
		if now.Sub(e.last) >= t.idle {
			delete(t.byID, id)
			delete(t.byAddr, e.addr.String())
			expired = append(expired, id)
		}
	}
	return expired
}

// Len is the number of live flows.
func (t *FlowTable) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.byID)
}

// EncodeDatagram puts the published port and flow id in front of a game
// packet, so the host knows which service and which game peer it belongs to.
func EncodeDatagram(port int, id FlowID, payload []byte) ([]byte, error) {
	if port < 1 || port > 65535 {
		return nil, errors.New("the datagram port is invalid")
	}
	if len(payload) > MaxUDPPayload {
		return nil, fmt.Errorf("the datagram is larger than %d bytes", MaxUDPPayload)
	}
	buf := make([]byte, udpHeader+len(payload))
	binary.BigEndian.PutUint16(buf[:2], uint16(port))
	binary.BigEndian.PutUint64(buf[2:udpHeader], uint64(id))
	copy(buf[udpHeader:], payload)
	return buf, nil
}

// DecodeDatagram reads the header written by EncodeDatagram.
func DecodeDatagram(buf []byte) (port int, id FlowID, payload []byte, err error) {
	if len(buf) < udpHeader {
		return 0, 0, nil, errors.New("the datagram is too short")
	}
	port = int(binary.BigEndian.Uint16(buf[:2]))
	if port < 1 {
		return 0, 0, nil, errors.New("the datagram port is invalid")
	}
	id = FlowID(binary.BigEndian.Uint64(buf[2:udpHeader]))
	return port, id, buf[udpHeader:], nil
}

// ServeFriendUDP reads game packets from a loopback socket, sends them through
// the tunnel, and writes replies back to the game. recv must carry only
// datagrams for this port.
func ServeFriendUDP(ctx context.Context, local *net.UDPConn, port int, send func([]byte) error, recv <-chan []byte) error {
	table := NewFlowTable(MaxUDPFlows, DefaultUDPIdle)
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = local.Close()
		case <-stop:
		}
	}()
	defer close(stop)

	go func() {
		for packet := range recv {
			p, id, payload, err := DecodeDatagram(packet)
			if err != nil || p != port {
				continue
			}
			addr, ok := table.Lookup(id)
			if !ok {
				continue
			}
			table.Touch(id)
			_, _ = local.WriteTo(payload, addr)
		}
	}()

	go expireFlows(ctx, table)

	buf := make([]byte, 65535)
	for {
		n, addr, err := local.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if n > MaxUDPPayload {
			continue
		}
		id, ok := table.New(addr)
		if !ok {
			continue
		}
		packet, err := EncodeDatagram(port, id, buf[:n])
		if err != nil {
			continue
		}
		if err := send(packet); err != nil {
			return err
		}
	}
}

// ServeHostUDP forwards one service's datagrams to its LAN target. recv must
// carry only datagrams for this port.
func ServeHostUDP(ctx context.Context, port int, send func([]byte) error, recv <-chan []byte, target string) error {
	targetAddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		return fmt.Errorf("target address: %w", err)
	}

	type hostFlow struct {
		conn *net.UDPConn
		last time.Time
	}

	flows := map[FlowID]*hostFlow{}
	var mu sync.Mutex

	closeFlows := func() {
		mu.Lock()
		for _, f := range flows {
			_ = f.conn.Close()
		}
		flows = map[FlowID]*hostFlow{}
		mu.Unlock()
	}
	go func() {
		<-ctx.Done()
		closeFlows()
	}()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				mu.Lock()
				for id, f := range flows {
					if now.Sub(f.last) >= DefaultUDPIdle {
						_ = f.conn.Close()
						delete(flows, id)
					}
				}
				mu.Unlock()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case packet, ok := <-recv:
			if !ok {
				return nil
			}
			p, id, payload, err := DecodeDatagram(packet)
			if err != nil || p != port {
				continue
			}

			mu.Lock()
			flow, ok := flows[id]
			if !ok {
				if len(flows) >= MaxUDPFlows {
					mu.Unlock()
					continue
				}
				conn, err := net.DialUDP("udp", nil, targetAddr)
				if err != nil {
					mu.Unlock()
					continue
				}
				flow = &hostFlow{conn: conn}
				flows[id] = flow
				go readHostReplies(id, flow.conn, port, send)
			}
			flow.last = time.Now()
			mu.Unlock()

			_, _ = flow.conn.Write(payload)
		}
	}
}

func readHostReplies(id FlowID, conn *net.UDPConn, port int, send func([]byte) error) {
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		packet, err := EncodeDatagram(port, id, buf[:n])
		if err != nil {
			continue
		}
		if err := send(packet); err != nil {
			return
		}
	}
}

func expireFlows(ctx context.Context, table *FlowTable) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			table.Expire(now)
		}
	}
}
