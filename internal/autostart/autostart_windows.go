//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName = "LAN Party"
)

// Set writes or removes the per-user Run entry. The configHome argument is
// Linux-specific; Windows always uses HKCU.
func Set(_ string, enabled bool) error {
	return set(runKey, valueName, enabled)
}

func set(keyPath, name string, enabled bool) error {
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("open run key: %w", err)
		}
		defer key.Close()
		if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("remove autostart value: %w", err)
		}
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, `"`+executable+`"`); err != nil {
		return fmt.Errorf("write autostart value: %w", err)
	}
	return nil
}
