package relay

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSetupCreatesRelayAndProfile(t *testing.T) {
	dir := t.TempDir()
	password := "correct horse battery staple"
	profile, err := Setup(SetupOptions{
		Dir:           dir,
		PublicAddress: "relay.example.net:7443",
		Listen:        ":7443",
		Password:      password,
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Address != "relay.example.net:7443" || profile.Password != password {
		t.Fatalf("profile = %+v", profile)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(password, cfg.PasswordSalt, cfg.PasswordHash) {
		t.Fatal("stored password does not verify")
	}
	if VerifyPassword("wrong", cfg.PasswordSalt, cfg.PasswordHash) {
		t.Fatal("wrong password verified")
	}
	cert, fingerprint, err := LoadIdentity(KeyPath(ConfigPath(dir), cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 || fingerprint != profile.Fingerprint {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, profile.Fingerprint)
	}

	if runtime.GOOS != "windows" {
		for _, path := range []string{KeyPath(ConfigPath(dir), cfg), ConfigPath(dir), ProfilePath(dir)} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode := info.Mode().Perm(); mode != 0o600 {
				t.Fatalf("%s mode = %o, want 600", filepath.Base(path), mode)
			}
		}
	}
}

func TestSetupDefaultsListenTo7443(t *testing.T) {
	dir := t.TempDir()
	if _, err := Setup(SetupOptions{
		Dir:           dir,
		PublicAddress: "relay.example.net:8443",
		Password:      "secret",
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":7443" {
		t.Fatalf("listen = %q, want :7443", cfg.Listen)
	}
	if cfg.PublicAddress != "relay.example.net:8443" {
		t.Fatalf("public address = %q", cfg.PublicAddress)
	}
}

func TestSetupRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	options := SetupOptions{
		Dir:           dir,
		PublicAddress: "relay.example.net:7443",
		Password:      "secret",
	}
	if _, err := Setup(options); err != nil {
		t.Fatal(err)
	}
	if _, err := Setup(options); err == nil {
		t.Fatal("a second setup overwrote the relay")
	}
}

func TestSetupKeepsExistingDirectoryMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix file modes")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Setup(SetupOptions{Dir: dir, PublicAddress: "relay.example.net:7443", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o755 {
		t.Fatalf("setup changed an existing directory to %o", mode)
	}
}

func TestSetupRefusesExistingKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, KeyFileName), []byte("existing key"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Setup(SetupOptions{
		Dir:           dir,
		PublicAddress: "relay.example.net:7443",
		Password:      "secret",
	})
	if err == nil {
		t.Fatal("setup overwrote an existing relay key")
	}
}

func TestLoadConfigRejectsInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix file modes")
	}
	dir := t.TempDir()
	if _, err := Setup(SetupOptions{Dir: dir, PublicAddress: "relay.example.net:7443", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ConfigPath(dir), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(ConfigPath(dir)); err == nil {
		t.Fatal("world-readable relay config was accepted")
	}
}

func TestLoadIdentityRejectsInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix file modes")
	}
	dir := t.TempDir()
	if _, err := Setup(SetupOptions{Dir: dir, PublicAddress: "relay.example.net:7443", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	keyPath := KeyPath(ConfigPath(dir), cfg)
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadIdentity(keyPath); err == nil {
		t.Fatal("world-readable relay key was accepted")
	}
}

func TestPublicAddressNormalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"relay.example.net", "relay.example.net:7443"},
		{"relay.example.net:9000", "relay.example.net:9000"},
		{"203.0.113.10", "203.0.113.10:7443"},
		{"2001:db8::1", "[2001:db8::1]:7443"},
	}
	for _, tc := range cases {
		got, _, err := normalizePublicAddress(tc.in)
		if err != nil {
			t.Errorf("normalizePublicAddress(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizePublicAddress(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
