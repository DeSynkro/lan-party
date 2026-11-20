package applog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenCreatesSecureLog(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	file, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("hello\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(Path(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %o, want 600", info.Mode().Perm())
	}
}

func TestOpenRotatesLargeLog(t *testing.T) {
	dataDir := t.TempDir()
	path := Path(dataDir)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, maxSize); err != nil {
		t.Fatal(err)
	}

	file, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	oldInfo, err := os.Stat(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if oldInfo.Size() != maxSize {
		t.Fatalf("rotated size = %d, want %d", oldInfo.Size(), maxSize)
	}
	newInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if newInfo.Size() != 0 {
		t.Fatalf("new log size = %d, want 0", newInfo.Size())
	}
}
