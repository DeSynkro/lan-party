package invite

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Store keeps the invites a friend has redeemed, so a group survives a restart.
type Store struct {
	path string
}

// OpenStore points at a file. The file is created on the first save.
func OpenStore(path string) *Store {
	return &Store{path: path}
}

// Load returns the saved invites. A missing file is not an error.
func (s *Store) Load() ([]Invite, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read invites: %w", err)
	}
	var invites []Invite
	if err := json.Unmarshal(data, &invites); err != nil {
		return nil, fmt.Errorf("parse invites: %w", err)
	}
	return invites, nil
}

// Save replaces the stored invites.
func (s *Store) Save(invites []Invite) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(invites, "", "  ")
	if err != nil {
		return fmt.Errorf("encode invites: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write invites: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace invites: %w", err)
	}
	return nil
}
