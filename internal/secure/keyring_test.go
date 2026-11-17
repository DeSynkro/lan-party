package secure

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

type fakeKeyring struct {
	values    map[string]string
	getErr    error
	setErr    error
	deleteErr error
}

func newFakeKeyring() *fakeKeyring {
	return &fakeKeyring{values: map[string]string{}}
}

func slot(service, user string) string {
	return service + "\x00" + user
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	value, ok := f.values[slot(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (f *fakeKeyring) Set(service, user, secret string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.values[slot(service, user)] = secret
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.values[slot(service, user)]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.values, slot(service, user))
	return nil
}

func TestLoadOrCreatePersists(t *testing.T) {
	kr := newFakeKeyring()
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
	store := NewStore(newFakeKeyring())
	_, ok, err := store.Load("host/group-1")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("found an identity that was never stored")
	}
}

func TestLoadOrCreateRejectsLockedKeyring(t *testing.T) {
	kr := newFakeKeyring()
	kr.getErr = errors.New("keyring is locked")
	store := NewStore(kr)

	if _, err := store.LoadOrCreate("host/group-1"); err == nil {
		t.Fatal("a locked keyring was treated as a missing key")
	}
	if len(kr.values) != 0 {
		t.Fatal("an identity was written while the keyring was failing")
	}
}

func TestLoadOrCreateRejectsCorruptSecret(t *testing.T) {
	kr := newFakeKeyring()
	kr.values[slot(keyringService, "host/group-1")] = "not base64"
	store := NewStore(kr)

	if _, err := store.LoadOrCreate("host/group-1"); err == nil {
		t.Fatal("a damaged identity was accepted")
	}
}

func TestDeleteIdentity(t *testing.T) {
	kr := newFakeKeyring()
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
