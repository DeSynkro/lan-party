package gtkui

import (
	"fmt"
	"strings"

	"github.com/desynkro/lan-party/internal/group"
)

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
