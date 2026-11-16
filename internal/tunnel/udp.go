package tunnel

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// gVisor ships net.Conn adapters for TCP only, so UDP gets a small one here.
// Deadlines are not implemented because the forwarder relies on closing an idle
// socket instead of timing out a single call.

type udpSocket struct {
	ep  tcpip.Endpoint
	wq  *waiter.Queue
	to  *tcpip.FullAddress
	err error
}

func (d *Device) newUDP() (tcpip.Endpoint, *waiter.Queue, error) {
	wq := &waiter.Queue{}
	ep, err := d.stack.NewEndpoint(udp.ProtocolNumber, ipv4.ProtocolNumber, wq)
	if err != nil {
		return nil, nil, fmt.Errorf("udp endpoint: %s", err)
	}
	return ep, wq, nil
}

// dialUDP opens a connected UDP socket on the box address.
func (d *Device) dialUDP(remote tcpip.FullAddress) (net.Conn, error) {
	ep, wq, err := d.newUDP()
	if err != nil {
		return nil, err
	}
	if err := ep.Bind(tcpip.FullAddress{}); err != nil {
		ep.Close()
		return nil, fmt.Errorf("bind udp: %s", err)
	}
	if err := ep.Connect(remote); err != nil {
		ep.Close()
		return nil, fmt.Errorf("connect udp: %s", err)
	}
	return &udpSocket{ep: ep, wq: wq}, nil
}

// listenUDP binds a UDP socket on the box address.
func (d *Device) listenUDP(local tcpip.FullAddress) (net.PacketConn, error) {
	ep, wq, err := d.newUDP()
	if err != nil {
		return nil, err
	}
	if err := ep.Bind(local); err != nil {
		ep.Close()
		return nil, fmt.Errorf("bind udp: %s", err)
	}
	return &udpSocket{ep: ep, wq: wq}, nil
}

func (s *udpSocket) Read(p []byte) (int, error) {
	n, _, err := s.read(p, false)
	return n, err
}

func (s *udpSocket) ReadFrom(p []byte) (int, net.Addr, error) {
	return s.read(p, true)
}

func (s *udpSocket) read(p []byte, wantAddr bool) (int, net.Addr, error) {
	if s.err != nil {
		return 0, nil, s.err
	}

	w := tcpip.SliceWriter(p)
	for {
		res, err := s.ep.Read(&w, tcpip.ReadOptions{NeedRemoteAddr: wantAddr})
		switch err.(type) {
		case nil:
			addr := udpAddr{res.RemoteAddr}
			return res.Count, addr, nil
		case *tcpip.ErrWouldBlock:
			entry, ch := waiter.NewChannelEntry(waiter.ReadableEvents)
			s.wq.EventRegister(&entry)
			<-ch
			s.wq.EventUnregister(&entry)
		case *tcpip.ErrClosedForReceive:
			return 0, nil, io.EOF
		case *tcpip.ErrBadLocalAddress:
			return 0, nil, io.EOF
		default:
			return 0, nil, errors.New(err.String())
		}
	}
}

func (s *udpSocket) Write(p []byte) (int, error) {
	return s.write(p, nil)
}

func (s *udpSocket) WriteTo(p []byte, to net.Addr) (int, error) {
	addr, ok := to.(udpAddr)
	if !ok {
		return 0, errors.New("write to an address this socket did not produce")
	}
	return s.write(p, &addr.full)
}

func (s *udpSocket) write(p []byte, to *tcpip.FullAddress) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.to != nil {
		to = s.to
	}

	r := bytes.NewReader(p)
	for {
		n, err := s.ep.Write(r, tcpip.WriteOptions{To: to})
		switch err.(type) {
		case nil:
			if int(n) == len(p) {
				return int(n), nil
			}
			if n > 0 {
				r = bytes.NewReader(p[n:])
			}
		case *tcpip.ErrWouldBlock:
			entry, ch := waiter.NewChannelEntry(waiter.WritableEvents)
			s.wq.EventRegister(&entry)
			<-ch
			s.wq.EventUnregister(&entry)
		case *tcpip.ErrClosedForSend:
			return 0, io.ErrClosedPipe
		default:
			return 0, errors.New(err.String())
		}
	}
}

func (s *udpSocket) Close() error {
	s.err = io.EOF
	s.ep.Close()
	return nil
}

func (s *udpSocket) LocalAddr() net.Addr {
	addr, err := s.ep.GetLocalAddress()
	if err != nil {
		return udpAddr{}
	}
	return udpAddr{addr}
}

func (s *udpSocket) RemoteAddr() net.Addr {
	addr, err := s.ep.GetRemoteAddress()
	if err != nil {
		return udpAddr{}
	}
	return udpAddr{addr}
}

func (s *udpSocket) SetDeadline(time.Time) error      { return errNoDeadlines }
func (s *udpSocket) SetReadDeadline(time.Time) error  { return errNoDeadlines }
func (s *udpSocket) SetWriteDeadline(time.Time) error { return errNoDeadlines }

var errNoDeadlines = errors.New("tunnel udp sockets do not support deadlines")

type udpAddr struct {
	full tcpip.FullAddress
}

func (a udpAddr) Network() string { return "udp" }
func (a udpAddr) String() string {
	return net.JoinHostPort(a.full.Addr.String(), strconv.Itoa(int(a.full.Port)))
}
