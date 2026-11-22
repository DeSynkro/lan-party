package webui

import (
	"net/http"
	"strconv"
	"time"
)

type loginView struct {
	BaseView
}

// authenticated wraps a handler with session, origin and CSRF checks.
func (s *Server) authenticated(next func(http.ResponseWriter, *http.Request, *Session)) http.HandlerFunc {
	return RequireAuth(s.sessions, func(w http.ResponseWriter, r *http.Request) {
		session := SessionFrom(r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := CheckOrigin(r, ""); err != nil {
				s.renderError(w, r, http.StatusForbidden, "That request did not come from this site.")
				return
			}
			if err := CheckCSRF(r, session); err != nil {
				s.renderError(w, r, http.StatusForbidden, "That form expired. Reload the page and try again.")
				return
			}
		}
		next(w, r, session)
	})
}

func (s *Server) base(r *http.Request, title string) BaseView {
	b := BaseView{Title: title}
	if session := SessionFrom(r); session != nil {
		b.Authenticated = true
		b.CSRF = session.CSRF
		b.Approval = NewApprovalView(s.app.Approval())
	}
	return b
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessions.Get(SessionID(r)); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderLogin(w, r, http.StatusOK, "")
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := CheckOrigin(r, ""); err != nil {
		s.renderError(w, r, http.StatusForbidden, "That request did not come from this site.")
		return
	}
	ip := s.clientIP(r)
	now := s.now()
	if ok, wait := s.limiter.Allow(ip, now); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(RetryAfterSeconds(wait)))
		s.renderLogin(w, r, http.StatusTooManyRequests, "Too many attempts. Try again in a minute.")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "The sign-in form could not be read.")
		return
	}
	if !s.creds.Verify(r.PostFormValue("password")) {
		if s.loginDelay > 0 {
			time.Sleep(s.loginDelay)
		}
		s.limiter.Fail(ip, now)
		s.renderLogin(w, r, http.StatusUnauthorized, "That password is not correct.")
		return
	}
	s.limiter.Success(ip)

	if old := SessionID(r); old != "" {
		s.sessions.Destroy(old)
	}
	session, err := s.sessions.Create()
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "Could not start a session.")
		return
	}
	SetSessionCookie(w, session.ID, s.secureRequest(r))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, session *Session) {
	s.sessions.Destroy(session.ID)
	ClearSessionCookie(w, s.secureRequest(r))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, status int, message string) {
	view := loginView{BaseView: s.base(r, "Sign in")}
	view.Error = message
	s.render(w, r, status, "login.html", view)
}
