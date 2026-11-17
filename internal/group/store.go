package group

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const storeVersion = 1

type diskFile struct {
	Version int     `json:"version"`
	Groups  []Group `json:"groups"`
}

// Store reads and writes groups as JSON. The file is readable by its owner
// only and is replaced atomically.
type Store struct {
	path string
}

// OpenStore points at a file. The file is created on the first save.
func OpenStore(path string) *Store {
	return &Store{path: path}
}

// Path is where the store lives, for messages that need to say so.
func (s *Store) Path() string { return s.path }

// Load returns every saved group. A missing file is not an error.
func (s *Store) Load() ([]Group, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read groups: %w", err)
	}

	var file diskFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse groups: %w", err)
	}
	if file.Version != storeVersion {
		return nil, fmt.Errorf("group store version %d is not supported", file.Version)
	}
	return file.Groups, nil
}

// Save replaces the stored groups.
func (s *Store) Save(groups []Group) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure data dir: %w", err)
	}

	if groups == nil {
		groups = []Group{}
	}
	data, err := json.MarshalIndent(diskFile{Version: storeVersion, Groups: groups}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode groups: %w", err)
	}

	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write groups: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("secure groups: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("write groups: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync groups: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close groups: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace groups: %w", err)
	}
	return nil
}
