package webui

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"path"
	"strings"
	"time"

	"github.com/desynkro/lan-party/internal/buildinfo"
)

const (
	sessionIdle     = 12 * time.Hour
	sessionAbsolute = 7 * 24 * time.Hour
)

//go:embed templates static
var embedded embed.FS

// Server serves the web UI.
type Server struct {
	app        App
	cfg        Config
	sessions   *SessionStore
	limiter    *LoginLimiter
	creds      Credentials
	tpl        map[string]*template.Template
	mux        *http.ServeMux
	now        func() time.Time
	loginDelay time.Duration
}

// New builds the web UI. The data directory must already have credentials.
func New(a App, cfg Config) (*Server, error) {
	creds, err := LoadCredentials(CredentialsPath(cfg.DataDir))
	if err != nil {
		return nil, fmt.Errorf("load admin password: %w", err)
	}
	tpl, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	s := &Server{
		app:        a,
		cfg:        cfg,
		sessions:   NewSessionStore(sessionIdle, sessionAbsolute),
		limiter:    NewLoginLimiter(),
		creds:      creds,
		tpl:        tpl,
		mux:        http.NewServeMux(),
		now:        time.Now,
		loginDelay: 300 * time.Millisecond,
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.Handle("GET /static/", s.staticHandler())
	s.mux.HandleFunc("GET /login", s.handleLoginGet)
	s.mux.HandleFunc("POST /login", s.handleLoginPost)
	s.mux.HandleFunc("POST /logout", s.authenticated(s.handleLogout))
	s.mux.HandleFunc("GET /{$}", s.authenticated(s.handleHome))
	s.mux.HandleFunc("GET /groups/{id}", s.authenticated(s.handleGroupPage))
	s.mux.HandleFunc("GET /groups/new", s.authenticated(s.handleGroupNew))
	s.mux.HandleFunc("POST /groups", s.authenticated(s.handleGroupCreate))
	s.mux.HandleFunc("GET /groups/{id}/delete", s.authenticated(s.handleGroupDeleteForm))
	s.mux.HandleFunc("POST /groups/{id}/delete", s.authenticated(s.handleGroupDelete))
	s.mux.HandleFunc("POST /groups/{id}/start", s.authenticated(s.handleGroupStart))
	s.mux.HandleFunc("POST /groups/{id}/stop", s.authenticated(s.handleGroupStop))
	s.mux.HandleFunc("GET /groups/{id}/services/new", s.authenticated(s.handleServiceNew))
	s.mux.HandleFunc("POST /groups/{id}/services", s.authenticated(s.handleServiceCreate))
	s.mux.HandleFunc("GET /groups/{id}/services/{sid}", s.authenticated(s.handleServiceEdit))
	s.mux.HandleFunc("POST /groups/{id}/services/{sid}", s.authenticated(s.handleServiceUpdate))
	s.mux.HandleFunc("GET /groups/{id}/services/{sid}/remove", s.authenticated(s.handleServiceRemoveForm))
	s.mux.HandleFunc("POST /groups/{id}/services/{sid}/remove", s.authenticated(s.handleServiceRemove))
	s.mux.HandleFunc("GET /events", s.authenticated(s.handleEvents))
	s.mux.HandleFunc("/", s.handleNotFound)
}

// Handler returns the middleware-wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	return SecurityHeaders(recoverer(LimitBody(s.mux, maxFormBytes)))
}

// Run serves until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if s.cfg.TLSCert != "" && s.cfg.TLSKey != "" {
			err = srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintln(w, "ok")
}

func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.StripPrefix("/static/", fileServer).ServeHTTP(w, r)
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.renderError(w, r, http.StatusNotFound, "That page does not exist.")
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, name string, data any) {
	tpl, ok := s.tpl[name]
	if !ok {
		log.Printf("web: missing template %q", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		log.Printf("web: render %s: %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if message == "" {
		message = http.StatusText(status)
	}
	s.render(w, r, status, "error.html", errorView{
		Status:  status,
		Title:   http.StatusText(status),
		Message: message,
	})
}

func (s *Server) secureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if s.trustedPeer(r) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

func (s *Server) trustedPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	for _, prefix := range s.cfg.TrustedProxies {
		if prefix.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

// clientIP is the address used for rate limiting and the audit log.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustedPeer(r) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if net.ParseIP(first) != nil {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func loadTemplates() (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"version": func() string { return buildinfo.Version },
		"when":    formatTime,
		"ago":     timeAgo,
	}
	layout, err := template.New("layout.html").Funcs(funcs).ParseFS(embedded, "templates/layout.html")
	if err != nil {
		return nil, err
	}
	pages, err := fs.Glob(embedded, "templates/*.html")
	if err != nil {
		return nil, err
	}
	out := map[string]*template.Template{}
	for _, page := range pages {
		name := path.Base(page)
		if name == "layout.html" {
			continue
		}
		clone, err := layout.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := clone.ParseFS(embedded, page); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		out[name] = clone
	}
	return out, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Local().Format("2 Jan 2006, 15:04")
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d d ago", int(d.Hours()/24))
	}
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				log.Printf("web: panic on %s %s: %v", r.Method, r.URL.Path, value)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
