package secure

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "lanparty"

// Keyring is the part of the OS secret store we use. Keeping it an interface
// lets the tests run without touching the real desktop keyring.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (osKeyring) Set(service, user, secret string) error {
	return keyring.Set(service, user, secret)
}

func (osKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// OSKeyring returns the desktop's Secret Service keyring.
func OSKeyring() Keyring {
	return osKeyring{}
}

// Store reads and writes identities by name.
type Store struct {
	kr Keyring
}

// NewStore wraps a keyring.
func NewStore(kr Keyring) *Store {
	return &Store{kr: kr}
}

// Load returns an identity if it exists. The bool is false when the keyring
// simply has no entry under that name.
func (s *Store) Load(name string) (Identity, bool, error) {
	secret, err := s.kr.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return Identity{}, false, nil
	}
	if err != nil {
		return Identity{}, false, fmt.Errorf("read identity: %w", err)
	}
	identity, err := parseIdentity(secret)
	if err != nil {
		return Identity{}, false, err
	}
	return identity, true, nil
}

// LoadOrCreate returns the identity stored under name, generating and storing
// a new Ed25519 key if there is none. A keyring failure is never treated as a
// missing key.
func (s *Store) LoadOrCreate(name string) (Identity, error) {
	identity, ok, err := s.Load(name)
	if err != nil {
		return Identity{}, err
	}
	if ok {
		return identity, nil
	}

	identity, err = newIdentity()
	if err != nil {
		return Identity{}, err
	}
	secret, err := encodeIdentity(identity)
	if err != nil {
		return Identity{}, err
	}
	if err := s.kr.Set(keyringService, name, secret); err != nil {
		return Identity{}, fmt.Errorf("store identity: %w", err)
	}
	return identity, nil
}

// Delete removes an identity. A missing entry is not an error.
func (s *Store) Delete(name string) error {
	err := s.kr.Delete(keyringService, name)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete identity: %w", err)
	}
	return nil
}
