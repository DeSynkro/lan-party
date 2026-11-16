// Package session ties the tunnel and the forwarder together. A host publishes
// services on the box address; a friend opens loopback ports that lead to them.
package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/forward"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/tunnel"
)

// BoxAddress is where the host listens inside the tunnel. Peers dial this.
const BoxAddress = "100.64.0.1"

// HostConfig is everything needed to publish a session.
type HostConfig struct {
	PrivateKey wgtypes.Key
	ListenPort uint16
	Peers      []tunnel.Peer
	Services   []group.Service
}

// Host is a running session on the host.
type Host struct {
	dev    *tunnel.Device
	cancel context.CancelFunc
	once   sync.Once
}

// StartHost brings the tunnel up and forwards every published port.
func StartHost(cfg HostConfig) (*Host, error) {
	dev, err := tunnel.Start(tunnel.Config{
		PrivateKey: cfg.PrivateKey,
		ListenPort: cfg.ListenPort,
		Addresses:  []net.IPNet{mustCIDR(BoxAddress + "/32")},
		Peers:      cfg.Peers,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	h := &Host{dev: dev, cancel: cancel}

	var dialer net.Dialer
	for _, svc := range cfg.Services {
		for _, port := range svc.Ports {
			target := targetFor(svc, port)

			if svc.Proto == "tcp" || svc.Proto == "both" {
				ln, err := dev.Listen(boxAddr(port))
				if err != nil {
					h.Close()
					return nil, fmt.Errorf("publish %s tcp %d: %w", svc.Name, port, err)
				}
				go func() { _ = forward.ServeTCP(ctx, ln, target, dialer.DialContext) }()
			}

			if svc.Proto == "udp" || svc.Proto == "both" {
				pc, err := dev.ListenPacket(boxAddr(port))
				if err != nil {
					h.Close()
					return nil, fmt.Errorf("publish %s udp %d: %w", svc.Name, port, err)
				}
				go func() { _ = forward.ServeUDP(ctx, pc, target, dialer.DialContext) }()
			}
		}
	}

	return h, nil
}

// Close ends the session. Every listener and the tunnel go with it.
func (h *Host) Close() error {
	h.once.Do(func() {
		h.cancel()
		h.dev.Close()
	})
	return nil
}

// AddPeer lets someone in while the session is already running.
func (h *Host) AddPeer(p tunnel.Peer) error {
	return h.dev.AddPeer(p)
}

// Presence reports the last handshake for every peer, keyed by public key. A
// zero time means that peer has never connected.
func (h *Host) Presence() map[string]time.Time {
	peers, err := h.dev.Peers()
	if err != nil {
		return map[string]time.Time{}
	}
	out := make(map[string]time.Time, len(peers))
	for _, p := range peers {
		out[p.PublicKey.String()] = p.LastHandshake
	}
	return out
}

// FriendConfig is what the friend's app starts with.
type FriendConfig struct {
	Invite invite.Invite
}

// Bound is one local address the friend can point a game at.
type Bound struct {
	Service string
	Port    int
	Proto   string
	Address string
}

// Friend is a running session on the friend's side.
type Friend struct {
	dev    *tunnel.Device
	cancel context.CancelFunc
	once   sync.Once
	bound  []Bound
}

// StartFriend brings the tunnel up and opens a loopback port per service.
func StartFriend(cfg FriendConfig) (*Friend, error) {
	if !cfg.Invite.Expires.IsZero() && time.Now().After(cfg.Invite.Expires) {
		return nil, errors.New("this invite has expired")
	}

	hostKey, peerKey, err := cfg.Invite.Keys()
	if err != nil {
		return nil, err
	}

	peerBox, err := parseBox(cfg.Invite.PeerBox)
	if err != nil {
		return nil, err
	}

	allowed, err := parseCIDR(BoxAddress + "/32")
	if err != nil {
		return nil, err
	}

	dev, err := tunnel.Start(tunnel.Config{
		PrivateKey: peerKey,
		ListenPort: 0,
		Addresses:  []net.IPNet{peerBox},
		Peers: []tunnel.Peer{{
			PublicKey:  hostKey,
			Endpoint:   cfg.Invite.Endpoint,
			AllowedIPs: []net.IPNet{allowed},
		}},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	f := &Friend{dev: dev, cancel: cancel}

	for _, svc := range cfg.Invite.Services {
		for _, port := range svc.Ports {
			target := boxAddr(port)

			if svc.Proto == "tcp" || svc.Proto == "both" {
				ln, addr, err := listenLoopback(port)
				if err != nil {
					f.Close()
					return nil, fmt.Errorf("open %s tcp %d: %w", svc.Name, port, err)
				}
				go func() { _ = forward.ServeTCP(ctx, ln, target, dev.DialContext) }()
				f.bound = append(f.bound, Bound{Service: svc.Name, Port: port, Proto: "tcp", Address: addr})
			}

			if svc.Proto == "udp" || svc.Proto == "both" {
				pc, addr, err := listenLoopbackPacket(port)
				if err != nil {
					f.Close()
					return nil, fmt.Errorf("open %s udp %d: %w", svc.Name, port, err)
				}
				go func() { _ = forward.ServeUDP(ctx, pc, target, dev.DialContext) }()
				f.bound = append(f.bound, Bound{Service: svc.Name, Port: port, Proto: "udp", Address: addr})
			}
		}
	}

	return f, nil
}

// Bound reports the local addresses a game can connect to.
func (f *Friend) Bound() []Bound { return append([]Bound(nil), f.bound...) }

// Close ends the friend's side.
func (f *Friend) Close() error {
	f.once.Do(func() {
		f.cancel()
		f.dev.Close()
	})
	return nil
}

// targetFor works out where a published port should go on the host's LAN. A
// single port service uses the address as typed, so a server on 25566 can still
// appear to friends as 25565.
func targetFor(svc group.Service, port int) string {
	if len(svc.Ports) == 1 {
		return svc.Target
	}
	host, _, err := net.SplitHostPort(svc.Target)
	if err != nil {
		return svc.Target
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func boxAddr(port int) string {
	return net.JoinHostPort(BoxAddress, strconv.Itoa(port))
}

func parseBox(s string) (net.IPNet, error) {
	return parseCIDR(s + "/32")
}

func parseCIDR(s string) (net.IPNet, error) {
	ip, network, err := net.ParseCIDR(s)
	if err != nil {
		return net.IPNet{}, errors.New("invalid tunnel address")
	}
	return net.IPNet{IP: ip, Mask: network.Mask}, nil
}

func mustCIDR(s string) net.IPNet {
	ip, network, err := net.ParseCIDR(s)
	if err != nil {
		panic("bad cidr " + s)
	}
	return net.IPNet{IP: ip, Mask: network.Mask}
}
