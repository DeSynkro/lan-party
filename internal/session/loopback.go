package session

import (
	"fmt"
	"net"
	"strconv"
)

// loopbackAttempts is how many aliases and spare ports we try before giving up.
const (
	loopbackAliases = 9
	sparePorts      = 20
)

// listenLoopback finds a free loopback address for a service. It prefers the
// game's own port on 127.0.0.1, then other loopback addresses, then the next
// ports, because a friend may already be running their own server on 25565.
func listenLoopback(port int) (net.Listener, string, error) {
	for _, addr := range candidates(port) {
		ln, err := net.Listen("tcp4", addr)
		if err == nil {
			return ln, ln.Addr().String(), nil
		}
	}
	return nil, "", fmt.Errorf("no free loopback address near port %d", port)
}

func listenLoopbackPacket(port int) (net.PacketConn, string, error) {
	for _, addr := range candidates(port) {
		pc, err := net.ListenPacket("udp4", addr)
		if err == nil {
			return pc, pc.LocalAddr().String(), nil
		}
	}
	return nil, "", fmt.Errorf("no free loopback address near port %d", port)
}

func candidates(port int) []string {
	var out []string

	for alias := 1; alias <= loopbackAliases; alias++ {
		out = append(out, net.JoinHostPort(fmt.Sprintf("127.0.0.%d", alias), strconv.Itoa(port)))
	}
	for extra := 1; extra <= sparePorts; extra++ {
		out = append(out, net.JoinHostPort("127.0.0.1", strconv.Itoa(port+extra)))
	}
	return out
}
