package preset

import (
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestPresetsAreSane(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range All() {
		if p.ID == "" || p.Name == "" {
			t.Errorf("preset %+v has no id or name", p)
		}
		if seen[p.ID] {
			t.Errorf("duplicate preset id %q", p.ID)
		}
		seen[p.ID] = true

		if !group.ValidProto(p.Proto) {
			t.Errorf("%s has invalid protocol %q", p.ID, p.Proto)
		}
		if len(p.Ports) == 0 {
			t.Errorf("%s has no ports", p.ID)
		}
		for _, port := range p.Ports {
			if port < 1 || port > 65535 {
				t.Errorf("%s has invalid port %d", p.ID, port)
			}
		}
		if _, err := group.ParsePorts(group.FormatPorts(p.Ports)); err != nil {
			t.Errorf("%s ports do not survive formatting: %v", p.ID, err)
		}
	}
}

func TestAllReturnsACopy(t *testing.T) {
	first := All()
	first[0].Name = "changed"
	if All()[0].Name == "changed" {
		t.Fatal("All() handed out the built-in slice")
	}
}

func TestByID(t *testing.T) {
	if _, ok := ByID("minecraft-java"); !ok {
		t.Fatal("minecraft-java missing")
	}
	if _, ok := ByID("nope"); ok {
		t.Fatal("unknown id resolved")
	}
}
