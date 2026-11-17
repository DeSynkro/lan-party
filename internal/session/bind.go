// Package session ties groups, the tunnel and the forwarders together.
package session

import (
	"fmt"
	"net"
	"strconv"
)

const bindAttempts = 10

// BindTCP binds a loopback port, trying the next few ports if the requested
// one is taken. It returns the port it actually got.
func BindTCP(port int) (net.Listener, int, error) {
	for i := 0; i < bindAttempts; i++ {
		p := port + i
		if p > 65535 {
			break
		}
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
		if err == nil {
			return ln, p, nil
		}
	}
	return nil, 0, fmt.Errorf("no free loopback TCP port near %d", port)
}

// BindUDP is the UDP version of BindTCP.
func BindUDP(port int) (*net.UDPConn, int, error) {
	for i := 0; i < bindAttempts; i++ {
		p := port + i
		if p > 65535 {
			break
		}
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: p})
		if err == nil {
			return conn, p, nil
		}
	}
	return nil, 0, fmt.Errorf("no free loopback UDP port near %d", port)
}
