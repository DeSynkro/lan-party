package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDataDirUsesXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG paths are not used on Windows")
	}
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	if got, want := DataDir(), filepath.Join("/tmp/xdg-data", "lan-party"); got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}

func TestDataDirFallsBackToHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG paths are not used on Windows")
	}
	t.Setenv("XDG_DATA_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := DataDir(), filepath.Join(home, ".local", "share", "lan-party"); got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}

func TestDirUsesConfigHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG paths are not used on Windows")
	}
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	if got, want := Dir(), filepath.Join("/tmp/xdg-config", "lan-party"); got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}
