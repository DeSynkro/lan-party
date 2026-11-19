package session

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/desynkro/lan-party/internal/forward"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/tunnel"
)

// Presence is the live state of one peer.
type Presence struct {
	Live       bool
	RTT        time.Duration
	LastSeen   time.Time
	RemoteAddr string
}

const (
	maxHostPeers      = 64
	maxPeersPerMember = 8
)

// HostConfig is everything needed to publish a group.
type HostConfig struct {
	Identity     secure.Identity
	GroupID      string
	AppPort      int
	Services     []group.Service
	Authenticate tunnel.Authenticator
}

// Host is a running session on the host.
type Host struct {
	server   *tunnel.Server
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	services serviceIndex
	peers    map[*hostPeer]struct{}
}

type hostPeer struct {
	conn     *tunnel.Conn
	memberID string
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	udp      map[int]*hostUDP
}

type hostUDP struct {
	target string
	recv   chan []byte
	cancel context.CancelFunc
}

// StartHost opens the tunnel and starts forwarding published services.
func StartHost(ctx context.Context, cfg HostConfig) (*Host, error) {
	if cfg.AppPort < 1 || cfg.AppPort > 65535 {
		return nil, fmt.Errorf("the app port must be between 1 and 65535")
	}
	server, err := tunnel.Listen(tunnel.ServerConfig{
		Identity:     cfg.Identity,
		ListenAddr:   net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.AppPort)),
		Authenticate: cfg.Authenticate,
	})
	if err != nil {
		return nil, err
	}

	hctx, cancel := context.WithCancel(ctx)
	h := &Host{
		server:   server,
		ctx:      hctx,
		cancel:   cancel,
		services: newServiceIndex(cfg.Services),
		peers:    map[*hostPeer]struct{}{},
	}
	go h.acceptLoop()
	return h, nil
}

// UpdateServices publishes a new service list and tells every connected peer.
func (h *Host) UpdateServices(services []group.Service) error {
	index := newServiceIndex(services)
	infos := ServiceInfos(services)

	h.mu.Lock()
	h.services = index
	peers := make([]*hostPeer, 0, len(h.peers))
	for p := range h.peers {
		peers = append(peers, p)
	}
	h.mu.Unlock()

	for _, p := range peers {
		if err := p.conn.Send(tunnel.ControlMessage{
			Type:     tunnel.MsgServices,
			Services: infos,
		}); err != nil {
			p.cancel()
		}
		p.dropUDP(index.hasUDP)
	}
	return nil
}

// Services returns the services currently published by the host.
func (h *Host) Services() []group.Service {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]group.Service, 0, len(h.services.byID))
	for _, service := range h.services.byID {
		service.Ports = append([]int(nil), service.Ports...)
		out = append(out, service)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Presence returns the live state of every connected member.
func (h *Host) Presence() map[string]Presence {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make(map[string]Presence, len(h.peers))
	for p := range h.peers {
		if p.memberID == "" {
			continue
		}
		live := p.ctx.Err() == nil && p.conn.Context().Err() == nil
		out[p.memberID] = Presence{
			Live:       live,
			RTT:        p.conn.RTT(),
			LastSeen:   time.Now(),
			RemoteAddr: p.conn.RemoteAddr().String(),
		}
	}
	return out
}

// Revoke closes every connection belonging to a member.
func (h *Host) Revoke(memberID string) error {
	h.mu.Lock()
	var peers []*hostPeer
	for p := range h.peers {
		if p.memberID == memberID {
			peers = append(peers, p)
		}
	}
	h.mu.Unlock()

	for _, p := range peers {
		_ = p.conn.Send(tunnel.ControlMessage{Type: tunnel.MsgRevoked})
		p.cancel()
		_ = p.conn.Close()
	}
	return nil
}

// Close ends the session and every peer connection.
func (h *Host) Close() error {
	h.cancel()
	_ = h.server.Close()

	h.mu.Lock()
	for p := range h.peers {
		p.cancel()
	}
	h.mu.Unlock()
	return nil
}

func (h *Host) acceptLoop() {
	for {
		conn, err := h.server.Accept(h.ctx)
		if err != nil {
			return
		}

		peerCtx, peerCancel := context.WithCancel(conn.Context())
		p := &hostPeer{
			conn:     conn,
			memberID: conn.MemberID,
			ctx:      peerCtx,
			cancel:   peerCancel,
			udp:      map[int]*hostUDP{},
		}

		if !h.trackPeer(p) {
			p.cancel()
			_ = conn.Close()
			continue
		}

		go func() {
			<-p.ctx.Done()
			h.removePeer(p)
		}()
		go h.control(p)
		go h.streams(p)
		go h.datagrams(p)
	}
}

func (h *Host) trackPeer(p *hostPeer) bool {
	if p.memberID == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.peers) >= maxHostPeers {
		return false
	}
	sameMember := 0
	for peer := range h.peers {
		if peer.memberID == p.memberID {
			sameMember++
		}
	}
	if sameMember >= maxPeersPerMember {
		return false
	}
	h.peers[p] = struct{}{}
	return true
}

