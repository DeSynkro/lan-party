package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/secure"
)

func TestHeadlessDoesNotWriteAutostart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	paths := config.Paths{ConfigDir: filepath.Join(home, "cfg"), DataDir: filepath.Join(home, "data")}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := config.DefaultSettings()
	s.StartOnLogin = true
	if err := config.SaveSettings(paths, s); err != nil {
		t.Fatal(err)
	}

	a, err := NewWithOptions(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")), Options{DisableAutostart: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	entry := filepath.Join(home, "autostart", "lan-party.desktop")
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Fatalf("headless app wrote autostart entry: %v", err)
	}
}

func TestAutostartWritesWithoutHeadlessOption(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	paths := config.Paths{ConfigDir: filepath.Join(home, "cfg"), DataDir: filepath.Join(home, "data")}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	s := config.DefaultSettings()
	s.StartOnLogin = true
	if err := config.SaveSettings(paths, s); err != nil {
		t.Fatal(err)
	}

	a, err := NewWithOptions(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	entry := filepath.Join(home, "autostart", "lan-party.desktop")
	if _, err := os.Stat(entry); err != nil {
		t.Fatalf("autostart entry missing: %v", err)
	}
}

func TestDisableRelayForcesDirect(t *testing.T) {
	paths := config.Paths{ConfigDir: t.TempDir(), DataDir: t.TempDir()}
	s := config.DefaultSettings()
	s.UseRelay = true
	s.RelayHost = "relay.example.net"
	s.RelayPort = 7443
	s.RelayID = strings.Repeat("a", 64)
	if err := config.SaveSettings(paths, s); err != nil {
		t.Fatal(err)
	}
	a, err := NewWithOptions(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")), Options{DisableAutostart: true, DisableRelay: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.Settings().UseRelay {
		t.Fatal("relay still enabled")
	}
	if a.Settings().RelayHost != "" || a.Settings().RelayID != "" {
		t.Fatal("relay identity still present")
	}
}

func TestDisableRelayNormalizesGroupOverrides(t *testing.T) {
	paths := config.Paths{ConfigDir: t.TempDir(), DataDir: t.TempDir()}
	store := group.OpenStore(filepath.Join(paths.DataDir, "groups.json"))
	g := group.CreateHostGroup("Game night", "Server")
	g.OverrideSettings = true
	g.UseRelay = true
	g.RelayCustom = true
	g.RelayHost = "relay.example.net"
	g.RelayPort = 7443
	g.RelayID = strings.Repeat("b", 64)
	if err := store.Save([]group.Group{g}); err != nil {
		t.Fatal(err)
	}

	a, err := NewWithOptions(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")), Options{DisableAutostart: true, DisableRelay: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	got, ok := a.Group(g.ID)
	if !ok {
		t.Fatal("group missing")
	}
	if got.OverrideSettings || got.UseRelay || got.RelayCustom || got.RelayHost != "" || got.RelayID != "" {
		t.Fatalf("group relay fields survived: %+v", got)
	}
}
