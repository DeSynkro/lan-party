package secure

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileStorePersistsWithOwnerOnlyFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "keys")
	kr := NewFileStore(dir)
	store := NewStore(kr)

	first, err := store.LoadOrCreate("host/group-1")
	if err != nil {
		t.Fatal(err)
	}

	reopened := NewStore(NewFileStore(dir))
	second, ok, err := reopened.Load("host/group-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the identity was not found after reopening")
	}
	if !first.Public.Equal(second.Public) {
		t.Fatal("the identity changed between loads")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(kr.path(keyService, "host/group-1"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("key file mode is %o, want 600", perm)
		}
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Fatalf("key directory mode is %o, want 700", perm)
		}
	}
}

func TestFileStoreRejectsLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses profile ACLs, not Unix modes")
	}
	dir := filepath.Join(t.TempDir(), "keys")
	kr := NewFileStore(dir)
	if _, err := NewStore(kr).LoadOrCreate("device/group-1"); err != nil {
		t.Fatal(err)
	}
	path := kr.path(keyService, "device/group-1")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	_, ok, err := NewStore(kr).Load("device/group-1")
	if err == nil {
		t.Fatal("a key file readable by other users was accepted")
	}
	if ok {
		t.Fatal("a rejected key file reported an identity")
	}
}

func TestFileStoreDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "keys")
	kr := NewFileStore(dir)
	store := NewStore(kr)
	if _, err := store.LoadOrCreate("device/group-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("device/group-1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Load("device/group-1"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("the identity still exists after delete")
	}
	if err := store.Delete("device/group-1"); err != nil {
		t.Fatalf("deleting a missing identity failed: %v", err)
	}
}
