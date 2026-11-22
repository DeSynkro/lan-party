package relay

import "github.com/desynkro/lan-party/internal/password"

// GeneratePassword returns a random relay password.
func GeneratePassword() (string, error) {
	return password.Generate()
}

// HashPassword returns a salted PBKDF2 hash for the relay password.
func HashPassword(p string) (salt, hash string, err error) {
	return password.Hash(p)
}

// VerifyPassword checks a password against the stored salt and hash.
func VerifyPassword(p, salt, hash string) bool {
	return password.Verify(p, salt, hash)
}
