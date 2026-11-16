// Package group holds the things the host owns: the group, its members, the
// services it publishes and the invites it has issued.
package group

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Group is one host's LAN party.
type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	HostName  string    `json:"hostName"`
	CreatedAt time.Time `json:"createdAt"`
	Members   []Member  `json:"members"`
	Services  []Service `json:"services"`
}

// Member is someone in the group. Only the host publishes services.
type Member struct {
	Name     string    `json:"name"`
	IsHost   bool      `json:"isHost"`
	LastSeen time.Time `json:"lastSeen,omitempty"`
}

// Service is one published thing. Target is where it lives on the host's LAN.
type Service struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Proto  string `json:"proto"`
	Ports  []int  `json:"ports"`
	Target string `json:"target"`
}

// Create makes an empty group with its host already in the roster.
func Create(name, hostName string) Group {
	return Group{
		ID:        NewID(),
		Name:      name,
		HostName:  hostName,
		CreatedAt: time.Now().UTC(),
		Members:   []Member{{Name: hostName, IsHost: true}},
	}
}

// NewID returns a random identifier. crypto/rand does not fail on the platforms
// we support, so there is no error to pass around.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
