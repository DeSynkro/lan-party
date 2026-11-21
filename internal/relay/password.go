package relay

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
)

const (
	passwordIterations = 600000
	passwordKeyLength  = 32
	passwordSaltLength = 16
)

// GeneratePassword returns a random relay password.
func GeneratePassword() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate relay password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashPassword returns a salted PBKDF2 hash for the relay password.
func HashPassword(password string) (salt, hash string, err error) {
	if password == "" {
		return "", "", errors.New("relay password is empty")
	}
	saltBytes := make([]byte, passwordSaltLength)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", fmt.Errorf("generate password salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, saltBytes, passwordIterations, passwordKeyLength)
	if err != nil {
		return "", "", fmt.Errorf("hash relay password: %w", err)
	}
	return base64.StdEncoding.EncodeToString(saltBytes), base64.StdEncoding.EncodeToString(key), nil
}

// VerifyPassword checks a password against the stored salt and hash.
func VerifyPassword(password, saltText, hashText string) bool {
	salt, err := base64.StdEncoding.DecodeString(saltText)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(hashText)
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordKeyLength)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
