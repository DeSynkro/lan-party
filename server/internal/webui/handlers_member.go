package webui

import (
	"net/http"

	"github.com/desynkro/lan-party/internal/group"
)

type memberPageView struct {
	BaseView
	GroupID string
	Member  MemberView
}

func (s *Server) handleApproval(w http.ResponseWriter, r *http.Request, session *Session) {
	approval := s.app.Approval()
	if approval == nil || approval.ID != r.PathValue("id") {
		s.renderError(w, r, http.StatusConflict, "That approval request is no longer waiting.")
		return
	}
	allow := r.PostFormValue("allow") == "yes"
	if !s.app.ResolveApproval(approval.ID, allow) {
		s.renderError(w, r, http.StatusConflict, "That approval request could not be resolved.")
		return
	}
	outcome := "denied"
	if allow {
		outcome = "allowed"
	}
	s.record(r, "approval", outcome+" id="+approval.ID)
	http.Redirect(w, r, "/groups/"+approval.GroupID, http.StatusSeeOther)
}

func (s *Server) handleMemberPage(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	member, ok := findMember(g, r.PathValue("mid"))
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That member does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "member.html", memberPageView{
		BaseView: s.base(r, member.Name),
		GroupID:  g.ID,
		Member:   NewMemberView(member, s.app.HostPresence()),
	})
}

func (s *Server) handleMemberRemoveForm(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	member, ok := findMember(g, r.PathValue("mid"))
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That member does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "confirm.html", confirmView{
		BaseView: s.base(r, "Remove "+member.Name),
		Eyebrow:  "Remove member",
		Heading:  "Remove " + member.Name + "?",
		Message:  "They lose access to this group's services. They need a new invite to join again.",
		Action:   "/groups/" + g.ID + "/members/" + member.ID + "/remove",
		Confirm:  "Remove member",
		Cancel:   "/groups/" + g.ID + "/members/" + member.ID,
	})
}

func (s *Server) handleMemberRemove(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := s.app.RemoveMember(g.ID, r.PathValue("mid")); err != nil {
		s.renderGroupWithError(w, r, g, err.Error(), http.StatusConflict)
		return
	}
	s.record(r, "member", "removed id="+r.PathValue("mid"))
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func findMember(g group.Group, id string) (group.Member, bool) {
	for _, member := range g.Members {
		if member.ID == id {
			return member, true
		}
	}
	return group.Member{}, false
}
