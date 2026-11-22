package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != "ok" {
		t.Fatalf("health: %d %q", rr.Code, rr.Body.String())
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("content type: %q", rr.Header().Get("Content-Type"))
	}
}

func TestStaticAnd404(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/static/app.css", nil))
	if rr.Code != 200 || !strings.Contains(rr.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("css: %d %q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("css cache control = %q; upgrades would keep stale assets", rr.Header().Get("Cache-Control"))
	}
	if strings.Contains(rr.Body.String(), "..") {
		t.Fatal("unexpected body")
	}
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/nope", nil))
	if rr.Code != 404 {
		t.Fatalf("404 = %d", rr.Code)
	}
}

func TestFavicon(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/static/favicon.svg", nil))
	if rr.Code != 200 || !strings.Contains(rr.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("favicon: %d %q", rr.Code, rr.Header().Get("Content-Type"))
	}
	rr = srv.get("/")
	if !strings.Contains(rr.Body.String(), `rel="icon" href="/static/favicon.svg"`) {
		t.Fatal("layout does not link the favicon")
	}
}

func TestStaticDirectoryListingDenied(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/static/", nil))
	if rr.Code != 404 {
		t.Fatalf("directory listing status = %d", rr.Code)
	}
}

func TestSecurityHeadersOnPages(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.get("/nope")
	if !strings.Contains(rr.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing CSP")
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
}
