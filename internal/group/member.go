package group

import "time"

// MemberState says whether a device has been approved by the host.
type MemberState string

const (
	MemberPending  MemberState = "pending"
	MemberApproved MemberState = "approved"
)

// Member is a friend's device. The public key is the identity; the name and
// address are there so the host knows who is who.
type Member struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	DeviceName  string      `json:"deviceName,omitempty"`
	PublicKey   string      `json:"publicKey,omitempty"`
	Fingerprint string      `json:"fingerprint,omitempty"`
	LastAddress string      `json:"lastAddress,omitempty"`
	FirstSeen   time.Time   `json:"firstSeen,omitempty"`
	LastSeen    time.Time   `json:"lastSeen,omitempty"`
	State       MemberState `json:"state"`
}

// IsApproved reports whether this member may open the tunnel.
func (m Member) IsApproved() bool { return m.State == MemberApproved }
