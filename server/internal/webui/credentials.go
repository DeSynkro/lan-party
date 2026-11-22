package webui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/desynkro/lan-party/internal/password"
)

// credentialsVersion is the on-disk format version.
const credentialsVersion = 1

// minPasswordLength is the smallest accepted admin password.
const minPasswordLength = 12

// Credentials is the stored admin password hash.
type Credentials struct {
	Version   int       `json:"version"`
	Salt      string    `json:"salt"`
	Hash      string    `json:"hash"`
	ChangedAt time.Time `json:"changedAt"`
}

// Verify reports whether plain matches the stored hash.
func (c Credentials) Verify(plain string) bool {
	return password.Verify(plain, c.Salt, c.Hash)
}

// LoadCredentials reads web.json. A missing file returns the os error.
func LoadCredentials(path string) (Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return Credentials{}, fmt.Errorf("parse credentials: %w", err)
	}
	if c.Version != credentialsVersion || c.Salt == "" || c.Hash == "" {
		return Credentials{}, errors.New("credentials file is not valid")
	}
	return c, nil
}

// SaveCredentials writes web.json atomically with owner-only permissions.
func SaveCredentials(path string, c Credentials) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("secure credentials: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("replace credentials: %w", err)
	}
	return nil
}

// EnsureCredentials loads web.json, or creates it from supplied or a generated
// password. The generated password is returned once so the caller can print it.
func EnsureCredentials(path string, supplied string) (Credentials, string, error) {
	c, err := LoadCredentials(path)
	if err == nil {
		return c, "", nil
	}
	if !os.IsNotExist(err) {
		return Credentials{}, "", err
	}

	var generated string
	if supplied == "" {
		generated, err = password.Generate()
		if err != nil {
			return Credentials{}, "", err
		}
	} else {
		if err := checkPasswordLength(supplied); err != nil {
			return Credentials{}, "", err
		}
	}
	secret := supplied
	if secret == "" {
		secret = generated
	}
	c, err = newCredentials(secret)
	if err != nil {
		return Credentials{}, "", err
	}
	if err := SaveCredentials(path, c); err != nil {
		return Credentials{}, "", err
	}
	return c, generated, nil
}

// SetPassword replaces the stored password.
func SetPassword(path, plain string) error {
	if err := checkPasswordLength(plain); err != nil {
		return err
	}
	c, err := newCredentials(plain)
	if err != nil {
		return err
	}
	return SaveCredentials(path, c)
}

func newCredentials(plain string) (Credentials, error) {
	salt, hash, err := password.Hash(plain)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		Version:   credentialsVersion,
		Salt:      salt,
		Hash:      hash,
		ChangedAt: time.Now().UTC(),
	}, nil
}

func checkPasswordLength(plain string) error {
	if len(strings.TrimSpace(plain)) < minPasswordLength {
		return fmt.Errorf("the password must be at least %d characters", minPasswordLength)
	}
	return nil
}

// CredentialsPath is where the admin password lives inside a data directory.
func CredentialsPath(dataDir string) string {
	return filepath.Join(dataDir, "web.json")
}
