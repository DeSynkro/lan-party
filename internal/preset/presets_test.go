package preset

import (
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestPresetsAreValid(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range All() {
		if seen[p.ID] {
			t.Errorf("duplicate preset id %q", p.ID)
		}
		seen[p.ID] = true
		if p.Hint == "" {
			t.Errorf("%s has no hint", p.Name)
		}
		service := group.Service{
			Name:   p.Name,
			Proto:  p.Proto,
			Target: "192.168.1.10",
			Ports:  p.Ports,
		}
		if err := group.ValidateService(service); err != nil {
			t.Errorf("%s: %v", p.Name, err)
		}
	}
	if len(seen) < 10 {
		t.Fatalf("only %d presets", len(seen))
	}
}

func TestAllReturnsCopy(t *testing.T) {
	first := All()
	first[0].Name = "changed"
	if All()[0].Name == "changed" {
		t.Fatal("All returned the shared slice")
	}
}

func TestByID(t *testing.T) {
	p, ok := ByID("minecraft-java")
	if !ok {
		t.Fatal("minecraft-java is missing")
	}
	if p.Proto != "tcp" || len(p.Ports) != 1 || p.Ports[0] != 25565 {
		t.Fatalf("preset = %+v", p)
	}
	if _, ok := ByID("does-not-exist"); ok {
		t.Fatal("unknown preset was found")
	}
}
