// Package invite packs everything a friend needs into one string they can paste
// or scan. The code is a bearer credential: whoever redeems it first becomes
// that peer, so it is single use and expires.
package invite

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Prefix marks a string as one of ours and carries a format version.
const Prefix = "lp1_"

// Service is the snapshot of one published service. The full list is refreshed
// over the control channel later; this is enough to open the local listeners.
type Service struct {
	Name  string `json:"name"`
	Proto string `json:"proto"`
	Ports []int  `json:"ports"`
	Hint  string `json:"hint,omitempty"`
}

// Invite is what the host hands out.
type Invite struct {
	Version   int       `json:"v"`
	GroupID   string    `json:"group"`
	GroupName string    `json:"groupName"`
	HostName  string    `json:"hostName"`
	Endpoint  string    `json:"endpoint"`
	HostKey   string    `json:"hostKey"`
	PeerName  string    `json:"peerName"`
	PeerKey   string    `json:"peerKey"`
	PeerBox   string    `json:"peerBox"`
	Expires   time.Time `json:"expires"`
	Services  []Service `json:"services"`
}

// Encode turns an invite into a pasteable string.
func Encode(inv Invite) (string, error) {
	if err := inv.validate(time.Time{}); err != nil {
		return "", err
	}
	data, err := json.Marshal(inv)
	if err != nil {
		return "", fmt.Errorf("encode invite: %w", err)
	}
	return Prefix + base64.RawURLEncoding.EncodeToString(data), nil
}

// Decode reads a code and refuses anything expired or malformed.
func Decode(code string) (Invite, error) {
	code = strings.TrimSpace(code)
	if !strings.HasPrefix(code, Prefix) {
		return Invite{}, errors.New("that does not look like an invite")
	}

	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, Prefix))
	if err != nil {
		return Invite{}, errors.New("invite is damaged")
	}

	var inv Invite
	if err := json.Unmarshal(data, &inv); err != nil {
		return Invite{}, errors.New("invite is damaged")
	}
	if err := inv.validate(time.Now()); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

func (inv Invite) validate(now time.Time) error {
	if inv.Version != 1 {
		return fmt.Errorf("invite version %d is not supported", inv.Version)
	}
	if inv.GroupID == "" || inv.GroupName == "" || inv.HostName == "" {
		return errors.New("invite is missing the group details")
	}
	if inv.PeerName == "" || inv.PeerKey == "" || inv.PeerBox == "" {
		return errors.New("invite is missing the peer details")
	}
	if _, err := wgtypes.ParseKey(inv.HostKey); err != nil {
		return errors.New("invite has a bad host key")
	}
	if _, err := wgtypes.ParseKey(inv.PeerKey); err != nil {
		return errors.New("invite has a bad peer key")
	}
	if !validEndpoint(inv.Endpoint) {
		return errors.New("invite has a bad endpoint")
	}
	if !validIPv4(inv.PeerBox) {
		return errors.New("invite has a bad tunnel address")
	}
	for _, s := range inv.Services {
		if s.Name == "" || len(s.Ports) == 0 {
			return errors.New("invite has an incomplete service")
		}
		if s.Proto != "tcp" && s.Proto != "udp" && s.Proto != "both" {
			return errors.New("invite has a service with an unknown protocol")
		}
	}
	if !now.IsZero() && !inv.Expires.IsZero() && now.After(inv.Expires) {
		return errors.New("this invite has expired")
	}
	return nil
}

// Keys returns the parsed host public key and peer private key.
func (inv Invite) Keys() (host wgtypes.Key, peer wgtypes.Key, err error) {
	host, err = wgtypes.ParseKey(inv.HostKey)
	if err != nil {
		return wgtypes.Key{}, wgtypes.Key{}, err
	}
	peer, err = wgtypes.ParseKey(inv.PeerKey)
	if err != nil {
		return wgtypes.Key{}, wgtypes.Key{}, err
	}
	return host, peer, nil
}

func validEndpoint(s string) bool {
	host, portText, err := net.SplitHostPort(s)
	if err != nil || host == "" {
		return false
	}
	port, err := strconv.Atoi(portText)
	return err == nil && port >= 1 && port <= 65535
}

func validIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}
