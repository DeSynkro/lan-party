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

// Service is one thing the host publishes. Target is an IP literal, optionally
// with the port the host actually uses. Ports are the ports friends see.
type Service struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Proto  string `json:"proto"`
	Target string `json:"target"`
	Ports  []int  `json:"ports"`
	Hint   string `json:"hint,omitempty"`
}

// ValidProto reports whether a service protocol is one we can carry.
func ValidProto(s string) bool {
	return s == "tcp" || s == "udp" || s == "both"
}

// ParsePorts reads the ways people write ports: 25565, 2456-2458,
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

// ParseTarget checks a LAN address. A target may be an IP with an optional
// port. A name is not accepted, because the host must dial exactly what it
// configured.
func ParseTarget(s string) (addr string, loopback bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false, errors.New("give the LAN address of the server")
	}

	host := s
	portText := ""
	hasPort := false
	if h, p, splitErr := net.SplitHostPort(s); splitErr == nil {
		host, portText, hasPort = h, p, true
	} else if strings.Contains(s, ":") && net.ParseIP(s) == nil {
		return "", false, errors.New("use an IP address like 192.168.1.50 or 192.168.1.50:25565")
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return "", false, errors.New("the host part must be an IP address, not a name")
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.Equal(net.IPv4bcast) {
		return "", false, errors.New("that address cannot be a target")
	}

	if hasPort {
		port, convErr := strconv.Atoi(portText)
		if convErr != nil || port < 1 || port > 65535 {
			return "", false, errors.New("the port must be between 1 and 65535")
		}
		return net.JoinHostPort(ip.String(), strconv.Itoa(port)), ip.IsLoopback(), nil
	}
	return ip.String(), ip.IsLoopback(), nil
}

// ValidateService checks everything the host can edit.
func ValidateService(s Service) error {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return errors.New("a service needs a name")
	}
	if len(name) > 64 {
		return errors.New("the service name is too long")
	}
	if !ValidProto(s.Proto) {
		return errors.New("choose TCP, UDP or both")
	}
	if len(s.Ports) == 0 {
		return errors.New("give at least one port")
	}
	if len(s.Ports) > MaxPorts {
		return fmt.Errorf("at most %d ports per service", MaxPorts)
	}

	seen := map[int]bool{}
	for _, port := range s.Ports {
		if port < 1 || port > 65535 {
			return errors.New("ports must be between 1 and 65535")
		}
		if seen[port] {
			return fmt.Errorf("port %d is listed twice", port)
		}
		seen[port] = true
	}

	_, _, hasPort, err := splitTarget(s.Target)
	if err != nil {
		return err
	}
	if hasPort && len(s.Ports) > 1 {
		return errors.New("a target with a port can only publish one port; use a bare IP for a range")
	}
	if len(s.Hint) > 120 {
		return errors.New("the hint is too long")
	}
	return nil
}

// TargetFor returns the LAN address for one published port.
func (s Service) TargetFor(port int) (string, error) {
	if err := ValidateService(s); err != nil {
		return "", err
	}
	host, targetPort, hasPort, err := splitTarget(s.Target)
	if err != nil {
		return "", err
	}
	if hasPort {
		return net.JoinHostPort(host, targetPort), nil
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// ServiceWarnings are things the host should know but that are not fatal.
func ServiceWarnings(s Service) []string {
	var warnings []string
	_, loopback, err := ParseTarget(s.Target)
	if err == nil && loopback {
		warnings = append(warnings, "the target is on this PC; only games on this machine can use it")
	}
	admin := map[int]bool{22: true, 3389: true, 445: true, 5900: true}
	for _, port := range s.Ports {
		if admin[port] {
			warnings = append(warnings, fmt.Sprintf("port %d is usually an administrative service", port))
		}
	}
	return warnings
}

func splitTarget(target string) (host, port string, hasPort bool, err error) {
	parsed, _, err := ParseTarget(target)
	if err != nil {
		return "", "", false, err
	}
	if h, p, splitErr := net.SplitHostPort(parsed); splitErr == nil {
		return h, p, true, nil
	}
	return parsed, "", false, nil
}

// ParseTargetIP checks a bare game server IP. A port in this field is rejected
// so the form can keep the address and port separate.
func ParseTargetIP(s string) (string, error) {
	parsed, _, err := ParseTarget(s)
	if err != nil {
		return "", err
	}
	if _, _, splitErr := net.SplitHostPort(parsed); splitErr == nil {
		return "", errors.New("use a bare IP address, without a port")
	}
	return parsed, nil
}

// ComposeServiceTarget builds a service target from the split form fields.
// A blank friend port means friends use the same port(s) as the game server.
func ComposeServiceTarget(ip string, gamePorts, friendPorts []int) (string, []int, error) {
	host, err := ParseTargetIP(ip)
	if err != nil {
		return "", nil, err
	}
	if len(gamePorts) == 0 {
		return "", nil, errors.New("give the game server port(s)")
	}

	if len(friendPorts) > 0 {
		if len(friendPorts) != 1 {
			return "", nil, errors.New("the friend port must be a single port")
		}
		if len(gamePorts) != 1 {
			return "", nil, errors.New("a game port range needs the same friend range; leave the friend port blank")
		}
		target := net.JoinHostPort(host, strconv.Itoa(gamePorts[0]))
		ports := []int{friendPorts[0]}
		if err := ValidateService(Service{Name: "service", Proto: "tcp", Target: target, Ports: ports}); err != nil {
			return "", nil, err
		}
		return target, ports, nil
	}

	ports := append([]int(nil), gamePorts...)
	if err := ValidateService(Service{Name: "service", Proto: "tcp", Target: host, Ports: ports}); err != nil {
		return "", nil, err
	}
	return host, ports, nil
}

// GamePorts returns the port(s) on the game server. A remapped target has one
// port; otherwise the published ports are the game ports.
func (s Service) GamePorts() ([]int, error) {
	_, targetPort, hasPort, err := splitTarget(s.Target)
	if err != nil {
		return nil, err
	}
	if hasPort {
		port, convErr := strconv.Atoi(targetPort)
		if convErr != nil {
			return nil, convErr
		}
		return []int{port}, nil
	}
	return append([]int(nil), s.Ports...), nil
}

// GameAddress returns the game server address for display, with its port(s).
func (s Service) GameAddress() (string, error) {
	host, _, hasPort, err := splitTarget(s.Target)
	if err != nil {
		return "", err
	}
	ports, err := s.GamePorts()
	if err != nil {
		return "", err
	}
	if len(ports) == 0 {
		return "", errors.New("the service has no ports")
	}
	if hasPort {
		return net.JoinHostPort(host, strconv.Itoa(ports[0])), nil
	}
	return net.JoinHostPort(host, FormatPorts(ports)), nil
}
