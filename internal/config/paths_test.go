package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUsesXDG(t *testing.T) {
	configHome := t.TempDir()
	dataHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	got := Resolve()
	if want := filepath.Join(configHome, "lan-party"); got.ConfigDir != want {
		t.Fatalf("ConfigDir = %q, want %q", got.ConfigDir, want)
	}
	if want := filepath.Join(dataHome, "lan-party"); got.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", got.DataDir, want)
	}
}

func TestResolveFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	got := Resolve()
	if want := filepath.Join(home, ".config", "lan-party"); got.ConfigDir != want {
		t.Fatalf("ConfigDir = %q, want %q", got.ConfigDir, want)
	}
	if want := filepath.Join(home, ".local", "share", "lan-party"); got.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", got.DataDir, want)
	}
}

func TestResolveCreatesNothing(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_ = Resolve()
	entries, err := os.ReadDir(configHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Resolve created %d entries", len(entries))
	}
}
