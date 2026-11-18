package secure

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileStore stores each identity in its own owner-only file. It replaces the
// desktop keyring on Linux: no session bus, no unlock prompt, no daemon.
type FileStore struct {
	dir string
}

// NewFileStore stores identities under dir. The directory is created on the
// first write with mode 0700; each key file is mode 0600.
func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (f *FileStore) path(service, user string) string {
	name := base64.RawURLEncoding.EncodeToString([]byte(service + "\x00" + user))
	return filepath.Join(f.dir, name+".key")
}

func (f *FileStore) Get(service, user string) (string, error) {
	path := f.path(service, user)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fs.ErrNotExist
		}
		return "", fmt.Errorf("stat key file: %w", err)
	}
	if err := checkKeyFile(path, info); err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fs.ErrNotExist
		}
		return "", fmt.Errorf("read key file: %w", err)
	}
	return string(data), nil
}

func (f *FileStore) Set(service, user, secret string) error {
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	if err := os.Chmod(f.dir, 0o700); err != nil {
		return fmt.Errorf("secure key directory: %w", err)
	}

	tmp, err := os.CreateTemp(f.dir, ".key-*")
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure key file: %w", err)
	}
	if _, err := tmp.WriteString(secret); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write key file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close key file: %w", err)
	}
	if err := os.Rename(name, f.path(service, user)); err != nil {
		return fmt.Errorf("replace key file: %w", err)
	}
	return nil
}

func (f *FileStore) Delete(service, user string) error {
	err := os.Remove(f.path(service, user))
	if errors.Is(err, fs.ErrNotExist) {
		return fs.ErrNotExist
	}
	if err != nil {
		return fmt.Errorf("remove key file: %w", err)
	}
	return nil
}
