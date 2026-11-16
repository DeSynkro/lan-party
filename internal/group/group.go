// Package group holds the things the host owns: the group, its members, the
// services it publishes and the invites it has issued.
package group

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Group is one host's LAN party.
type Group struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	HostName   string    `json:"hostName"`
	PrivateKey string    `json:"privateKey"`
	ListenPort int       `json:"listenPort"`
	Endpoint   string    `json:"endpoint"`
	CreatedAt  time.Time `json:"createdAt"`
	Members    []Member  `json:"members"`
	Services   []Service `json:"services"`
	Invites    []Code    `json:"invites"`
}

// Member is someone in the group. Only the host publishes services.
type Member struct {
	Name       string    `json:"name"`
	IsHost     bool      `json:"isHost"`
	PublicKey  string    `json:"publicKey,omitempty"`
	BoxAddress string    `json:"boxAddress,omitempty"`
	LastSeen   time.Time `json:"lastSeen,omitempty"`
}

// Code is an invite the host has issued, kept so it can be shown again.
type Code struct {
	Value    string    `json:"value"`
	PeerName string    `json:"peerName"`
	Expires  time.Time `json:"expires"`
	Redeemed bool      `json:"redeemed"`
}

// DefaultListenPort is WireGuard's usual port.
const DefaultListenPort = 51820

// Create makes an empty group with its host already in the roster.
func Create(name, hostName string) Group {
	key, err := wgtypes.GeneratePrivateKey()
	private := ""
	if err == nil {
		private = key.String()
	}

	return Group{
		ID:         NewID(),
		Name:       name,
		HostName:   hostName,
		PrivateKey: private,
		ListenPort: DefaultListenPort,
		CreatedAt:  time.Now().UTC(),
		Members:    []Member{{Name: hostName, IsHost: true}},
	}
}

// NextBoxAddress returns the next free tunnel address, starting at 100.64.0.2
// because the host owns .1.
func (g Group) NextBoxAddress() string {
	used := map[string]bool{}
	for _, m := range g.Members {
		used[m.BoxAddress] = true
	}
	for n := 2; n < 255; n++ {
		addr := fmt.Sprintf("100.64.0.%d", n)
		if !used[addr] {
			return addr
		}
	}
	return ""
}

// Service is one published thing. Target is where it lives on the host's LAN.
type Service struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Proto  string `json:"proto"`
	Ports  []int  `json:"ports"`
	Target string `json:"target"`
}

// NewID returns a random identifier. crypto/rand does not fail on the platforms
// we support, so there is no error to pass around.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
