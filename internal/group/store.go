package group

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Store reads and writes groups as JSON. Writes are atomic and the file is
// readable by its owner only.
type Store struct {
	path string
}

// OpenStore points at a file. The file is created on the first save.
func OpenStore(path string) *Store {
	return &Store{path: path}
}

// Path is where the store lives, for messages that need to say so.
func (s *Store) Path() string { return s.path }

// Load returns every group. A missing file is not an error.
func (s *Store) Load() ([]Group, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read groups: %w", err)
	}
	var groups []Group
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("parse groups: %w", err)
	}
	return groups, nil
}

// Save replaces the stored groups.
func (s *Store) Save(groups []Group) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return fmt.Errorf("encode groups: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write groups: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace groups: %w", err)
	}
	return nil
}
