package relay

import (
	"encoding/json"
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
	if profile.Host != "relay.example.net" || profile.Port != 7443 {
		t.Fatalf("profile host/port = %q/%d", profile.Host, profile.Port)
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

func TestSetupSeparateHostPort(t *testing.T) {
	dir := t.TempDir()
	profile, err := Setup(SetupOptions{
		Dir:        dir,
		PublicHost: "relay.example.net",
		PublicPort: 8443,
		Password:   "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Host != "relay.example.net" || profile.Port != 8443 {
		t.Fatalf("profile host/port = %q/%d", profile.Host, profile.Port)
	}
	if profile.Address != "relay.example.net:8443" {
		t.Fatalf("profile address = %q", profile.Address)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicAddress != "relay.example.net:8443" {
		t.Fatalf("config public address = %q", cfg.PublicAddress)
	}

	ipv6, err := Setup(SetupOptions{
		Dir:        t.TempDir(),
		PublicHost: "2001:db8::1",
		PublicPort: 7443,
		Password:   "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ipv6.Host != "2001:db8::1" || ipv6.Port != 7443 || ipv6.Address != "[2001:db8::1]:7443" {
		t.Fatalf("IPv6 profile = %+v", ipv6)
	}
}

func TestHostProfileHostPortFallback(t *testing.T) {
	profile := HostProfile{Address: "relay.example.net:8443"}
	host, port, err := profile.HostPort()
	if err != nil {
		t.Fatal(err)
	}
	if host != "relay.example.net" || port != 8443 {
		t.Fatalf("host/port = %q/%d", host, port)
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

func TestSetPasswordUpdatesConfigAndProfile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Setup(SetupOptions{Dir: dir, PublicAddress: "relay.example.net:7443", Password: "old password"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	_, fingerprint, err := LoadIdentity(KeyPath(ConfigPath(dir), cfg))
	if err != nil {
		t.Fatal(err)
	}
	if err := SetPassword(dir, "new password"); err != nil {
		t.Fatal(err)
	}

	cfg, err = LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("new password", cfg.PasswordSalt, cfg.PasswordHash) {
		t.Fatal("the new password does not verify")
	}
	if VerifyPassword("old password", cfg.PasswordSalt, cfg.PasswordHash) {
		t.Fatal("the old password still verifies")
	}

	data, err := os.ReadFile(ProfilePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var profile HostProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Password != "new password" {
		t.Fatalf("profile password = %q", profile.Password)
	}
	if profile.Fingerprint != fingerprint {
		t.Fatalf("profile fingerprint changed from %q to %q", fingerprint, profile.Fingerprint)
	}
	_, loadedFingerprint, err := LoadIdentity(KeyPath(ConfigPath(dir), cfg))
	if err != nil {
		t.Fatal(err)
	}
	if loadedFingerprint != fingerprint {
		t.Fatalf("rotation changed the fingerprint from %q to %q", fingerprint, loadedFingerprint)
	}
}

func TestSetPasswordRecreatesMissingProfile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Setup(SetupOptions{Dir: dir, PublicAddress: "relay.example.net:7443", Password: "old password"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	_, fingerprint, err := LoadIdentity(KeyPath(ConfigPath(dir), cfg))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ProfilePath(dir)); err != nil {
		t.Fatal(err)
	}
	if err := SetPassword(dir, "new password"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ProfilePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var profile HostProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Password != "new password" || profile.Fingerprint != fingerprint || profile.Address != "relay.example.net:7443" {
		t.Fatalf("recreated profile = %+v", profile)
	}
}

func TestSetPasswordRequiresPassword(t *testing.T) {
	if err := SetPassword(t.TempDir(), "   "); err == nil {
		t.Fatal("an empty password was accepted")
	}
}

func TestRotateIdentityUpdatesFingerprint(t *testing.T) {
	dir := t.TempDir()
	if _, err := Setup(SetupOptions{Dir: dir, PublicAddress: "relay.example.net:7443", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	keyPath := KeyPath(ConfigPath(dir), cfg)
	_, oldFingerprint, err := LoadIdentity(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	newFingerprint, err := RotateIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	if newFingerprint == oldFingerprint {
		t.Fatal("rotation kept the old fingerprint")
	}
	_, loadedFingerprint, err := LoadIdentity(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if loadedFingerprint != newFingerprint {
		t.Fatalf("loaded fingerprint = %q, want %q", loadedFingerprint, newFingerprint)
	}

	data, err := os.ReadFile(ProfilePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	var profile HostProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Fingerprint != newFingerprint {
		t.Fatalf("profile fingerprint = %q, want %q", profile.Fingerprint, newFingerprint)
	}
	if profile.Password != "secret" {
		t.Fatalf("rotation changed the password to %q", profile.Password)
	}
	cfg, err = LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("secret", cfg.PasswordSalt, cfg.PasswordHash) {
		t.Fatal("rotation changed the password hash")
	}
	if _, err := os.Stat(keyPath + ".new"); !os.IsNotExist(err) {
		t.Fatalf("temporary key still exists: %v", err)
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{keyPath, ConfigPath(dir), ProfilePath(dir)} {
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

func TestUpdatePublicAddressKeepsIdentityAndPassword(t *testing.T) {
	dir := t.TempDir()
	profile, err := Setup(SetupOptions{
		Dir:           dir,
		PublicAddress: "127.0.0.1:7443",
		Password:      "relay password",
	})
	if err != nil {
		t.Fatal(err)
	}

	changed, err := UpdatePublicAddress(dir, "relay.example.net", 8443, "")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("the public address was not changed")
	}
	cfg, err := LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicAddress != "relay.example.net:8443" {
		t.Fatalf("config address = %q", cfg.PublicAddress)
	}
	got, err := LoadProfile(ProfilePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != "relay.example.net:8443" || got.Host != "relay.example.net" || got.Port != 8443 {
		t.Fatalf("profile = %+v", got)
	}
	if got.Fingerprint != profile.Fingerprint || got.Password != profile.Password {
		t.Fatalf("identity or password changed: %+v", got)
	}

	changed, err = UpdatePublicAddress(dir, "relay.example.net", 8443, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("an unchanged address reported a change")
	}
	if _, err := UpdatePublicAddress(dir, "https://relay.example.net", 8443, ""); err == nil {
		t.Fatal("an invalid host was accepted")
	}
	cfg, err = LoadConfig(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicAddress != "relay.example.net:8443" {
		t.Fatalf("a failed update changed the config: %q", cfg.PublicAddress)
	}
}
