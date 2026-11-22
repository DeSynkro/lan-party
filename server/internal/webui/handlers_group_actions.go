package webui

import (
	"net/http"
	"strings"
)

type groupFormView struct {
	BaseView
	Name     string
	HostName string
}

type confirmView struct {
	BaseView
	Eyebrow string
	Heading string
	Message string
	Action  string
	Confirm string
	Cancel  string
	Hidden  map[string]string
}

func (s *Server) handleGroupNew(w http.ResponseWriter, r *http.Request, session *Session) {
	s.render(w, r, http.StatusOK, "group_form.html", groupFormView{
		BaseView: s.base(r, "New group"),
		HostName: s.app.DeviceName(),
	})
}

func (s *Server) handleGroupCreate(w http.ResponseWriter, r *http.Request, session *Session) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "The form could not be read.")
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	hostName := strings.TrimSpace(r.PostFormValue("hostName"))
	g, err := s.app.CreateGroup(name, hostName)
	if err != nil {
		base := s.base(r, "New group")
		base.Error = err.Error()
		s.render(w, r, http.StatusBadRequest, "group_form.html", groupFormView{
			BaseView: base,
			Name:     name,
			HostName: hostName,
		})
		return
	}
	s.record(r, "group", "created id="+g.ID)
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func (s *Server) handleGroupDeleteForm(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "confirm.html", confirmView{
		BaseView: s.base(r, "Delete "+g.Name),
		Eyebrow:  "Delete group",
		Heading:  "Delete " + g.Name + "?",
		Message:  "This removes the group, its services, invites and members. The host key for this group is deleted, so old invites stop working.",
		Action:   "/groups/" + g.ID + "/delete",
		Confirm:  "Delete group",
		Cancel:   "/groups/" + g.ID,
	})
}

func (s *Server) handleGroupDelete(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := s.app.DeleteGroup(g.ID); err != nil {
		s.renderError(w, r, http.StatusConflict, err.Error())
		return
	}
	s.record(r, "group", "deleted id="+g.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleGroupStart(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := s.app.StartSession(g.ID); err != nil {
		s.renderGroupWithError(w, r, g, err.Error(), http.StatusConflict)
		return
	}
	s.record(r, "session", "started group="+g.ID)
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func (s *Server) handleGroupStop(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := s.app.StopSession(); err != nil {
		s.renderGroupWithError(w, r, g, err.Error(), http.StatusConflict)
		return
	}
	s.record(r, "session", "stopped")
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}
