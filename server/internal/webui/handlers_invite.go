package webui

import (
	"net/http"
	"strings"

	"github.com/desynkro/lan-party/internal/group"
)

type inviteFormView struct {
	BaseView
	GroupID       string
	PeerName      string
	SessionOff    bool
	OtherLiveName string
	HasAddress    bool
}

func (s *Server) handleInviteNew(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "invite.html", s.inviteFormView(r, g))
}

func (s *Server) inviteFormView(r *http.Request, g group.Group) inviteFormView {
	view := inviteFormView{
		BaseView:   s.base(r, "Invite a friend"),
		GroupID:    g.ID,
		SessionOff: s.app.LiveGroupID() != g.ID,
		HasAddress: strings.TrimSpace(s.app.Settings().PublicAddress) != "",
	}
	if live := s.app.LiveGroupID(); live != "" && live != g.ID {
		if other, ok := s.app.Group(live); ok {
			view.OtherLiveName = other.Name
		}
	}
	return view
}

func (s *Server) handleInviteCreate(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "The form could not be read.")
		return
	}
	peerName := strings.TrimSpace(r.PostFormValue("peerName"))
	view := s.inviteFormView(r, g)
	view.PeerName = peerName
	if peerName == "" {
		view.Error = "Give the invite a name so you know who it is for."
		s.render(w, r, http.StatusBadRequest, "invite.html", view)
		return
	}
	if other := view.OtherLiveName; other != "" {
		view.Error = "A session for " + other + " is already running. End it before starting " + g.Name + "."
		s.render(w, r, http.StatusConflict, "invite.html", view)
		return
	}
	if s.app.LiveGroupID() != g.ID {
		if err := s.app.StartSession(g.ID); err != nil {
			view.Error = err.Error()
			s.render(w, r, http.StatusConflict, "invite.html", view)
			return
		}
	}
	if _, err := s.app.CreateInvite(g.ID, peerName); err != nil {
		view.Error = err.Error()
		s.render(w, r, http.StatusConflict, "invite.html", view)
		return
	}
	s.record(r, "invite", "created for "+peerName)
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func (s *Server) handleInviteCancelForm(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	invite, ok := findInvite(g, r.PathValue("iid"))
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That invite does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "confirm.html", confirmView{
		BaseView: s.base(r, "Cancel invite"),
		Eyebrow:  "Cancel invite",
		Heading:  "Cancel the invite for " + invite.Name + "?",
		Message:  "The code stops working immediately. If they already joined, they stay a member.",
		Action:   "/groups/" + g.ID + "/invites/" + invite.ID + "/cancel",
		Confirm:  "Cancel invite",
		Cancel:   "/groups/" + g.ID,
	})
}

func (s *Server) handleInviteCancel(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := s.app.RemoveInvite(g.ID, r.PathValue("iid")); err != nil {
		s.renderGroupWithError(w, r, g, err.Error(), http.StatusConflict)
		return
	}
	s.record(r, "invite", "cancelled id="+r.PathValue("iid"))
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func findInvite(g group.Group, id string) (group.Invite, bool) {
	for _, invite := range g.Invites {
		if invite.ID == id {
			return invite, true
		}
	}
	return group.Invite{}, false
}
