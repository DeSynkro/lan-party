package group

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

// MaxPorts caps how many ports one service can publish, so a preset cannot
// turn into a port scan of the host.
const MaxPorts = 8

// ParseTarget checks a LAN address and returns it in canonical form.
//
// Loopback is allowed, because plenty of game servers bind only 127.0.0.1, but
// the caller is told about it so the interface can warn.
func ParseTarget(s string) (addr string, loopback bool, err error) {
	s = strings.TrimSpace(s)
	host, portText, err := net.SplitHostPort(s)
	if err != nil {
		return "", false, errors.New("use an address like 192.168.1.50:25565")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", false, errors.New("the host part must be an IP address, not a name")
	}
	if ip.IsUnspecified() {
		return "", false, errors.New("a wildcard address is not a target")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", false, errors.New("the port must be between 1 and 65535")
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), ip.IsLoopback(), nil
}

// ParsePorts reads the many ways people write ports: 25565, 2456-2458,
// 25565,25566. It returns them sorted and deduplicated.
func ParsePorts(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("give at least one port")
	}

	seen := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			first, err1 := strconv.Atoi(strings.TrimSpace(lo))
			last, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 != nil || err2 != nil || first < 1 || last > 65535 || first > last {
				return nil, fmt.Errorf("%q is not a port range", part)
			}
			for p := first; p <= last; p++ {
				seen[p] = true
			}
			continue
		}
		port, err := strconv.Atoi(part)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%q is not a port", part)
		}
		seen[port] = true
	}

	if len(seen) == 0 {
		return nil, errors.New("give at least one port")
	}
	if len(seen) > MaxPorts {
		return nil, fmt.Errorf("at most %d ports per service", MaxPorts)
	}

	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports, nil
}

// FormatPorts writes ports the way ParsePorts reads them, collapsing runs.
func FormatPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	sorted := append([]int(nil), ports...)
	sort.Ints(sorted)

	var parts []string
	for i := 0; i < len(sorted); {
		j := i
		for j+1 < len(sorted) && sorted[j+1] == sorted[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, fmt.Sprintf("%d-%d", sorted[i], sorted[j]))
		} else {
			parts = append(parts, strconv.Itoa(sorted[i]))
		}
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// ValidProto reports whether a service protocol is one we can carry.
func ValidProto(s string) bool {
	return s == "tcp" || s == "udp" || s == "both"
}
