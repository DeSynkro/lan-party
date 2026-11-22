package webui

import (
	"strings"
	"testing"

	"github.com/desynkro/lan-party/internal/group"
)

func TestModalShellAndLinks(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Services: []group.Service{{ID: "s1", Name: "Game", Proto: "tcp", Target: "127.0.0.1", Ports: []int{25565}}},
		Members:  []group.Member{{ID: "m1", Name: "Ash", State: group.MemberApproved}},
	})
	groupPage := srv.get("/groups/g1").Body.String()
	for _, want := range []string{
		`<dialog id="modal"`,
		`href="/groups/g1/services/new" data-modal`,
		`href="/groups/g1/services/s1" data-modal`,
		`href="/groups/g1/services/s1/remove" data-modal`,
		`href="/groups/g1/invites/new" data-modal`,
		`href="/groups/g1/delete" data-modal`,
		`href="/groups/g1/members/m1" data-modal`,
	} {
		if !strings.Contains(groupPage, want) {
			t.Fatalf("group page is missing %s", want)
		}
	}

	dashboard := srv.get("/").Body.String()
	if !strings.Contains(dashboard, `href="/groups/new" data-modal`) {
		t.Fatal("dashboard new-group link does not open the modal")
	}
}
