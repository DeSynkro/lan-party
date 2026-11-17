package gtkui

import (
	"path/filepath"
	"testing"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/secure"
)

func TestGroupSignatureTracksStructuralChanges(t *testing.T) {
	base := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(base, "config", "lan-party"),
		DataDir:   filepath.Join(base, "data", "lan-party"),
	}
	state, err := apppkg.NewWithStore(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })

	settings := state.Settings()
	settings.PublicAddress = "home.example.net"
	if err := state.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}

	g, err := state.CreateGroup("Friday Night", "Desynkro")
	if err != nil {
		t.Fatal(err)
	}
	u := &probeUI{state: state}

	sig := u.groupSignature(g)
	if got := u.groupSignature(g); got != sig {
		t.Fatal("the signature changed without a change to the group")
	}

	if err := state.AddService(g.ID, group.Service{
		ID:     "mc",
		Name:   "Minecraft",
		Proto:  "tcp",
		Target: "192.168.0.10",
		Ports:  []int{25565},
	}); err != nil {
		t.Fatal(err)
	}
	g, _ = state.Group(g.ID)
	withService := u.groupSignature(g)
	if withService == sig {
		t.Fatal("the signature did not change when a service was added")
	}

	if _, err := state.CreateInvite(g.ID, "Ash"); err != nil {
		t.Fatal(err)
	}
	g, _ = state.Group(g.ID)
	withInvite := u.groupSignature(g)
	if withInvite == withService {
		t.Fatal("the signature did not change when an invite was added")
	}
}
