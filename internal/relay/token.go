package relay

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// tokenLength is 128 bits. Shorter tokens keep the relay header small enough
// to avoid IP fragmentation on a normal 1500-byte path.
const tokenLength = 16

// NewFriendToken returns a raw token and its copyable text form.
func NewFriendToken() ([]byte, string, error) {
	raw := make([]byte, tokenLength)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate friend token: %w", err)
	}
	return raw, base64.RawURLEncoding.EncodeToString(raw), nil
}

// DecodeFriendToken reads a copyable friend token.
func DecodeFriendToken(text string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.New("that is not a relay friend token")
	}
	if len(raw) != tokenLength {
		return nil, errors.New("that is not a relay friend token")
	}
	return raw, nil
}

// TokenHash is the value the relay stores for a token.
func TokenHash(raw []byte) [32]byte {
	return sha256.Sum256(raw)
}

func newDataToken() ([]byte, error) {
	raw := make([]byte, tokenLength)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate relay data token: %w", err)
	}
	return raw, nil
}

func newSessionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate relay session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
