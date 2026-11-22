package webui

import "net/http"

func (s *Server) handleGuide(w http.ResponseWriter, r *http.Request, session *Session) {
	s.render(w, r, http.StatusOK, "guide.html", s.base(r, "Guide"))
}
