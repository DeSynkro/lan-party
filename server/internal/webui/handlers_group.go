package webui

import (
	"net/http"
	"strings"
	"time"

	"github.com/desynkro/lan-party/internal/group"
)

func (s *Server) handleGroupPage(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "group.html", s.buildGroupPage(r, g))
}

func (s *Server) renderGroupWithError(w http.ResponseWriter, r *http.Request, g group.Group, message string, status int) {
	view := s.buildGroupPage(r, g)
	view.Error = message
	s.render(w, r, status, "group.html", view)
}

func (s *Server) buildGroupPage(r *http.Request, g group.Group) GroupPageView {
	settings := s.app.Settings()
	endpoint, _ := s.app.GroupEndpoint(g.ID)
	appPort, err := s.app.GroupAppPort(g.ID)
	if err != nil {
		appPort = settings.AppPort
	}
	if appPort == 0 {
		appPort = settings.AppPort
	}
	presence := s.app.HostPresence()
	live := s.app.LiveGroupID() == g.ID
	now := time.Now()

	base := s.base(r, g.Name)
	base.GroupID = g.ID
	base.MemberSignature = memberSignature(g.Members)
	var openInvites []string
	for _, invite := range g.Invites {
		if invite.Open(now) {
			openInvites = append(openInvites, invite.ID)
		}
	}
	base.InviteSignature = strings.Join(openInvites, ",")

	view := GroupPageView{
		BaseView: base,
		Group:    NewGroupView(g, presence, live, 0, endpoint),
	}
	view.Group.AppPort = appPort
	view.Group.HostNote = s.app.HostNote()
	if approval := s.app.Approval(); approval != nil && approval.GroupID == g.ID {
		view.Approval = NewApprovalView(approval)
	}

	for _, service := range g.Services {
		view.Services = append(view.Services, NewServiceView(service))
	}
	for _, member := range g.Members {
		view.Members = append(view.Members, NewMemberView(member, presence))
	}
	for _, invite := range g.Invites {
		if !invite.Open(now) {
			continue
		}
		view.Invites = append(view.Invites, NewInviteView(invite, s.app.InviteCode(invite.ID), now))
	}
	return view
}

func (s *Server) requireHostGroup(r *http.Request) (group.Group, bool) {
	g, ok := s.app.Group(r.PathValue("id"))
	if !ok || !g.IsHost() {
		return group.Group{}, false
	}
	return g, true
}

func memberSignature(members []group.Member) string {
	ids := make([]string, len(members))
	for i, member := range members {
		ids[i] = member.ID
	}
	return strings.Join(ids, ",")
}
