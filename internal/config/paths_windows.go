//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// Resolve uses the per-user Windows profile directories. Roaming AppData holds
// the small settings file; Local AppData holds groups and keys.
func Resolve() Paths {
	configBase, err := os.UserConfigDir()
	if err != nil || configBase == "" {
		configBase = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}

	dataBase, err := os.UserCacheDir()
	if err != nil || dataBase == "" {
		dataBase = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}

	return Paths{
		ConfigDir: filepath.Join(configBase, "lan-party"),
		DataDir:   filepath.Join(dataBase, "lan-party"),
	}
}
