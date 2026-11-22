package webui

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxFormBytes bounds every form body.
const maxFormBytes = 64 << 10

const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

type contextKey int

const sessionContextKey contextKey = 1

// SessionFrom returns the authenticated session placed by RequireAuth.
func SessionFrom(r *http.Request) *Session {
	session, _ := r.Context().Value(sessionContextKey).(*Session)
	return session
}

// SecurityHeaders sets the response headers every page needs.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cache-Control", "no-store")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth rejects unauthenticated requests and exposes the session.
func RequireAuth(store *SessionStore, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := store.Get(SessionID(r))
		if !ok {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, session)))
	}
}

// CheckOrigin verifies that a POST came from this site. Browsers always send
// Origin on form posts; a missing Origin is allowed only for non-browser
// clients, which still need a CSRF token.
func CheckOrigin(r *http.Request, allowedOrigin string) error {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		if r.Header.Get("Sec-Fetch-Site") != "" {
			return errors.New("missing origin")
		}
		return nil
	}
	if allowedOrigin != "" && origin == allowedOrigin {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return errors.New("invalid origin")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("invalid origin scheme")
	}
	if !strings.EqualFold(u.Host, r.Host) {
		return errors.New("origin does not match host")
	}
	return nil
}

// CheckCSRF compares the submitted token with the session token.
func CheckCSRF(r *http.Request, session *Session) error {
	if session == nil {
		return errors.New("no session")
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	got := r.PostFormValue("csrf")
	if got == "" {
		return errors.New("missing csrf token")
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(session.CSRF)) != 1 {
		return errors.New("invalid csrf token")
	}
	return nil
}

// LimitBody caps the request body so a large form cannot exhaust memory.
func LimitBody(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

// RetryAfterSeconds renders a wait as a whole number of seconds, at least one.
func RetryAfterSeconds(wait time.Duration) int {
	seconds := int((wait + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
