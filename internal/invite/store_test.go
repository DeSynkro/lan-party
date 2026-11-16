package invite

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "joined.json")
	store := OpenStore(path)

	inv := sample(t)
	if err := store.Save([]Invite{inv}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GroupID != inv.GroupID {
		t.Fatalf("got %+v, want the invite back", got)
	}
}

func TestStoreMissingFile(t *testing.T) {
	store := OpenStore(filepath.Join(t.TempDir(), "nothing.json"))
	got, err := store.Load()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d invites, want 0", len(got))
	}
}

func TestStoreWritesOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "joined.json")
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
