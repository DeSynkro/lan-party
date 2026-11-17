package buildinfo

import "testing"

func TestVersionHasAValue(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty")
	}
}
