package gtkui

import (
	"fmt"

	"github.com/desynkro/lan-party/internal/session"
)

type friendAddressView struct {
	address   string
	protocols map[string]bool
}

type friendServiceView struct {
	id        string
	name      string
	hint      string
	protocols map[string]bool
	addresses []*friendAddressView
	notes     []string
	errors    []string
}

func buildFriendServiceViews(bounds []session.Bound) []*friendServiceView {
	var views []*friendServiceView
	byService := map[string]*friendServiceView{}
	for _, b := range bounds {
		key := b.ServiceID
		if key == "" {
			key = b.Service
		}
		view := byService[key]
		if view == nil {
			view = &friendServiceView{id: key, name: b.Service, protocols: map[string]bool{}}
			byService[key] = view
			views = append(views, view)
		}
		if b.Service != "" {
			view.name = b.Service
		}
		if b.Hint != "" {
			view.hint = b.Hint
		}
		if b.Proto != "" {
			view.protocols[b.Proto] = true
		}
		if b.Error != "" {
			view.errors = appendUnique(view.errors, b.Error)
			continue
		}
		if b.Address != "" {
			var address *friendAddressView
			for _, existing := range view.addresses {
				if existing.address == b.Address {
					address = existing
					break
				}
			}
			if address == nil {
				address = &friendAddressView{address: b.Address, protocols: map[string]bool{}}
				view.addresses = append(view.addresses, address)
			}
			if b.Proto != "" {
				address.protocols[b.Proto] = true
			}
		}
		if b.Port != b.PublishedPort {
			view.notes = appendUnique(view.notes, fmt.Sprintf("Port %d was busy; this game uses %d.", b.PublishedPort, b.Port))
		}
	}
	return views
}

func protocolSetName(protocols map[string]bool) string {
	tcp, udp := protocols["tcp"], protocols["udp"]
	switch {
	case tcp && udp:
		return "TCP + UDP"
	case tcp:
		return "TCP"
	case udp:
		return "UDP"
	default:
		return ""
	}
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
