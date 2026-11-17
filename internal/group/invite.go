package group

import "time"

// Invite is the host's record of a one-time code. Only the hash is stored.
type Invite struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	TokenHash  string    `json:"tokenHash"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	RedeemedAt time.Time `json:"redeemedAt,omitempty"`
}

// Open reports whether the invite can still be redeemed at now.
func (i Invite) Open(now time.Time) bool {
	if i.TokenHash == "" {
		return false
	}
	if !i.RedeemedAt.IsZero() {
		return false
	}
	if !i.CreatedAt.IsZero() && now.Before(i.CreatedAt) {
		return false
	}
	if i.ExpiresAt.IsZero() || !now.Before(i.ExpiresAt) {
		return false
	}
	return true
}
