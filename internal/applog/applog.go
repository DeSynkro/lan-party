// Package applog writes the application log into the data directory.
package applog

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// FileName is the log file name inside the data directory.
	FileName = "lan-party.log"
	maxSize  = 5 << 20
)

// Path is where the app log lives for a data directory.
func Path(dataDir string) string {
	return filepath.Join(dataDir, FileName)
}

// Open creates the data directory and opens the log for appending. A log at or
// above maxSize is rotated to .1 first; one previous log is kept.
func Open(dataDir string) (*os.File, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	_ = os.Chmod(dataDir, 0o700)

	path := Path(dataDir)
	if info, err := os.Stat(path); err == nil && info.Size() >= maxSize {
		old := path + ".1"
		_ = os.Remove(old)
		_ = os.Rename(path, old)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure log: %w", err)
	}
	return file, nil
}
