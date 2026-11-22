// Package password provides the salted password hashing shared by the relay
// and the web host.
package password

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
	iterations = 600000
	keyLength  = 32
	saltLength = 16
)

// Generate returns a random 24-byte password encoded with base64url.
func Generate() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Hash returns a salted PBKDF2-SHA256 hash for password.
func Hash(password string) (salt, hash string, err error) {
	if password == "" {
		return "", "", errors.New("password is empty")
	}
	saltBytes := make([]byte, saltLength)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", fmt.Errorf("generate password salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, saltBytes, iterations, keyLength)
	if err != nil {
		return "", "", fmt.Errorf("hash password: %w", err)
	}
	return base64.StdEncoding.EncodeToString(saltBytes), base64.StdEncoding.EncodeToString(key), nil
}

// Verify reports whether password matches the stored salt and hash.
func Verify(password, saltText, hashText string) bool {
	salt, err := base64.StdEncoding.DecodeString(saltText)
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(hashText)
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
