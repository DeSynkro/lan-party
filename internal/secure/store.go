package secure

import (
	"errors"
	"fmt"
	"io/fs"
)

const keyService = "lanparty"

// SecretStore is the part of the key store we use. Keeping it an interface
// lets the tests run without touching the real store.
type SecretStore interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// Store reads and writes identities by name.
type Store struct {
	kr SecretStore
}

// NewStore wraps a key store.
func NewStore(kr SecretStore) *Store {
	return &Store{kr: kr}
}

// Load returns an identity if it exists. The bool is false when the store
// simply has no entry under that name.
func (s *Store) Load(name string) (Identity, bool, error) {
	secret, err := s.kr.Get(keyService, name)
	if errors.Is(err, fs.ErrNotExist) {
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
// a new Ed25519 key if there is none. A store failure is never treated as a
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
	if err := s.kr.Set(keyService, name, secret); err != nil {
		return Identity{}, fmt.Errorf("store identity: %w", err)
	}
	return identity, nil
}

// Delete removes an identity. A missing entry is not an error.
func (s *Store) Delete(name string) error {
	err := s.kr.Delete(keyService, name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete identity: %w", err)
	}
	return nil
}
