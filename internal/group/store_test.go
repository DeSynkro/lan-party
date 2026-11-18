package group

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return OpenStore(filepath.Join(t.TempDir(), "lan-party", "groups.json"))
}

func TestStoreMissingFile(t *testing.T) {
	store := testStore(t)
	groups, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("loaded %d groups", len(groups))
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store := testStore(t)
	created := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	want := CreateHostGroup("Friday Night", "Desynkro")
	want.CreatedAt = created
	want.PublicAddress = "group.example.net"
	want.AppPort = 41000
	if err := want.AddService(Service{
		Name:   "Minecraft",
		Proto:  "tcp",
		Target: "192.168.1.50",
		Ports:  []int{25565},
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Save([]Group{want}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("loaded %d groups", len(got))
	}
	if got[0].ID != want.ID || got[0].Name != want.Name || got[0].HostName != want.HostName {
		t.Fatalf("group = %+v", got[0])
	}
	if got[0].PublicAddress != want.PublicAddress || got[0].AppPort != want.AppPort {
		t.Fatalf("connection = %+v", got[0])
	}
	if len(got[0].Services) != 1 || got[0].Services[0].Target != "192.168.1.50" {
		t.Fatalf("services = %+v", got[0].Services)
	}

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("file mode = %o", mode)
	}
	dir, err := os.Stat(filepath.Dir(store.Path()))
	if err != nil {
		t.Fatal(err)
	}
	if mode := dir.Mode().Perm(); mode != 0o700 {
		t.Fatalf("dir mode = %o", mode)
	}
}

func TestStoreBadJSON(t *testing.T) {
	store := testStore(t)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("bad JSON was accepted")
	}
}

func TestStoreRejectsUnknownVersion(t *testing.T) {
	store := testStore(t)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte(`{"version":2,"groups":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("an unknown store version was accepted")
	}
}

func TestStoreRejectsOldArrayFormat(t *testing.T) {
	store := testStore(t)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.Load()
	if err == nil || !strings.Contains(err.Error(), "previous version") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreSavesEmptyList(t *testing.T) {
	store := testStore(t)
	if err := store.Save(nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("store is empty")
	}
	groups, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("loaded %d groups", len(groups))
	}
}

func TestStoreLoadsGroupWithoutConnectionFields(t *testing.T) {
	store := testStore(t)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"version":1,"groups":[{"id":"group-1","name":"Friday Night","role":"host","hostName":"Desynkro"}]}`
	if err := os.WriteFile(store.Path(), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	groups, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("loaded %d groups", len(groups))
	}
	if groups[0].PublicAddress != "" || groups[0].AppPort != 0 {
		t.Fatalf("connection = %+v", groups[0])
	}
}

func TestStoreKeepsInviteExpiry(t *testing.T) {
	store := testStore(t)
	want := CreateHostGroup("Friday Night", "Desynkro")
	created := time.Now().UTC()
	expires := created.Add(24 * time.Hour)
	if err := want.AddInvite(Invite{
		Name:      "Ash",
		TokenHash: "hash",
		CreatedAt: created,
		ExpiresAt: expires,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save([]Group{want}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Invites) != 1 {
		t.Fatalf("loaded = %+v", got)
	}
	if !got[0].Invites[0].ExpiresAt.Equal(expires) {
		t.Fatalf("expiry = %v, want %v", got[0].Invites[0].ExpiresAt, expires)
	}
}
