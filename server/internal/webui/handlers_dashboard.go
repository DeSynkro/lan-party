package webui

import (
	"net/http"

	"github.com/desynkro/lan-party/internal/group"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request, session *Session) {
	settings := s.app.Settings()
	liveID := s.app.LiveGroupID()
	presence := s.app.HostPresence()
	view := DashboardView{
		BaseView: s.base(r, "Dashboard"),
		Pending:  NewApprovalView(s.app.Approval()),
	}
	for _, g := range hostGroups(s.app.Groups()) {
		endpoint, _ := s.app.GroupEndpoint(g.ID)
		groupView := NewGroupView(g, presence, liveID == g.ID, 0, endpoint)
		groupView.AppPort = settings.AppPort
		view.Groups = append(view.Groups, groupView)
	}
	s.render(w, r, http.StatusOK, "dashboard.html", view)
}

// hostGroups returns every host group, dropping friend groups a copied data
// directory might contain.
func hostGroups(groups []group.Group) []group.Group {
	out := make([]group.Group, 0, len(groups))
	for _, g := range groups {
		if g.IsHost() {
			out = append(out, g)
		}
	}
	return out
}
