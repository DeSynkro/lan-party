//go:build windows

package autostart

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestSetAndRemove(t *testing.T) {
	const keyPath = `Software\lan-party-tests\autostart`
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, keyPath) })

	if err := set(keyPath, "LAN Party", true); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	value, _, err := key.GetStringValue("LAN Party")
	key.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
		t.Fatalf("autostart value is not quoted: %q", value)
	}

	if err := set(keyPath, "LAN Party", false); err != nil {
		t.Fatal(err)
	}
	key, err = registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = key.GetStringValue("LAN Party")
	key.Close()
	if !errors.Is(err, registry.ErrNotExist) {
		t.Fatalf("autostart value still exists: %v", err)
	}
}

func TestDisableMissingValue(t *testing.T) {
	const keyPath = `Software\lan-party-tests\autostart-missing`
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, keyPath) })

	if err := set(keyPath, "LAN Party", false); err != nil {
		t.Fatalf("disable without a key: %v", err)
	}
}
