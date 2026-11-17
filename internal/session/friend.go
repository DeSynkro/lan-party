package session

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/desynkro/lan-party/internal/forward"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/tunnel"
)

var errResumeDetails = errors.New("the saved group is missing its host details")

// Bound is one local address the friend can point a game at.
type Bound struct {
	ServiceID     string
	Service       string
	Proto         string
	PublishedPort int
	Port          int
	Address       string
	Hint          string
	Error         string
}

// JoinConfig is what a friend needs to open a group.
type JoinConfig struct {
	Identity   secure.Identity
	Invite     invite.Invite
	DeviceName string
	MemberID   string
	Token      string
}

// Friend is a running session on the friend's side.
type Friend struct {
	conn     *tunnel.Conn
	MemberID string
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	bounds   []Bound
	tcp      []net.Listener
	udp      []*net.UDPConn
	udpChans map[int]chan []byte
	services []tunnel.ServiceInfo
	note     string
}

// Join opens the tunnel, waits for the host, and binds the published services.
func Join(ctx context.Context, cfg JoinConfig) (*Friend, error) {
	if cfg.MemberID == "" {
		if err := invite.Validate(cfg.Invite, time.Now()); err != nil {
			return nil, err
		}
	} else if cfg.Invite.Endpoint == "" || cfg.Invite.HostPin == "" || cfg.Invite.GroupID == "" {
		return nil, errResumeDetails
	}

	conn, err := tunnel.Dial(ctx, tunnel.ClientConfig{
		Identity:   cfg.Identity,
		Endpoint:   cfg.Invite.Endpoint,
		HostPin:    cfg.Invite.HostPin,
		GroupID:    cfg.Invite.GroupID,
		MemberID:   cfg.MemberID,
		PeerName:   cfg.Invite.PeerName,
		DeviceName: cfg.DeviceName,
		Token:      cfg.Token,
	})
	if err != nil {
		return nil, err
	}

	f := &Friend{
		conn:     conn,
		MemberID: conn.MemberID,
		udpChans: map[int]chan []byte{},
	}
	f.ctx, f.cancel = context.WithCancel(context.Background())
	f.bind(conn.Services)
	go f.datagrams()
	go f.control()
	return f, nil
}

// Bound returns the local addresses currently open for games.
func (f *Friend) Bound() []Bound {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Bound(nil), f.bounds...)
}

// Services is the last service list the host sent.
func (f *Friend) Services() []tunnel.ServiceInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]tunnel.ServiceInfo, len(f.services))
	for i, svc := range f.services {
		out[i] = svc
		out[i].Ports = append([]int(nil), svc.Ports...)
	}
	return out
}

// Presence reports whether the host connection is still live.
func (f *Friend) Presence() Presence {
	live := f.ctx.Err() == nil && f.conn.Context().Err() == nil
	return Presence{
		Live:     live,
		RTT:      f.conn.RTT(),
		LastSeen: time.Now(),
	}
}

// Note explains why a session ended, when there is something useful to say.
func (f *Friend) Note() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.note
}

// Close ends the session and closes every local listener.
func (f *Friend) Close() error {
	f.shutdown("")

	f.mu.Lock()
	for _, ln := range f.tcp {
		_ = ln.Close()
	}
	for _, conn := range f.udp {
		_ = conn.Close()
	}
	f.mu.Unlock()
	return nil
}

func (f *Friend) bind(services []tunnel.ServiceInfo) {
	if len(services) > group.MaxServices {
		services = services[:group.MaxServices]
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, ln := range f.tcp {
		_ = ln.Close()
	}
	for _, conn := range f.udp {
		_ = conn.Close()
	}
	f.tcp = nil
	f.udp = nil
	f.bounds = nil
	f.udpChans = map[int]chan []byte{}
	f.services = nil
	for _, svc := range services {
		copied := svc
		copied.Ports = append([]int(nil), svc.Ports...)
		if len(copied.Ports) > group.MaxPorts {
			copied.Ports = copied.Ports[:group.MaxPorts]
		}
		f.services = append(f.services, copied)
	}

	for _, svc := range services {
		ports := svc.Ports
		if len(ports) > group.MaxPorts {
			ports = ports[:group.MaxPorts]
		}
		for _, port := range ports {
			if svc.Proto == "tcp" || svc.Proto == "both" {
				b := Bound{
					ServiceID:     svc.ID,
					Service:       svc.Name,
					Proto:         "tcp",
					PublishedPort: port,
					Port:          port,
					Hint:          svc.Hint,
				}
				ln, actual, err := BindTCP(port)
				if err != nil {
					b.Error = err.Error()
				} else {
					b.Port = actual
					b.Address = ln.Addr().String()
					f.tcp = append(f.tcp, ln)
					go forward.ServeTCP(f.ctx, ln, func(ctx context.Context) (io.ReadWriteCloser, error) {
						return f.conn.OpenData(ctx, tunnel.DataHeader{ServiceID: svc.ID, Port: port})
					})
				}
				f.bounds = append(f.bounds, b)
			}

			if svc.Proto == "udp" || svc.Proto == "both" {
				b := Bound{
					ServiceID:     svc.ID,
					Service:       svc.Name,
					Proto:         "udp",
					PublishedPort: port,
					Port:          port,
					Hint:          svc.Hint,
				}
				conn, actual, err := BindUDP(port)
				if err != nil {
					b.Error = err.Error()
				} else {
					b.Port = actual
					b.Address = conn.LocalAddr().String()
					recv := make(chan []byte, 32)
					f.udpChans[port] = recv
					f.udp = append(f.udp, conn)
					go forward.ServeFriendUDP(f.ctx, conn, port, f.conn.SendDatagram, recv)
				}
				f.bounds = append(f.bounds, b)
			}
		}
	}
}

func (f *Friend) control() {
	for {
		select {
		case <-f.ctx.Done():
			return
		case msg, ok := <-f.conn.Messages():
			if !ok {
				f.shutdown("the host closed the session")
				return
			}
			switch msg.Type {
			case tunnel.MsgServices:
				f.bind(msg.Services)
			case tunnel.MsgRevoked:
				f.shutdown("the host removed this device")
				return
			case tunnel.MsgBye:
				f.shutdown("the host closed the session")
				return
			}
		}
	}
}

func (f *Friend) datagrams() {
	for {
		packet, err := f.conn.ReceiveDatagram(f.ctx)
		if err != nil {
			f.shutdown("the connection to the host ended")
			return
		}
		port, _, _, err := forward.DecodeDatagram(packet)
		if err != nil {
			continue
		}
		f.mu.Lock()
		ch := f.udpChans[port]
		f.mu.Unlock()
		if ch == nil {
			continue
		}
		select {
		case ch <- packet:
		default:
		}
	}
}

func (f *Friend) shutdown(note string) {
	f.mu.Lock()
	if f.note == "" {
		f.note = note
	}
	f.mu.Unlock()

	f.cancel()
	_ = f.conn.Close()
}
