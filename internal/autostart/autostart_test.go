package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetAndRemove(t *testing.T) {
	home := t.TempDir()
	if err := Set(home, true); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(home, "autostart", fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"[Desktop Entry]",
		"Name=LAN Party",
		"Exec=\"",
		"X-GNOME-Autostart-enabled=true",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("entry missing %q", want)
		}
	}

	if err := Set(home, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the autostart entry was not removed")
	}
}

func TestDisableMissingEntry(t *testing.T) {
	if err := Set(t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
}
