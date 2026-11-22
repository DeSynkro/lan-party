package tunnel

import (
	"context"
	"crypto/ed25519"
	"errors"
)

// ProtocolVersion is the control protocol this build speaks.
const ProtocolVersion = 1

// ALPN keeps other QUIC clients from being mistaken for LAN Party.
const ALPN = "lanparty/1"

// Control message types.
const (
	MsgHello    = "hello"
	MsgWelcome  = "welcome"
	MsgDenied   = "denied"
	MsgServices = "services"
	MsgRevoked  = "revoked"
	MsgBye      = "bye"
)

// ErrDenied means the host read the request and refused it.
var ErrDenied = errors.New("the host denied the join")

// ServiceInfo is the tunnel's copy of a published service. The session layer
// maps it to the group model.
type ServiceInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Proto string `json:"proto"`
	Ports []int  `json:"ports"`
	Hint  string `json:"hint,omitempty"`
}

// ControlMessage is one JSON object on the control stream.
type ControlMessage struct {
	Type            string        `json:"type"`
	Version         int           `json:"version,omitempty"`
	GroupID         string        `json:"groupID,omitempty"`
	MemberID        string        `json:"memberID,omitempty"`
	PeerName        string        `json:"peerName,omitempty"`
	DeviceName      string        `json:"deviceName,omitempty"`
	Token           string        `json:"token,omitempty"`
	HasRelayToken   bool          `json:"hasRelayToken,omitempty"`
	RelayToken      string        `json:"relayToken,omitempty"`
	ForgetAfterDays int           `json:"forgetAfterDays,omitempty"`
	Reason          string        `json:"reason,omitempty"`
	Services        []ServiceInfo `json:"services,omitempty"`
}

// AuthRequest is what the host policy layer gets to inspect before a
// connection is allowed. The public key comes from the TLS client
// certificate, not from the request body.
type AuthRequest struct {
	GroupID       string
	MemberID      string
	PeerName      string
	DeviceName    string
	Token         string
	HasRelayToken bool
	RemoteAddr    string
	Relay         bool
	PublicKey     ed25519.PublicKey
	Fingerprint   string
}

// AuthDecision is the host's answer.
type AuthDecision struct {
	Allow           bool
	MemberID        string
	Services        []ServiceInfo
	RelayToken      string
	ForgetAfterDays int
	Reason          string
}

// Authenticator decides whether a connection may proceed. It may block while
// the host approves a device; the tunnel gives it a bounded context.
type Authenticator interface {
	Authenticate(context.Context, AuthRequest) (AuthDecision, error)
}

// DataHeader identifies which service a data stream belongs to.
type DataHeader struct {
	ServiceID string
	Port      int
}

func clipText(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
