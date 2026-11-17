// Package config resolves where the app keeps its files.
package config

import (
	"os"
	"path/filepath"
)

// Paths are the two directories the app owns. Settings are small and belong
// beside the user's other configuration; group data is larger and belongs in
// the data directory.
type Paths struct {
	ConfigDir string
	DataDir   string
}

// Resolve follows the XDG base directory rules. During development the
// Makefile points both variables into .tmp.
func Resolve() Paths {
	home, _ := os.UserHomeDir()

	configBase := os.Getenv("XDG_CONFIG_HOME")
	if configBase == "" {
		configBase = filepath.Join(home, ".config")
	}

	dataBase := os.Getenv("XDG_DATA_HOME")
	if dataBase == "" {
		dataBase = filepath.Join(home, ".local", "share")
	}

	return Paths{
		ConfigDir: filepath.Join(configBase, "lan-party"),
		DataDir:   filepath.Join(dataBase, "lan-party"),
	}
}
