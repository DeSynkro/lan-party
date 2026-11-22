package webui

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/preset"
)

type serviceFormView struct {
	BaseView
	GroupID   string
	ServiceID string
	Name      string
	Proto     string
	Target    string
	PortsText string
	Hint      string
	Presets   []preset.Preset
	Editing   bool
}

func (s *Server) handleServiceNew(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	view := serviceFormView{
		BaseView: s.base(r, "New service"),
		GroupID:  g.ID,
		Proto:    "tcp",
		Target:   "127.0.0.1",
		Presets:  preset.All(),
	}
	if id := r.URL.Query().Get("preset"); id != "" {
		for _, p := range preset.All() {
			if p.ID != id {
				continue
			}
			view.Name = p.Name
			view.Proto = p.Proto
			view.PortsText = joinPorts(p.Ports)
			view.Hint = p.Hint
			break
		}
	}
	s.render(w, r, http.StatusOK, "service_form.html", view)
}

func (s *Server) handleServiceEdit(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	service, ok := findService(g, r.PathValue("sid"))
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That service does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "service_form.html", serviceFormView{
		BaseView:  s.base(r, "Edit "+service.Name),
		GroupID:   g.ID,
		ServiceID: service.ID,
		Name:      service.Name,
		Proto:     service.Proto,
		Target:    service.Target,
		PortsText: joinPorts(service.Ports),
		Hint:      service.Hint,
		Presets:   preset.All(),
		Editing:   true,
	})
}

func (s *Server) handleServiceCreate(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	service, view, err := s.serviceFromForm(r, g, "")
	if err != nil {
		s.render(w, r, http.StatusBadRequest, "service_form.html", view)
		return
	}
	if err := s.app.AddService(g.ID, service); err != nil {
		base := view.BaseView
		base.Error = err.Error()
		view.BaseView = base
		s.render(w, r, http.StatusConflict, "service_form.html", view)
		return
	}
	s.record(r, "service", "added id="+service.ID)
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func (s *Server) handleServiceUpdate(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if _, ok := findService(g, r.PathValue("sid")); !ok {
		s.renderError(w, r, http.StatusNotFound, "That service does not exist.")
		return
	}
	service, view, err := s.serviceFromForm(r, g, r.PathValue("sid"))
	if err != nil {
		s.render(w, r, http.StatusBadRequest, "service_form.html", view)
		return
	}
	if err := s.app.UpdateService(g.ID, service); err != nil {
		base := view.BaseView
		base.Error = err.Error()
		view.BaseView = base
		s.render(w, r, http.StatusConflict, "service_form.html", view)
		return
	}
	s.record(r, "service", "updated id="+service.ID)
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func (s *Server) handleServiceRemoveForm(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	service, ok := findService(g, r.PathValue("sid"))
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That service does not exist.")
		return
	}
	s.render(w, r, http.StatusOK, "confirm.html", confirmView{
		BaseView: s.base(r, "Remove "+service.Name),
		Eyebrow:  "Remove service",
		Heading:  "Remove " + service.Name + "?",
		Message:  "Friends will no longer be able to reach this service. The game server itself is not touched.",
		Action:   "/groups/" + g.ID + "/services/" + service.ID + "/remove",
		Confirm:  "Remove service",
		Cancel:   "/groups/" + g.ID,
	})
}

func (s *Server) handleServiceRemove(w http.ResponseWriter, r *http.Request, session *Session) {
	g, ok := s.requireHostGroup(r)
	if !ok {
		s.renderError(w, r, http.StatusNotFound, "That group does not exist.")
		return
	}
	if err := s.app.RemoveService(g.ID, r.PathValue("sid")); err != nil {
		s.renderGroupWithError(w, r, g, err.Error(), http.StatusConflict)
		return
	}
	s.record(r, "service", "removed id="+r.PathValue("sid"))
	http.Redirect(w, r, "/groups/"+g.ID, http.StatusSeeOther)
}

func (s *Server) serviceFromForm(r *http.Request, g group.Group, serviceID string) (group.Service, serviceFormView, error) {
	if err := r.ParseForm(); err != nil {
		return group.Service{}, serviceFormView{}, err
	}
	ports, err := group.ParsePorts(r.PostFormValue("ports"))
	service := group.Service{
		ID:     serviceID,
		Name:   strings.TrimSpace(r.PostFormValue("name")),
		Proto:  strings.TrimSpace(r.PostFormValue("proto")),
		Target: strings.TrimSpace(r.PostFormValue("target")),
		Ports:  ports,
		Hint:   strings.TrimSpace(r.PostFormValue("hint")),
	}
	if service.ID == "" {
		service.ID = group.NewID()
	}
	view := serviceFormView{
		BaseView:  s.base(r, "Service"),
		GroupID:   g.ID,
		ServiceID: service.ID,
		Name:      service.Name,
		Proto:     service.Proto,
		Target:    service.Target,
		PortsText: r.PostFormValue("ports"),
		Hint:      service.Hint,
		Presets:   preset.All(),
		Editing:   serviceID != "",
	}
	if err != nil {
		base := view.BaseView
		base.Error = err.Error()
		view.BaseView = base
		return service, view, err
	}
	if err := group.ValidateService(service); err != nil {
		base := view.BaseView
		base.Error = err.Error()
		view.BaseView = base
		return service, view, err
	}
	return service, view, nil
}

func findService(g group.Group, id string) (group.Service, bool) {
	for _, service := range g.Services {
		if service.ID == id {
			return service, true
		}
	}
	return group.Service{}, false
}

func joinPorts(ports []int) string {
	parts := make([]string, len(ports))
	for i, port := range ports {
		parts[i] = strconv.Itoa(port)
	}
	return strings.Join(parts, ", ")
}
