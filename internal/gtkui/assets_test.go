package gtkui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindAssetRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFont(t, root)
	if got, ok := findAssetRoot(filepath.Join(root, "bin")); !ok || got != root {
		t.Fatalf("repository layout = %q, %v; want %q", got, ok, root)
	}

	installed := filepath.Join(root, "usr", "share", "lan-party")
	writeTestFont(t, installed)
	if got, ok := findAssetRoot(filepath.Join(root, "usr", "bin")); !ok || got != installed {
		t.Fatalf("installed layout = %q, %v; want %q", got, ok, installed)
	}

	emptyBase := ""
	if runtime.GOOS != "windows" {
		emptyBase = "/tmp"
	}
	empty, err := os.MkdirTemp(emptyBase, "lan-party-assets-empty-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(empty) })
	if _, ok := findAssetRoot(empty); ok {
		t.Fatal("found assets in an empty directory")
	}
}

func TestAssetRootEnvironmentOverride(t *testing.T) {
	root := t.TempDir()
	writeTestFont(t, root)
	t.Setenv("LANPARTY_ASSET_ROOT", root)
	got, err := assetRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("asset root = %q, want %q", got, root)
	}
}

func writeTestFont(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "brand", "fonts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SpaceGrotesk.ttf"), []byte("font"), 0o600); err != nil {
		t.Fatal(err)
	}
}
