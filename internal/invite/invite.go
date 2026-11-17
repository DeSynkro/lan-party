// Package invite packs everything a friend needs into one pasteable string.
// The token inside is a bearer credential: it is single use, expires, and the
// host stores only its hash.
package invite

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// CurrentVersion is the only invite format this build understands.
const CurrentVersion = 1

const (
	codePrefix   = "lp1_"
	tokenPrefix  = "LP1"
	tokenBytes   = 20
	tokenEncoded = 32
)

// Invite is the decoded payload. The token is the one-time secret; the pin is
// the host's public-key fingerprint. No private key is ever part of it.
type Invite struct {
	Version   int       `json:"v"`
	GroupID   string    `json:"group"`
	GroupName string    `json:"groupName"`
	HostName  string    `json:"hostName"`
	Endpoint  string    `json:"endpoint"`
	HostPin   string    `json:"hostPin"`
	Token     string    `json:"token"`
	PeerName  string    `json:"peerName"`
	Expires   time.Time `json:"expires"`
}

// NewToken returns a formatted one-time token and the hash the host stores.
func NewToken() (token, hash string, err error) {
	var raw [tokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", fmt.Errorf("generate invite token: %w", err)
	}
	body := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:])
	return formatToken(body), hashBody(body), nil
}

// NormalizeToken makes a token pasted in any case or spacing usable again.
func NormalizeToken(token string) string {
	body, err := tokenBody(token)
	if err != nil {
		return strings.TrimSpace(token)
	}
	return formatToken(body)
}

// HashToken hashes the canonical token body, so spacing and case do not
// change the stored value.
func HashToken(token string) string {
	body, err := tokenBody(token)
	if err != nil {
		return ""
	}
	return hashBody(body)
}

// Normalize cleans up an invite code without touching the base64 payload.
func Normalize(code string) string {
	code = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, code)
	if len(code) >= len(codePrefix) && strings.EqualFold(code[:len(codePrefix)], codePrefix) {
		code = codePrefix + code[len(codePrefix):]
	}
	return code
}

// Encode turns an invite into a pasteable string.
func Encode(inv Invite) (string, error) {
	if err := Validate(inv, time.Now()); err != nil {
		return "", err
	}
	data, err := json.Marshal(inv)
	if err != nil {
		return "", fmt.Errorf("encode invite: %w", err)
	}
	return codePrefix + base64.RawURLEncoding.EncodeToString(data), nil
}

// Decode reads a code and refuses anything damaged, incomplete or expired.
func Decode(code string) (Invite, error) {
	code = Normalize(strings.TrimSpace(code))
	if !strings.HasPrefix(code, codePrefix) {
		return Invite{}, errors.New("that does not look like an invite")
	}

	data, err := base64.RawURLEncoding.DecodeString(code[len(codePrefix):])
	if err != nil {
		return Invite{}, errors.New("the invite is damaged")
	}

	var inv Invite
	if err := json.Unmarshal(data, &inv); err != nil {
		return Invite{}, errors.New("the invite is damaged")
	}
	if err := Validate(inv, time.Now()); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

// Validate checks the shape of an invite. A zero now skips the expiry check,
// which is only useful in tests.
func Validate(inv Invite, now time.Time) error {
	if inv.Version != CurrentVersion {
		return fmt.Errorf("invite version %d is not supported", inv.Version)
	}
	if strings.TrimSpace(inv.GroupID) == "" || strings.TrimSpace(inv.GroupName) == "" || strings.TrimSpace(inv.HostName) == "" {
		return errors.New("the invite is missing the group details")
	}
	if strings.TrimSpace(inv.PeerName) == "" {
		return errors.New("the invite is missing your name")
	}
	if err := validateEndpoint(inv.Endpoint); err != nil {
		return err
	}
	if _, err := tokenBody(inv.Token); err != nil {
		return errors.New("the invite has a bad token")
	}
	pin, err := base64.RawURLEncoding.DecodeString(inv.HostPin)
	if err != nil || len(pin) != sha256.Size {
		return errors.New("the invite has a bad host key")
	}
	if inv.Expires.IsZero() {
		return errors.New("the invite has no expiry")
	}
	if !now.IsZero() && !now.Before(inv.Expires) {
		return errors.New("this invite has expired")
	}
	return nil
}

func validateEndpoint(endpoint string) error {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || strings.TrimSpace(host) == "" {
		return errors.New("the invite has a bad endpoint")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("the invite has a bad endpoint")
	}
	return nil
}

func formatToken(body string) string {
	parts := make([]string, 0, len(body)/4)
	for i := 0; i < len(body); i += 4 {
		end := i + 4
		if end > len(body) {
			end = len(body)
		}
		parts = append(parts, body[i:end])
	}
	return tokenPrefix + "-" + strings.Join(parts, "-")
}

func tokenBody(token string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(token))
	s = strings.Map(func(r rune) rune {
		switch r {
		case '-', ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, s)
	s = strings.TrimPrefix(s, tokenPrefix)
	if len(s) != tokenEncoded {
		return "", errors.New("that does not look like an invite token")
	}
	if _, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s); err != nil {
		return "", errors.New("that does not look like an invite token")
	}
	return s, nil
}

func hashBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
