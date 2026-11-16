package autostart

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName = "LAN Party"
)

// Enable adds the app to the current user's login items. No admin needed.
func Enable(exec string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(valueName, fmt.Sprintf("%q", exec))
}

// Disable removes the login item. A missing value is not an error.
func Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	err = key.DeleteValue(valueName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
