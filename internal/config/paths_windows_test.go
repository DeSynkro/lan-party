//go:build windows

package config

import (
	"path/filepath"
	"testing"
)

func TestResolveUsesWindowsProfile(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	t.Setenv("AppData", configHome)
	t.Setenv("LocalAppData", dataHome)

	got := Resolve()
	if got.ConfigDir != filepath.Join(configHome, "lan-party") {
		t.Fatalf("config dir = %q", got.ConfigDir)
	}
	if got.DataDir != filepath.Join(dataHome, "lan-party") {
		t.Fatalf("data dir = %q", got.DataDir)
	}
}
