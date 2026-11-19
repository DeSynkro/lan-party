package config

import (
	"os"
	"path/filepath"
	"runtime"
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
