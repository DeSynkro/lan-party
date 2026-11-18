//go:build linux

package config

import (
	"os"
	"path/filepath"
)

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
