// Package tunnel runs WireGuard and a TCP/IP stack inside the process. Nothing
// here creates a kernel interface, opens a listening port on the LAN, or needs
// elevated privileges. The only real resource is one UDP socket.
package tunnel

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// DefaultMTU leaves room for a tunnel inside a system VPN, which is the common
// case for people who leave Proton on.
const DefaultMTU = 1280

// Peer is one friend. AllowedIPs is what that peer is allowed to claim as its
// source address, which is how source spoofing is stopped.
type Peer struct {
	PublicKey    wgtypes.Key
	PresharedKey *wgtypes.Key
	Endpoint     string
	AllowedIPs   []net.IPNet
}

// Config is everything needed to bring a device up.
type Config struct {
	PrivateKey wgtypes.Key
	ListenPort uint16
	Addresses  []net.IPNet
	MTU        int
	Peers      []Peer
}

// Device is a running tunnel.
type Device struct {
	wg    *device.Device
	stack *stack.Stack

	mu     sync.Mutex
	closed bool
}

// Start brings a device up.
func Start(cfg Config) (*Device, error) {
	if cfg.PrivateKey == (wgtypes.Key{}) {
		return nil, errors.New("missing private key")
	}
	if len(cfg.Addresses) == 0 {
		return nil, errors.New("missing address")
	}
	if cfg.MTU == 0 {
		cfg.MTU = DefaultMTU
	}
	if cfg.MTU < 576 || cfg.MTU > 65535 {
		return nil, fmt.Errorf("mtu %d is out of range", cfg.MTU)
	}
	for _, p := range cfg.Peers {
		if p.PublicKey == (wgtypes.Key{}) {
			return nil, errors.New("peer has no public key")
		}
		if len(p.AllowedIPs) == 0 {
			return nil, errors.New("peer has no allowed addresses")
		}
	}

	ns, link, err := newStack(cfg.MTU, cfg.Addresses)
	if err != nil {
		return nil, err
	}

	tunDevice := newBoxTUN(link, uint32(cfg.MTU))
	wg := device.NewDevice(tunDevice, conn.NewDefaultBind(),
		device.NewLogger(device.LogLevelError, "lan-party: "))

	if err := wg.IpcSet(buildUAPI(cfg)); err != nil {
		wg.Close()
		ns.Close()
		return nil, fmt.Errorf("configure wireguard: %w", err)
	}
	if err := wg.Up(); err != nil {
		wg.Close()
		ns.Close()
		return nil, fmt.Errorf("start wireguard: %w", err)
	}

	return &Device{wg: wg, stack: ns}, nil
}

// Close tears the device down.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	d.wg.Close()
	d.stack.Close()
	d.stack.Wait()
	return nil
}

// AddPeer lets someone in.
func (d *Device) AddPeer(p Peer) error {
	if p.PublicKey == (wgtypes.Key{}) {
		return errors.New("peer has no public key")
	}
	if len(p.AllowedIPs) == 0 {
		return errors.New("peer has no allowed addresses")
	}
	return d.wg.IpcSet(buildPeer(p))
}

// RemovePeer shuts someone out. WireGuard forgets the peer and its keys.
func (d *Device) RemovePeer(key wgtypes.Key) error {
	var b strings.Builder
	fmt.Fprintf(&b, "public_key=%s\n", hex.EncodeToString(key[:]))
	b.WriteString("remove=true\n")
	return d.wg.IpcSet(b.String())
}

// DialContext opens a connection through the tunnel.
func (d *Device) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	addr, err := parseAddress(address)
	if err != nil {
		return nil, err
	}

	switch network {
	case "tcp":
		return gonet.DialContextTCP(ctx, d.stack, addr, header.IPv4ProtocolNumber)
	case "udp":
		return d.dialUDP(addr)
	default:
		return nil, fmt.Errorf("network %q is not supported", network)
	}
}

// Listen accepts connections arriving through the tunnel.
func (d *Device) Listen(address string) (net.Listener, error) {
	addr, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	return gonet.ListenTCP(d.stack, addr, header.IPv4ProtocolNumber)
}

// ListenPacket accepts datagrams arriving through the tunnel.
func (d *Device) ListenPacket(address string) (net.PacketConn, error) {
	addr, err := parseAddress(address)
	if err != nil {
		return nil, err
	}
	return d.listenUDP(addr)
}

func parseAddress(address string) (tcpip.FullAddress, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return tcpip.FullAddress{}, fmt.Errorf("invalid address %q", address)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return tcpip.FullAddress{}, errors.New("address must be an IP literal")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return tcpip.FullAddress{}, fmt.Errorf("invalid port in %q", address)
	}
	v4 := ip.To4()
	if v4 == nil {
		return tcpip.FullAddress{}, errors.New("only IPv4 is supported for now")
	}
	return tcpip.FullAddress{
		NIC:  nicID,
		Addr: tcpip.AddrFromSlice(v4),
		Port: uint16(port),
	}, nil
}

func buildUAPI(cfg Config) string {
	var b strings.Builder

	fmt.Fprintf(&b, "private_key=%s\n", hex.EncodeToString(cfg.PrivateKey[:]))
	fmt.Fprintf(&b, "listen_port=%d\n", cfg.ListenPort)
	b.WriteString("replace_peers=true\n")

	for _, p := range cfg.Peers {
		b.WriteString(buildPeer(p))
	}
	return b.String()
}

func buildPeer(p Peer) string {
	var b strings.Builder

	fmt.Fprintf(&b, "public_key=%s\n", hex.EncodeToString(p.PublicKey[:]))
	if p.PresharedKey != nil {
		fmt.Fprintf(&b, "preshared_key=%s\n", hex.EncodeToString((*p.PresharedKey)[:]))
	}
	if p.Endpoint != "" {
		fmt.Fprintf(&b, "endpoint=%s\n", p.Endpoint)
		// Keepalive is what holds a NAT mapping open, so it only makes sense
		// for the side that knows where the other end is.
		b.WriteString("persistent_keepalive_interval=25\n")
	}
	for _, allowed := range p.AllowedIPs {
		fmt.Fprintf(&b, "allowed_ip=%s\n", allowed.String())
	}

	return b.String()
}
