package gtkui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/desynkro/lan-party/internal/group"
)

func serviceFormValues(s group.Service) (ip, gamePorts, friendPort, hint string, err error) {
	host, err := s.GameHost()
	if err != nil {
		return "", "", "", "", err
	}
	ports, err := s.GamePorts()
	if err != nil {
		return "", "", "", "", err
	}
	if len(ports) == 1 && len(s.Ports) == 1 && s.Ports[0] != ports[0] {
		friendPort = strconv.Itoa(s.Ports[0])
	}
	return host, group.FormatPorts(ports), friendPort, s.Hint, nil
}

const serviceTargetHelp = "Use 127.0.0.1 for a server or container on this PC.\nFor another PC, use that PC's LAN IP."

func serviceWarningText(ip, gamePortsText, friendPortsText, proto string) string {
	gamePorts, err := group.ParsePorts(gamePortsText)
	if err != nil {
		return ""
	}
	var friendPorts []int
	if strings.TrimSpace(friendPortsText) != "" {
		friendPorts, err = group.ParsePorts(friendPortsText)
		if err != nil {
			return ""
		}
	}
	target, ports, err := group.ComposeServiceTarget(ip, gamePorts, friendPorts)
	if err != nil {
		return ""
	}
	warnings := group.ServiceWarnings(group.Service{
		Name:   "service",
		Proto:  proto,
		Target: target,
		Ports:  ports,
	})
	return strings.Join(warnings, "\n")
}

func serviceRoutePreview(ip, gamePortsText, friendPortsText string) string {
	gamePorts, err := group.ParsePorts(gamePortsText)
	if err != nil {
		return "Enter the game server IP and port(s)."
	}

	var friendPorts []int
	if strings.TrimSpace(friendPortsText) != "" {
		friendPorts, err = group.ParsePorts(friendPortsText)
		if err != nil {
			return "Enter one friend port, or leave it blank."
		}
	}

	if len(friendPorts) > 1 {
		return "Use one friend port, or leave it blank."
	}
	if len(gamePorts) > 1 && len(friendPorts) > 0 {
		return "Leave the friend port blank to use the game port range."
	}

	target, ports, err := group.ComposeServiceTarget(ip, gamePorts, friendPorts)
	if err != nil {
		return "Check the game address and port(s)."
	}
	s := group.Service{Name: "service", Proto: "tcp", Target: target, Ports: ports}
	game, err := s.GameAddress()
	if err != nil {
		return "Check the game address and port(s)."
	}
	return fmt.Sprintf("Friends open 127.0.0.1:%s → game server %s", group.FormatPorts(ports), game)
}
