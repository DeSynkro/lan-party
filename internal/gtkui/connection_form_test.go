package gtkui

import (
	"strings"
	"testing"
)

func TestConnectionEndpointNote(t *testing.T) {
	if got := connectionEndpointNote(true, false, 1, false); !strings.Contains(got, "cancels 1 open invite") {
		t.Fatalf("one invite note = %q", got)
	}
	if got := connectionEndpointNote(true, true, 2, true); !strings.Contains(got, "cancels 2 open invites") || !strings.Contains(got, "after the restart") {
		t.Fatalf("two invites restart note = %q", got)
	}
	if got := connectionEndpointNote(true, false, 0, false); got != "" {
		t.Fatalf("no-invite endpoint note = %q, want empty", got)
	}
	if got := connectionEndpointNote(false, true, 0, false); !strings.Contains(got, "router forward") {
		t.Fatalf("router note = %q", got)
	}
	if got := connectionEndpointNote(false, false, 0, false); got != "" {
		t.Fatalf("no-change note = %q, want empty", got)
	}
}
