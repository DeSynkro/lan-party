package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	return Paths{
		ConfigDir: filepath.Join(t.TempDir(), "lan-party"),
		DataDir:   filepath.Join(t.TempDir(), "lan-party"),
	}
}

func TestSettingsDefaults(t *testing.T) {
	s := DefaultSettings()
	if s.AppPort != DefaultAppPort {
		t.Fatalf("AppPort = %d", s.AppPort)
	}
	if s.RelayPort != DefaultRelayPort {
		t.Fatalf("RelayPort = %d", s.RelayPort)
	}
	if s.ForgetFriendsDays != 5 {
		t.Fatalf("ForgetFriendsDays = %d", s.ForgetFriendsDays)
	}
	if s.UseRelay {
		t.Fatal("UseRelay should be off by default")
	}
	if !s.ApproveNewDevices {
		t.Fatal("ApproveNewDevices should be on by default")
	}
	if !s.CloseToTray {
		t.Fatal("CloseToTray should be on by default")
	}
	if !s.AutoReconnect {
		t.Fatal("AutoReconnect should be on by default")
	}
}

func TestLoadSettingsMissingFile(t *testing.T) {
	got, err := LoadSettings(testPaths(t))
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSettings() {
		t.Fatalf("got %+v", got)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	p := testPaths(t)
	want := DefaultSettings()
	want.PublicAddress = "home.example.net:40000"
	want.StartMinimised = true
	want.CloseToTray = true
	want.AutoReconnect = false

	if err := SaveSettings(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(p.ConfigDir, "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("file mode = %o", mode)
		}
		dir, err := os.Stat(p.ConfigDir)
		if err != nil {
			t.Fatal(err)
		}
		if mode := dir.Mode().Perm(); mode != 0o700 {
			t.Fatalf("dir mode = %o", mode)
		}
	}
}

func TestSaveSettingsRejectsBadPort(t *testing.T) {
	for _, port := range []int{0, -1, 65536} {
		s := DefaultSettings()
		s.AppPort = port
		if err := SaveSettings(testPaths(t), s); err == nil {
			t.Fatalf("port %d was accepted", port)
		}
	}
}

func TestSettingsRelayRoundTrip(t *testing.T) {
	p := testPaths(t)
	want := DefaultSettings()
	want.UseRelay = true
	want.RelayHost = "relay.example.net"
	want.RelayPort = 9443
	want.RelayID = strings.Repeat("ab", 32)

	if err := SaveSettings(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadSettingsFillsRelayPort(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"appPort":51820,"relayHost":"relay.example.net","relayID":"` + strings.Repeat("ab", 32) + `"}`
	if err := os.WriteFile(filepath.Join(p.ConfigDir, "settings.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.RelayPort != DefaultRelayPort {
		t.Fatalf("RelayPort = %d", got.RelayPort)
	}
}

func TestSettingsKeepsNeverForget(t *testing.T) {
	p := testPaths(t)
	s := DefaultSettings()
	s.ForgetFriendsDays = 0
	if err := SaveSettings(p, s); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.ForgetFriendsDays != 0 {
		t.Fatalf("ForgetFriendsDays = %d, want 0", got.ForgetFriendsDays)
	}
}

func TestSettingsInactiveFriendValidation(t *testing.T) {
	for _, days := range []int{1, 5, 365, 0} {
		s := DefaultSettings()
		s.ForgetFriendsDays = days
		if err := s.Validate(); err != nil {
			t.Errorf("days %d = %v", days, err)
		}
	}
	for _, days := range []int{-1, 366} {
		s := DefaultSettings()
		s.ForgetFriendsDays = days
		if err := s.Validate(); err == nil {
			t.Errorf("days %d was accepted", days)
		}
	}
}

func TestSettingsRelayValidation(t *testing.T) {
	valid := DefaultSettings()
	valid.UseRelay = true
	valid.RelayHost = "relay.example.net"
	valid.RelayID = strings.Repeat("ab", 32)
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid relay settings = %v", err)
	}

	for _, host := range []string{"192.0.2.10", "2001:db8::10", "[2001:db8::10]"} {
		s := valid
		s.RelayHost = host
		if err := s.Validate(); err != nil {
			t.Errorf("relay host %q = %v", host, err)
		}
	}

	for _, host := range []string{"", "relay example", "https://relay.example.net", "relay.example.net:7443"} {
		s := valid
		s.RelayHost = host
		if err := s.Validate(); err == nil {
			t.Errorf("relay host %q was accepted", host)
		}
	}

	for _, port := range []int{0, -1, 65536} {
		s := valid
		s.RelayPort = port
		if err := s.Validate(); err == nil {
			t.Errorf("relay port %d was accepted", port)
		}
	}

	for _, id := range []string{"", "abcd", strings.Repeat("zz", 32)} {
		s := valid
		s.RelayID = id
		if err := s.Validate(); err == nil {
			t.Errorf("Relay ID %q was accepted", id)
		}
	}

	// Turning relay off keeps the saved values but stops requiring them.
	off := DefaultSettings()
	if err := off.Validate(); err != nil {
		t.Fatalf("relay-off defaults = %v", err)
	}
	off.RelayHost = "not a host"
	off.RelayID = "short"
	if err := off.Validate(); err != nil {
		t.Fatalf("inactive relay values blocked direct mode: %v", err)
	}
	off.UseRelay = true
	if err := off.Validate(); err == nil {
		t.Fatal("activating invalid relay values was accepted")
	}
}

func TestLoadSettingsBadJSON(t *testing.T) {
	p := testPaths(t)
	if err := os.MkdirAll(p.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.ConfigDir, "settings.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(p); err == nil {
		t.Fatal("bad JSON was accepted")
	}
}

func TestValidateAddress(t *testing.T) {
	ok := []string{
		"",
		"home.example.net",
		"home.example.net:40000",
		"192.168.1.10",
		"192.168.1.10:40000",
		"::1",
		"[::1]:40000",
	}
	for _, s := range ok {
		if err := ValidateAddress(s); err != nil {
			t.Errorf("ValidateAddress(%q) = %v", s, err)
		}
	}

	bad := []string{
		"http://home.example.net",
		"home example.net",
		":40000",
		"home.example.net:0",
		"home.example.net:70000",
		"home.example.net/path",
		"home.example.net:40000:1",
	}
	for _, s := range bad {
		if err := ValidateAddress(s); err == nil {
			t.Errorf("ValidateAddress(%q) accepted an invalid address", s)
		}
	}
}
