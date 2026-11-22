package webui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCredentialsGeneratesAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web.json")
	c, generated, err := EnsureCredentials(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if generated == "" {
		t.Fatal("no generated password")
	}
	if !c.Verify(generated) {
		t.Fatal("generated password does not verify")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	c2, generated2, err := EnsureCredentials(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if generated2 != "" {
		t.Fatal("password regenerated on second run")
	}
	if !c2.Verify(generated) {
		t.Fatal("persisted password lost")
	}
}

func TestEnsureCredentialsUsesSuppliedPasswordOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web.json")
	c, generated, err := EnsureCredentials(path, "a supplied password")
	if err != nil {
		t.Fatal(err)
	}
	if generated != "" {
		t.Fatal("supplied password was regenerated")
	}
	if !c.Verify("a supplied password") {
		t.Fatal("supplied password rejected")
	}
	c2, _, err := EnsureCredentials(path, "a different password")
	if err != nil {
		t.Fatal(err)
	}
	if !c2.Verify("a supplied password") {
		t.Fatal("existing password replaced by environment")
	}
}

func TestSetPasswordReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web.json")
	if err := SetPassword(path, "a long enough password"); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Verify("a long enough password") {
		t.Fatal("new password rejected")
	}
}

func TestShortPasswordRejected(t *testing.T) {
	if err := SetPassword(filepath.Join(t.TempDir(), "web.json"), "short"); err == nil {
		t.Fatal("short password accepted")
	}
}

func TestCorruptCredentialsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredentials(path); err == nil {
		t.Fatal("corrupt file accepted")
	}
}

func TestMissingCredentialsAreNotAnError(t *testing.T) {
	_, err := LoadCredentials(filepath.Join(t.TempDir(), "web.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("missing file error = %v", err)
	}
}
