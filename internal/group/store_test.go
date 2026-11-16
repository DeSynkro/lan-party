package group

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	store := OpenStore(path)

	g := Create("Friday Night", "Desynkro")
	if err := store.Save([]Group{g}); err != nil {
		t.Fatal(err)
	}

	groups, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].ID != g.ID || groups[0].Name != g.Name {
		t.Fatalf("got %+v, want %+v", groups[0], g)
	}
}

func TestStoreMissingFile(t *testing.T) {
	store := OpenStore(filepath.Join(t.TempDir(), "nothing.json"))
	groups, err := store.Load()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("got %d groups, want 0", len(groups))
	}
}

func TestStoreWritesOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	if err := OpenStore(path).Save(nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode = %o, want 600", mode)
	}
}

func TestCreatePutsHostInRoster(t *testing.T) {
	g := Create("Friday Night", "Desynkro")
	if len(g.Members) != 1 {
		t.Fatalf("got %d members, want 1", len(g.Members))
	}
	if !g.Members[0].IsHost {
		t.Fatal("host is not marked as host")
	}
	if g.ID == "" {
		t.Fatal("group has no id")
	}
}