func (h *Host) control(p *hostPeer) {
	for {
		select {
		case <-p.ctx.Done():
			return
		case msg, ok := <-p.conn.Messages():
			if !ok {
				p.cancel()
				return
			}
			switch msg.Type {
			case tunnel.MsgBye:
				p.cancel()
				return
			}
		}
	}
}

func (h *Host) streams(p *hostPeer) {
	for {
		stream, err := p.conn.AcceptData(p.ctx)
		if err != nil {
			return
		}
		go h.handleStream(p, stream)
	}
}

func (h *Host) handleStream(p *hostPeer, stream *tunnel.DataStream) {
	svc, ok := h.serviceFor(stream.Header.ServiceID, stream.Header.Port)
	if !ok {
		stream.CancelRead(0)
		stream.CancelWrite(0)
		return
	}
	target, err := svc.TargetFor(stream.Header.Port)
	if err != nil {
		stream.CancelRead(0)
		stream.CancelWrite(0)
		return
	}

	dialCtx, cancel := context.WithTimeout(p.ctx, 10*time.Second)
	defer cancel()
	_ = forward.HandleTCP(stream, func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", target)
	}, dialCtx)
}

func (h *Host) datagrams(p *hostPeer) {
	for {
		packet, err := p.conn.ReceiveDatagram(p.ctx)
		if err != nil {
			return
		}
		port, _, _, err := forward.DecodeDatagram(packet)
		if err != nil {
			continue
		}
		svc, ok := h.serviceForUDP(port)
		if !ok {
			continue
		}
		target, err := svc.TargetFor(port)
		if err != nil {
			continue
		}
		p.forwardUDP(port, target, packet)
	}
}

func (p *hostPeer) forwardUDP(port int, target string, packet []byte) {
	p.mu.Lock()
	fwd, ok := p.udp[port]
	if ok && fwd.target != target {
		fwd.cancel()
		delete(p.udp, port)
		ok = false
	}
	if !ok {
		ctx, cancel := context.WithCancel(p.ctx)
		fwd = &hostUDP{
			target: target,
			recv:   make(chan []byte, 32),
			cancel: cancel,
		}
		p.udp[port] = fwd
		go forward.ServeHostUDP(ctx, port, p.conn.SendDatagram, fwd.recv, target)
	}
	p.mu.Unlock()

	select {
	case fwd.recv <- packet:
	default:
	}
}

func (p *hostPeer) dropUDP(has func(int) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for port, fwd := range p.udp {
		if !has(port) {
			fwd.cancel()
			delete(p.udp, port)
		}
	}
}

func (h *Host) removePeer(p *hostPeer) {
	p.cancel()

	p.mu.Lock()
	for _, fwd := range p.udp {
		fwd.cancel()
	}
	p.udp = map[int]*hostUDP{}
	p.mu.Unlock()

	_ = p.conn.Close()

	h.mu.Lock()
	delete(h.peers, p)
	h.mu.Unlock()
}

func (h *Host) serviceFor(id string, port int) (group.Service, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.services.forStream(id, port)
}

func (h *Host) serviceForUDP(port int) (group.Service, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.services.forUDP(port)
}

// ServiceInfos converts the group's services to the tunnel's copy.
func ServiceInfos(services []group.Service) []tunnel.ServiceInfo {
	out := make([]tunnel.ServiceInfo, 0, len(services))
	for _, s := range services {
		out = append(out, tunnel.ServiceInfo{
			ID:    s.ID,
			Name:  s.Name,
			Proto: s.Proto,
			Ports: append([]int(nil), s.Ports...),
			Hint:  s.Hint,
		})
	}
	return out
}

type serviceIndex struct {
	byID     map[string]group.Service
	udpPorts map[int]group.Service
}

func newServiceIndex(services []group.Service) serviceIndex {
	index := serviceIndex{
		byID:     make(map[string]group.Service, len(services)),
		udpPorts: map[int]group.Service{},
	}
	for _, s := range services {
		index.byID[s.ID] = s
		if s.Proto == "udp" || s.Proto == "both" {
			for _, port := range s.Ports {
				index.udpPorts[port] = s
			}
		}
	}
	return index
}

func (i serviceIndex) forStream(id string, port int) (group.Service, bool) {
	svc, ok := i.byID[id]
	if !ok || svc.Proto == "udp" {
		return group.Service{}, false
	}
	for _, p := range svc.Ports {
		if p == port {
			return svc, true
		}
	}
	return group.Service{}, false
}

func (i serviceIndex) forUDP(port int) (group.Service, bool) {
	svc, ok := i.udpPorts[port]
	return svc, ok
}

func (i serviceIndex) hasUDP(port int) bool {
	_, ok := i.udpPorts[port]
	return ok
}
