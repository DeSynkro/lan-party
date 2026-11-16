// Package config resolves where the app keeps its files.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Dir is where settings live.
func Dir() string {
	if root, ok := portableRoot(); ok {
		return filepath.Join(root, "data")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "lan-party"
	}
	return filepath.Join(base, "lan-party")
}

// DataDir is where the group store lives. On Windows that is local app data,
// because a roaming profile should not carry a machine's tunnel state.
func DataDir() string {
	if root, ok := portableRoot(); ok {
		return filepath.Join(root, "data")
	}
	if runtime.GOOS == "windows" {
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "LAN Party")
		}
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "lan-party"
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "lan-party")
}

// portableRoot reports the folder beside the binary when a file named
// "portable" sits there. Used by the AppImage and the Windows zip so the app
// can keep its data on a stick.
func portableRoot() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	root := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(root, "portable")); err != nil {
		return "", false
	}
	return root, true
}
