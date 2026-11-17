package secure

import (
	"errors"
	"io/fs"
	"testing"
)

type fakeStore struct {
	values    map[string]string
	getErr    error
	setErr    error
	deleteErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{values: map[string]string{}}
}

func slot(service, user string) string {
	return service + "\x00" + user
}

func (f *fakeStore) Get(service, user string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	value, ok := f.values[slot(service, user)]
	if !ok {
		return "", fs.ErrNotExist
	}
	return value, nil
}

func (f *fakeStore) Set(service, user, secret string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.values[slot(service, user)] = secret
	return nil
}

func (f *fakeStore) Delete(service, user string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.values[slot(service, user)]; !ok {
		return fs.ErrNotExist
	}
	delete(f.values, slot(service, user))
	return nil
}

func TestLoadOrCreatePersists(t *testing.T) {
	kr := newFakeStore()
	store := NewStore(kr)

	first, err := store.LoadOrCreate("host/group-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.LoadOrCreate("host/group-1")
	if err != nil {
		t.Fatal(err)
	}

	if !first.Public.Equal(second.Public) {
		t.Fatal("the identity changed between loads")
	}
	if len(kr.values) != 1 {
		t.Fatalf("stored %d entries", len(kr.values))
	}
}

func TestLoadMissingIdentity(t *testing.T) {
	store := NewStore(newFakeStore())
	_, ok, err := store.Load("host/group-1")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("found an identity that was never stored")
	}
}

func TestLoadOrCreateRejectsFailingStore(t *testing.T) {
	kr := newFakeStore()
	kr.getErr = errors.New("the store is locked")
	store := NewStore(kr)

	if _, err := store.LoadOrCreate("host/group-1"); err == nil {
		t.Fatal("a failing store was treated as a missing key")
	}
	if len(kr.values) != 0 {
		t.Fatal("an identity was written while the store was failing")
	}
}

func TestLoadOrCreateRejectsCorruptSecret(t *testing.T) {
	kr := newFakeStore()
	kr.values[slot(keyService, "host/group-1")] = "not base64"
	store := NewStore(kr)

	if _, err := store.LoadOrCreate("host/group-1"); err == nil {
		t.Fatal("a damaged identity was accepted")
	}
}

func TestDeleteIdentity(t *testing.T) {
	kr := newFakeStore()
	store := NewStore(kr)
	if _, err := store.LoadOrCreate("device/group-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("device/group-1"); err != nil {
		t.Fatal(err)
	}
	if len(kr.values) != 0 {
		t.Fatal("the identity was not deleted")
	}
}
