package webui

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const sessionCookieName = "lanparty_web_session"

// Session is one logged-in browser.
type Session struct {
	ID       string
	CSRF     string
	Created  time.Time
	LastSeen time.Time

	store *SessionStore
}

// SessionStore holds the live sessions in memory. A restart logs everyone out.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	idle     time.Duration
	absolute time.Duration
	now      func() time.Time
}

// NewSessionStore creates an empty store.
func NewSessionStore(idle, absolute time.Duration) *SessionStore {
	return &SessionStore{
		sessions: map[string]*Session{},
		idle:     idle,
		absolute: absolute,
		now:      time.Now,
	}
}

// Create makes a new session with a random ID and CSRF token.
func (s *SessionStore) Create() (*Session, error) {
	id, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	session := &Session{ID: id, CSRF: csrf, Created: now, LastSeen: now, store: s}
	s.sessions[id] = session
	return session, nil
}

// Get returns a live session and touches it. Expired sessions are removed.
func (s *SessionStore) Get(id string) (*Session, bool) {
	if id == "" {
		return nil, false
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return nil, false
	}
	if s.expiredLocked(session, now) {
		delete(s.sessions, id)
		return nil, false
	}
	session.LastSeen = now
	return session, true
}

// Destroy removes one session.
func (s *SessionStore) Destroy(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// DestroyAll removes every session, for a password change or shutdown.
func (s *SessionStore) DestroyAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = map[string]*Session{}
}

// Empty reports whether no sessions remain. Used by tests.
func (s *SessionStore) Empty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions) == 0
}

func (s *SessionStore) pruneLocked(now time.Time) {
	for id, session := range s.sessions {
		if s.expiredLocked(session, now) {
			delete(s.sessions, id)
		}
	}
}

func (s *SessionStore) expiredLocked(session *Session, now time.Time) bool {
	if now.Sub(session.LastSeen) > s.idle {
		return true
	}
	return now.Sub(session.Created) > s.absolute
}

// SetSessionCookie writes the session cookie.
func SetSessionCookie(w http.ResponseWriter, id string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// SessionID reads the session cookie from a request.
func SessionID(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func randomToken(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
