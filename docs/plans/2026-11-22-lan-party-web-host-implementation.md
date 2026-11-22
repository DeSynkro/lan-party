# LAN Party 1.3 web host implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a host-only web app that runs the existing LAN Party host engine headlessly on a homelab or VPS, with a server-rendered UI, strong defaults, Docker and native packaging, and no relay or join flow.

**Architecture:** A new `server/cmd/lan-party-web` binary imports the existing `internal/app` engine and a new `server/internal/webui` HTTP layer. The UI is Go `html/template` plus a small external CSS/JS pair embedded with `go:embed`. All state lives in `./data` beside the Compose file. Auth is a single admin password with in-memory sessions, CSRF, Origin checks, login rate limiting and optional TLS.

**Tech Stack:** Go 1.27.4, `net/http`, `html/template`, `go:embed`, `golang.org/x/term` for password prompts, existing `internal/app` / `internal/group` / `internal/session` / `internal/secure` / `internal/config` / `internal/applog` / `internal/buildinfo`, Docker Compose, distroless/static.

**Spec:** [2026-11-22-lan-party-web-host.md](2026-11-22-lan-party-web-host.md)

## Global constraints

- The web binary must not import `internal/gtkui`, `internal/relay`, GTK or cgo.
- One live host session at a time, matching the desktop app.
- Direct mode only. Normalize away any relay settings at load.
- Zero-JavaScript core; JavaScript only enhances live updates and copy.
- No external CDN, no remote fonts, no inline scripts.
- Data lives in `./data`; directories `0700`, files `0600`.
- One admin password; no usernames, roles or multi-user accounts.
- The UDP app port is deployment-owned and read-only in the UI.
- The public address is editable in the UI and saved to `settings.json`.
- No per-group IP/port UI.
- No app-level key encryption in 1.3; document disk and backup encryption.
- Linux amd64/arm64 native builds and a multi-arch container image.
- No timers, no installers, no root requirement for the normal path.
- All mutations are POST + redirect; destructive actions get a confirmation page.

## Review focus

- **Auth bypass:** every route except `/login`, `/healthz` and `/static/` must redirect or return 401. Test direct `GET`/`POST` without a cookie.
- **CSRF and Origin:** a valid session without a token, a wrong token, and a foreign Origin must all be rejected on POST.
- **Session fixation and expiry:** login rotates the ID; idle and absolute expiry both destroy the session; logout destroys it server-side.
- **Secret leakage:** logged-out HTML, logs and error pages must never contain invite codes, password hashes, private keys or session IDs.
- **Docker port drift:** the app port shown in the UI must equal the port the container publishes; changing Compose must be the only way to change it.
- **Headless isolation:** the web binary must not write autostart files, and must not attempt a relay connection even if the data directory contains relay settings.
- **Template escaping:** group names, member names, service names, hints and invite names must be escaped in every context.
- **Body and header limits:** oversized form bodies and headers must be rejected without a panic.
- **Bind-mount ownership:** `setup.sh` must produce a `./data` the container user can write, without running the container as root.
- **JS-disabled flow:** every core action must work with JavaScript disabled.

---

## File structure

```text
internal/password/
  password.go
  password_test.go

internal/app/
  app.go                         modify: Options, headless guards, relay normalization
  app_test.go                    modify/add tests

server/cmd/lan-party-web/
  main.go                        CLI dispatch

server/internal/webui/
  app.go                         narrow interface over *app.App
  config.go                      server config, data dir resolution
  credentials.go                 web.json password hash, first-run, passwd
  credentials_test.go
  session.go                     in-memory sessions, cookies
  session_test.go
  ratelimit.go                   login limiter
  ratelimit_test.go
  security.go                    headers, CSRF, origin, body limits
  security_test.go
  server.go                      routes, lifecycle, static assets
  server_test.go
  views.go                       view models
  audit.go                       audit ring + log
  audit_test.go
  handlers_auth.go               login/logout
  handlers_dashboard.go
  handlers_group.go
  handlers_service.go
  handlers_invite.go
  handlers_member.go
  handlers_settings.go
  handlers_guide.go
  events.go                      SSE snapshot
  events_test.go
  templates/
    layout.html
    login.html
    dashboard.html
    group.html
    service_form.html
    invite.html
    member.html
    approval.html
    settings.html
    password.html
    guide.html
    confirm.html
    error.html
  static/
    app.css
    app.js
    fonts/                       optional WOFF2 subsets

server/docker/
  Dockerfile
  .dockerignore
  compose.yaml
  .env.example
  setup.sh
  webctl
  README.md

server/native/
  run.sh
  lan-party-web.service
  web.env.example
  README.md

server/README.md
docs/web-host.md
docs/releases/v1.3.0.md
```

---

## Task 1: Shared password package

**Files:**
- Create: `internal/password/password.go`
- Create: `internal/password/password_test.go`
- Modify: `internal/relay/password.go`

**Interfaces:**
- Produces:
  - `func Generate() (string, error)`
  - `func Hash(password string) (salt, hash string, err error)`
  - `func Verify(password, salt, hash string) bool`
- The relay keeps its existing exported names as wrappers.

- [ ] **Step 1: Write the failing tests**

`internal/password/password_test.go`:

```go
func TestHashVerify(t *testing.T) {
    salt, hash, err := Hash("correct horse battery staple")
    if err != nil { t.Fatal(err) }
    if salt == "" || hash == "" { t.Fatal("empty salt or hash") }
    if !Verify("correct horse battery staple", salt, hash) { t.Fatal("valid password rejected") }
    if Verify("wrong", salt, hash) { t.Fatal("wrong password accepted") }
}

func TestHashUsesFreshSalt(t *testing.T) {
    salt1, hash1, _ := Hash("same")
    salt2, hash2, _ := Hash("same")
    if salt1 == salt2 || hash1 == hash2 { t.Fatal("salt or hash reused") }
}

func TestEmptyPasswordRejected(t *testing.T) {
    if _, _, err := Hash(""); err == nil { t.Fatal("empty password accepted") }
}

func TestGenerateLength(t *testing.T) {
    got, err := Generate()
    if err != nil { t.Fatal(err) }
    if len(got) < 24 { t.Fatalf("generated password too short: %d", len(got)) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/password -v`
Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the package**

Move the PBKDF2 implementation from `internal/relay/password.go` into
`internal/password/password.go`. Keep the same parameters:

```go
const (
    iterations = 600000
    keyLength  = 32
    saltLength = 16
)
```

`Generate` returns a 24-byte random value encoded with
`base64.RawURLEncoding`. `Hash` returns base64 standard salt and key. `Verify`
uses `subtle.ConstantTimeCompare`.

In `internal/relay/password.go`, replace the bodies with wrappers:

```go
func GeneratePassword() (string, error) { return password.Generate() }
func HashPassword(p string) (string, string, error) { return password.Hash(p) }
func VerifyPassword(p, salt, hash string) bool { return password.Verify(p, salt, hash) }
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/password ./internal/relay -v`
Expected: PASS, including the existing relay password tests.

- [ ] **Step 5: Commit**

```bash
git add internal/password internal/relay/password.go
git commit -m "Extracted password hashing for reuse"
```

---

## Task 2: Headless app options

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/headless_test.go`

**Interfaces:**
- Produces:
  - `type Options struct { DisableAutostart bool; DisableRelay bool }`
  - `func NewWithOptions(paths config.Paths, kr secure.SecretStore, opts Options) (*App, error)`
- `New` and `NewWithStore` delegate to `NewWithOptions` with zero options.

- [ ] **Step 1: Write the failing tests**

`internal/app/headless_test.go`:

```go
func TestHeadlessDoesNotWriteAutostart(t *testing.T) {
    home := t.TempDir()
    t.Setenv("HOME", home)
    t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
    paths := config.Paths{ConfigDir: filepath.Join(home, "cfg"), DataDir: filepath.Join(home, "data")}
    a, err := NewWithOptions(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")), Options{DisableAutostart: true})
    if err != nil { t.Fatal(err) }
    defer a.Close()
    entries, _ := os.ReadDir(filepath.Join(home, ".config", "autostart"))
    if len(entries) != 0 { t.Fatalf("autostart entries written: %v", entries) }
}

func TestDisableRelayForcesDirect(t *testing.T) {
    paths := config.Paths{ConfigDir: t.TempDir(), DataDir: t.TempDir()}
    s := config.DefaultSettings()
    s.UseRelay = true
    s.RelayHost = "relay.example.net"
    s.RelayPort = 7443
    s.RelayID = strings.Repeat("a", 64)
    if err := config.SaveSettings(paths, s); err != nil { t.Fatal(err) }
    a, err := NewWithOptions(paths, secure.NewFileStore(filepath.Join(paths.DataDir, "keys")), Options{DisableAutostart: true, DisableRelay: true})
    if err != nil { t.Fatal(err) }
    defer a.Close()
    if a.Settings().UseRelay { t.Fatal("relay still enabled") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app -run 'TestHeadless|TestDisableRelay' -v`
Expected: FAIL because `NewWithOptions` and `Options` do not exist.

- [ ] **Step 3: Implement**

- Add `Options` and `NewWithOptions`.
- Store `disableAutostart` and `disableRelay` on `App`.
- Guard both `autostart.Set` calls.
- In `NewWithOptions`, if `DisableRelay`, set `settings.UseRelay = false`, clear
  `RelayHost`, `RelayPort`, `RelayID`, and for each host group set
  `OverrideSettings = false`, `UseRelay = false`, `RelayCustom = false`, and
  clear the relay fields. Log one warning if anything changed.
- Keep the existing public API intact.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/app -run 'TestHeadless|TestDisableRelay' -v`
Expected: PASS.

- [ ] **Step 5: Run the full app tests**

Run: `go test ./internal/app`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app
git commit -m "Added headless app options"
```

---

## Task 3: Server config, paths and CLI skeleton

**Files:**
- Create: `server/cmd/lan-party-web/main.go`
- Create: `server/internal/webui/config.go`
- Create: `server/internal/webui/config_test.go`

**Interfaces:**
- Produces:
  - `type Config struct { DataDir, Listen, TLSCert, TLSKey string; AppPort int; TrustedProxies []netip.Prefix }`
  - `func ResolveDataDir(flagValue string) (string, error)`
  - `func ConfigPaths(dataDir string) config.Paths`
- CLI commands: `setup`, `run`, `passwd`, `show-info`, `healthcheck`, `version`, `help`.

- [ ] **Step 1: Write the failing tests**

```go
func TestResolveDataDirPrecedence(t *testing.T) {
    t.Setenv("LANPARTY_WEB_DATA_DIR", "/env/data")
    if got, _ := ResolveDataDir("/flag/data"); got != "/flag/data" { t.Fatalf("flag lost: %s", got) }
    if got, _ := ResolveDataDir(""); got != "/env/data" { t.Fatalf("env lost: %s", got) }
}

func TestConfigPathsShareDataDir(t *testing.T) {
    p := ConfigPaths("/srv/lan-party")
    if p.ConfigDir != "/srv/lan-party" || p.DataDir != "/srv/lan-party" { t.Fatalf("unexpected paths: %+v", p) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestResolveDataDir|TestConfigPaths' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- `config.go` resolves the data dir from flag, env, then `./data`.
- `main.go` dispatches commands with `flag.NewFlagSet`.
- `version` prints `buildinfo.Version`.
- `healthcheck` performs `GET` on the URL with a 2s timeout and exits non-zero
  on failure.
- `show-info` prints the data dir, listen address, app port and whether TLS is
  configured. It never prints the password.
- `setup` and `run` can be stubs that return "not implemented" for now.

- [ ] **Step 4: Run the tests and build**

Run: `go test ./server/internal/webui -run 'TestResolveDataDir|TestConfigPaths' -v`
Run: `go build ./server/...`
Expected: PASS and build.

- [ ] **Step 5: Commit**

```bash
git add server
git commit -m "Added web host CLI skeleton"
```

---

## Task 4: Credentials file and first-run password

**Files:**
- Create: `server/internal/webui/credentials.go`
- Create: `server/internal/webui/credentials_test.go`
- Modify: `server/cmd/lan-party-web/main.go`

**Interfaces:**
- Produces:
  - `type Credentials struct { Version int; Salt string; Hash string; ChangedAt time.Time }`
  - `func LoadCredentials(path string) (Credentials, error)`
  - `func SaveCredentials(path string, c Credentials) error`
  - `func EnsureCredentials(path string, supplied string) (Credentials, string, error)` — returns the credentials and a one-time generated password, or empty if supplied/existing.
  - `func (c Credentials) Verify(password string) bool`
  - `func SetPassword(path, password string) error`

- [ ] **Step 1: Write the failing tests**

```go
func TestEnsureCredentialsGeneratesAndPersists(t *testing.T) {
    path := filepath.Join(t.TempDir(), "web.json")
    c, generated, err := EnsureCredentials(path, "")
    if err != nil { t.Fatal(err) }
    if generated == "" { t.Fatal("no generated password") }
    if !c.Verify(generated) { t.Fatal("generated password does not verify") }
    info, _ := os.Stat(path)
    if info.Mode().Perm() != 0o600 { t.Fatalf("mode = %o", info.Mode().Perm()) }
    c2, generated2, err := EnsureCredentials(path, "")
    if err != nil { t.Fatal(err) }
    if generated2 != "" { t.Fatal("password regenerated on second run") }
    if !c2.Verify(generated) { t.Fatal("persisted password lost") }
}

func TestSetPasswordReplaces(t *testing.T) {
    path := filepath.Join(t.TempDir(), "web.json")
    if err := SetPassword(path, "a long enough password"); err != nil { t.Fatal(err) }
    c, _ := LoadCredentials(path)
    if !c.Verify("a long enough password") { t.Fatal("new password rejected") }
}

func TestShortPasswordRejected(t *testing.T) {
    if err := SetPassword(filepath.Join(t.TempDir(), "web.json"), "short"); err == nil { t.Fatal("short password accepted") }
}

func TestCorruptCredentialsRejected(t *testing.T) {
    path := filepath.Join(t.TempDir(), "web.json")
    os.WriteFile(path, []byte("{"), 0o600)
    if _, err := LoadCredentials(path); err == nil { t.Fatal("corrupt file accepted") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestEnsureCredentials|TestSetPassword|TestShortPassword|TestCorruptCredentials' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Use `internal/password`.
- Write via temp file + rename, mode `0600`.
- `EnsureCredentials` uses `LANPARTY_WEB_PASSWORD` or the supplied value on
  first run; otherwise generates and returns it once.
- `SetPassword` enforces a 12-character minimum.
- `setup` and `passwd` commands use these functions. `passwd` reads twice with
  `term.ReadPassword` when stdin is a terminal.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestEnsureCredentials|TestSetPassword|TestShortPassword|TestCorruptCredentials' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/credentials.go server/internal/webui/credentials_test.go server/cmd/lan-party-web/main.go
git commit -m "Added web host password storage"
```

---

## Task 5: Session store and cookies

**Files:**
- Create: `server/internal/webui/session.go`
- Create: `server/internal/webui/session_test.go`

**Interfaces:**
- Produces:
  - `type SessionStore struct{ ... }`
  - `func NewSessionStore(idle, absolute time.Duration) *SessionStore`
  - `func (s *SessionStore) Create() (*Session, error)`
  - `func (s *SessionStore) Get(id string) (*Session, bool)`
  - `func (s *SessionStore) Destroy(id string)`
  - `func (s *SessionStore) DestroyAll()`
  - `type Session struct { ID string; CSRF string; Created, LastSeen time.Time }`
  - `func SetSessionCookie(w http.ResponseWriter, id string, secure bool)`
  - `func ClearSessionCookie(w http.ResponseWriter, secure bool)`
  - `func SessionID(r *http.Request) string`

- [ ] **Step 1: Write the failing tests**

```go
func TestSessionLifecycle(t *testing.T) {
    store := NewSessionStore(time.Hour, 24*time.Hour)
    s, err := store.Create()
    if err != nil { t.Fatal(err) }
    if len(s.ID) < 32 || s.CSRF == "" { t.Fatal("weak session") }
    if got, ok := store.Get(s.ID); !ok || got != s { t.Fatal("session not found") }
    store.Destroy(s.ID)
    if _, ok := store.Get(s.ID); ok { t.Fatal("session survived destroy") }
}

func TestSessionIdleExpiry(t *testing.T) {
    store := NewSessionStore(time.Millisecond, time.Hour)
    s, _ := store.Create()
    time.Sleep(2 * time.Millisecond)
    if _, ok := store.Get(s.ID); ok { t.Fatal("idle session survived") }
}

func TestCookieFlags(t *testing.T) {
    rr := httptest.NewRecorder()
    SetSessionCookie(rr, "abc", true)
    c := rr.Result().Cookies()[0]
    if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode { t.Fatalf("cookie flags: %+v", c) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestSession|TestCookie' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Use `crypto/rand` for 32-byte IDs and 32-byte CSRF tokens.
- Store in a `map[string]*Session` guarded by a mutex.
- A background janitor is not required; expire on `Get` and on create. No
  goroutine leak.
- Cookie name `lanparty_web_session`.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestSession|TestCookie' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/session.go server/internal/webui/session_test.go
git commit -m "Added web host sessions"
```

---

## Task 6: Security middleware and login rate limiter

**Files:**
- Create: `server/internal/webui/security.go`
- Create: `server/internal/webui/security_test.go`
- Create: `server/internal/webui/ratelimit.go`
- Create: `server/internal/webui/ratelimit_test.go`

**Interfaces:**
- Produces:
  - `func SecurityHeaders(next http.Handler) http.Handler`
  - `func RequireAuth(store *SessionStore, next http.HandlerFunc) http.HandlerFunc`
  - `func CheckOrigin(r *http.Request, allowedOrigin string) error`
  - `func CheckCSRF(r *http.Request, s *Session) error`
  - `func LimitBody(next http.Handler, limit int64) http.Handler`
  - `type LoginLimiter struct{ ... }`
  - `func NewLoginLimiter() *LoginLimiter`
  - `func (l *LoginLimiter) Allow(ip string, now time.Time) (bool, time.Duration)`
  - `func (l *LoginLimiter) Fail(ip string, now time.Time)`
  - `func (l *LoginLimiter) Success(ip string)`

- [ ] **Step 1: Write the failing tests**

```go
func TestSecurityHeaders(t *testing.T) {
    rr := httptest.NewRecorder()
    SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
    h := rr.Header()
    if h.Get("X-Content-Type-Options") != "nosniff" { t.Fatal("missing nosniff") }
    if !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") { t.Fatal("missing CSP") }
    if h.Get("Cache-Control") != "no-store" { t.Fatal("missing no-store") }
}

func TestCheckOrigin(t *testing.T) {
    r := httptest.NewRequest("POST", "http://lan.local/settings", nil)
    r.Host = "lan.local"
    if err := CheckOrigin(r, ""); err != nil { t.Fatal(err) }
    r.Header.Set("Origin", "http://evil.example")
    if err := CheckOrigin(r, ""); err == nil { t.Fatal("foreign origin accepted") }
}

func TestCheckCSRF(t *testing.T) {
    r := httptest.NewRequest("POST", "/", strings.NewReader("csrf=wrong"))
    r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    s := &Session{CSRF: "right"}
    if err := CheckCSRF(r, s); err == nil { t.Fatal("wrong token accepted") }
}

func TestLoginLimiter(t *testing.T) {
    l := NewLoginLimiter()
    now := time.Now()
    for i := 0; i < 5; i++ { l.Fail("1.2.3.4", now) }
    ok, wait := l.Allow("1.2.3.4", now)
    if ok { t.Fatal("limiter did not block") }
    if wait <= 0 { t.Fatal("no retry delay") }
    l.Success("1.2.3.4")
    if ok, _ := l.Allow("1.2.3.4", now); !ok { t.Fatal("success did not reset") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestSecurity|TestCheck|TestLoginLimiter' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Headers exactly as in the design.
- `RequireAuth` loads the session, touches it, and redirects to `/login` for
  GET or returns 401 for non-GET.
- `CheckOrigin` accepts a missing Origin only for same-origin form posts from
  browsers that omit it? No: require Origin for POST, allow an empty Origin only
  when the request is not a browser form (tests). For the app, require Origin
  and compare scheme+host to `r.Host`, or to the configured allowed origin.
- `CheckCSRF` parses the form with a 64 KiB limit and compares in constant time.
- `LoginLimiter`: token bucket per IP, 5 failures, exponential backoff capped at
  15 minutes, global cap 100 failures/minute.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestSecurity|TestCheck|TestLoginLimiter' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/security.go server/internal/webui/security_test.go server/internal/webui/ratelimit.go server/internal/webui/ratelimit_test.go
git commit -m "Added web host security middleware"
```

---

## Task 7: HTTP server, routes, static assets and health

**Files:**
- Create: `server/internal/webui/server.go`
- Create: `server/internal/webui/server_test.go`
- Create: `server/internal/webui/templates/layout.html`
- Create: `server/internal/webui/templates/error.html`
- Create: `server/internal/webui/static/app.css`
- Create: `server/internal/webui/static/app.js`
- Modify: `server/cmd/lan-party-web/main.go`

**Interfaces:**
- Produces:
  - `type Server struct{ ... }`
  - `func New(a App, cfg Config) (*Server, error)`
  - `func (s *Server) Handler() http.Handler`
  - `func (s *Server) Run(ctx context.Context) error`
- Routes registered in `Handler`.

- [ ] **Step 1: Write the failing tests**

```go
func TestHealthz(t *testing.T) {
    srv := newTestServer(t)
    rr := httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
    if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != "ok" { t.Fatalf("health: %d %q", rr.Code, rr.Body.String()) }
}

func TestStaticAnd404(t *testing.T) {
    srv := newTestServer(t)
    rr := httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/static/app.css", nil))
    if rr.Code != 200 || !strings.Contains(rr.Header().Get("Content-Type"), "text/css") { t.Fatal("css") }
    rr = httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/nope", nil))
    if rr.Code != 404 { t.Fatalf("404 = %d", rr.Code) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestHealthz|TestStaticAnd404' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- `//go:embed templates static` in `server.go`.
- Parse templates with a `template.FuncMap` for time formatting and ports.
- Serve `/static/` with `http.FileServer(http.FS(...))`, cache headers and no
  directory listing.
- `/healthz` returns `ok` with `text/plain`.
- Wrap the mux with `SecurityHeaders`, `LimitBody`, and a request logger.
- `Run` builds an `http.Server` with the design timeouts, starts it, and shuts
  down on `ctx.Done()` with a 10s timeout.
- `main.go`'s `run` command opens the app with headless options, loads
  credentials, creates the server and waits for signals.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestHealthz|TestStaticAnd404' -v`
Run: `go build ./server/...`
Expected: PASS and build.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/server.go server/internal/webui/server_test.go server/internal/webui/templates server/internal/webui/static server/cmd/lan-party-web/main.go
git commit -m "Added web host HTTP server"
```

---

## Task 8: App interface and view models

**Files:**
- Create: `server/internal/webui/app.go`
- Create: `server/internal/webui/views.go`
- Create: `server/internal/webui/app_test.go`

**Interfaces:**
- Produces `type App interface` with exactly the methods used by handlers:

```go
type App interface {
    Groups() []group.Group
    Group(id string) (group.Group, bool)
    Settings() config.Settings
    SaveSettings(config.Settings) error
    SetPublicAddress(string) error
    LiveGroupID() string
    HostPresence() map[string]session.Presence
    HostNote() string
    Approval() *app.Approval
    ResolveApproval(id string, allow bool) bool
    CreateGroup(name, hostName string) (group.Group, error)
    DeleteGroup(id string) error
    AddService(groupID string, s group.Service) error
    UpdateService(groupID string, s group.Service) error
    RemoveService(groupID, serviceID string) error
    CreateInvite(groupID, peerName string) (string, error)
    InviteCode(inviteID string) string
    RemoveInvite(groupID, inviteID string) error
    StartSession(groupID string) error
    StopSession() error
    RemoveMember(groupID, memberID string) error
    GroupEndpoint(groupID string) (string, error)
    GroupAppPort(groupID string) (int, error)
    DeviceName() string
    DataDir() string
    ConfigDir() string
    Close() error
}
```

- Produces view models: `GroupView`, `MemberView`, `InviteView`, `ServiceView`,
  `DashboardView`, `GroupPageView`, `SettingsView`.

- [ ] **Step 1: Write the failing test**

```go
func TestGroupViewHidesFriendAndRelayFields(t *testing.T) {
    g := group.Group{ID: "g1", Name: "Test", Role: group.RoleHost, UseRelay: true, RelayHost: "secret.example"}
    v := NewGroupView(g, nil, false, 0, "")
    if v.UseRelay || v.RelayHost != "" { t.Fatal("relay fields leaked") }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./server/internal/webui -run TestGroupView -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Define the interface.
- Build view models with only host-safe fields.
- Include `EffectiveEndpoint` and `AppPort` from the app methods, with errors
  rendered as empty strings.
- Include `Live`, `Connected`, `Members`, `Invites`, `Services`, `HostNote`.

- [ ] **Step 4: Run the test**

Run: `go test ./server/internal/webui -run TestGroupView -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/app.go server/internal/webui/views.go server/internal/webui/app_test.go
git commit -m "Added web host app interface and views"
```

---

## Task 9: Layout, CSS and login page

**Files:**
- Create: `server/internal/webui/handlers_auth.go`
- Create: `server/internal/webui/templates/login.html`
- Create: `server/internal/webui/templates/layout.html`
- Modify: `server/internal/webui/static/app.css`
- Create: `server/internal/webui/auth_test.go`

**Interfaces:**
- `GET /login`, `POST /login`, `POST /logout`.
- `Server.render(w, name, data)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestLoginRequired(t *testing.T) {
    srv := newTestServer(t)
    rr := httptest.NewRecorder()
    srv.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
    if rr.Code != 303 || rr.Header().Get("Location") != "/login" { t.Fatalf("redirect: %d %q", rr.Code, rr.Header().Get("Location")) }
}

func TestLoginSuccessAndLogout(t *testing.T) {
    srv := newTestServer(t)
    body := url.Values{"password": {"test-password-123"}}
    rr := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/login", strings.NewReader(body.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Origin", "http://example.com")
    req.Host = "example.com"
    srv.Handler().ServeHTTP(rr, req)
    if rr.Code != 303 || rr.Header().Get("Location") != "/" { t.Fatalf("login: %d %q", rr.Code, rr.Header().Get("Location")) }
    cookies := rr.Result().Cookies()
    if len(cookies) == 0 { t.Fatal("no cookie") }
}

func TestLoginFailureIsGeneric(t *testing.T) {
    srv := newTestServer(t)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/login", strings.NewReader("password=wrong"))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Origin", "http://example.com")
    req.Host = "example.com"
    srv.Handler().ServeHTTP(rr, req)
    if rr.Code != 401 { t.Fatalf("status = %d", rr.Code) }
    if strings.Contains(rr.Body.String(), "wrong") { t.Fatal("error leaked detail") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestLogin|TestLogout' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Login form with password only, `autocomplete="current-password"`.
- Check Origin, rate limiter, credentials, rotate session, set cookie.
- Generic error, fixed delay.
- Logout requires auth + CSRF, destroys the session, clears the cookie.
- CSS: palette from `internal/gtkui/style.css`, responsive, system font stack
  until WOFF2 files are added.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestLogin|TestLogout' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_auth.go server/internal/webui/templates/login.html server/internal/webui/templates/layout.html server/internal/webui/static/app.css server/internal/webui/auth_test.go
git commit -m "Added web host login"
```

---

## Task 10: Dashboard

**Files:**
- Create: `server/internal/webui/handlers_dashboard.go`
- Create: `server/internal/webui/templates/dashboard.html`
- Create: `server/internal/webui/dashboard_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestDashboardRendersGroups(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "<script>alert(1)</script>", Role: group.RoleHost})
    rr := srv.get(t, "/")
    if rr.Code != 200 { t.Fatalf("status = %d", rr.Code) }
    if strings.Contains(rr.Body.String(), "<script>alert(1)</script>") { t.Fatal("unescaped group name") }
    if !strings.Contains(rr.Body.String(), "&lt;script&gt;") { t.Fatal("escaped name missing") }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./server/internal/webui -run TestDashboard -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- List host groups, live dot, connected count, member count, start/stop button.
- Pending approval banner from `Approval()`.
- Empty state with a create link.
- No join section.

- [ ] **Step 4: Run the test**

Run: `go test ./server/internal/webui -run TestDashboard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_dashboard.go server/internal/webui/templates/dashboard.html server/internal/webui/dashboard_test.go
git commit -m "Added web host dashboard"
```

---

## Task 11: Group page

**Files:**
- Create: `server/internal/webui/handlers_group.go`
- Create: `server/internal/webui/templates/group.html`
- Create: `server/internal/webui/group_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestGroupPageShowsSessionAndServices(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{
        ID: "g1", Name: "Game night", Role: group.RoleHost,
        Services: []group.Service{{ID: "s1", Name: "Minecraft", Proto: "tcp", Target: "127.0.0.1:25565", Ports: []int{25565}}},
    })
    rr := srv.get(t, "/groups/g1")
    body := rr.Body.String()
    if rr.Code != 200 { t.Fatalf("status = %d", rr.Code) }
    if !strings.Contains(body, "Game night") || !strings.Contains(body, "Minecraft") { t.Fatal("missing content") }
    if !strings.Contains(body, "Session off") { t.Fatal("missing session state") }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./server/internal/webui -run TestGroupPage -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Session panel: status, start/end, endpoint, app port read-only note, connected
  count, host note.
- Services list with add/edit/remove links.
- People list with member details links.
- Invites list with create/remove and code display.
- Delete group link.

- [ ] **Step 4: Run the test**

Run: `go test ./server/internal/webui -run TestGroupPage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_group.go server/internal/webui/templates/group.html server/internal/webui/group_test.go
git commit -m "Added web host group page"
```

---

## Task 12: SSE live state

**Files:**
- Create: `server/internal/webui/events.go`
- Create: `server/internal/webui/events_test.go`
- Modify: `server/internal/webui/static/app.js`

- [ ] **Step 1: Write the failing test**

```go
func TestSnapshotChangesWhenPresenceChanges(t *testing.T) {
    fake := newFakeApp()
    s := NewSnapshot(fake)
    first := s.Hash()
    fake.presence = map[string]session.Presence{"m1": {Live: true}}
    second := NewSnapshot(fake).Hash()
    if first == second { t.Fatal("snapshot did not change") }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./server/internal/webui -run TestSnapshot -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- `Snapshot` with groups, live ID, presence, approval, host note.
- `Hash()` over the JSON encoding.
- `GET /events` requires auth, sets `text/event-stream`, sends an initial
  snapshot, then ticks every second and sends only on change.
- `app.js`: `EventSource`, update `[data-live]` dots, `[data-connected]` text,
  `[data-member]` rows and the approval banner; copy button enhancement.

- [ ] **Step 4: Run the test**

Run: `go test ./server/internal/webui -run TestSnapshot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/events.go server/internal/webui/events_test.go server/internal/webui/static/app.js
git commit -m "Added web host live state"
```

---

## Task 13: Group actions

**Files:**
- Modify: `server/internal/webui/handlers_group.go`
- Create: `server/internal/webui/templates/confirm.html`
- Create: `server/internal/webui/group_actions_test.go`

Routes: `GET/POST /groups/new`, `POST /groups`, `GET/POST /groups/{id}/delete`,
`POST /groups/{id}/start`, `POST /groups/{id}/stop`.

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateGroup(t *testing.T) {
    srv := newTestServer(t)
    rr := srv.post(t, "/groups", url.Values{"name": {"Friday"}, "hostName": {"Server"}})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if len(srv.fake.groups) != 1 || srv.fake.groups[0].Name != "Friday" { t.Fatal("group not created") }
}

func TestDeleteGroupRequiresConfirmation(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
    rr := srv.get(t, "/groups/g1/delete")
    if rr.Code != 200 || !strings.Contains(rr.Body.String(), "Delete") { t.Fatal("confirmation missing") }
    rr = srv.post(t, "/groups/g1/delete", url.Values{})
    if rr.Code != 303 { t.Fatalf("delete status = %d", rr.Code) }
    if len(srv.fake.groups) != 0 { t.Fatal("group not deleted") }
}

func TestStartAndStop(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
    if rr := srv.post(t, "/groups/g1/start", url.Values{}); rr.Code != 303 { t.Fatalf("start = %d", rr.Code) }
    if rr := srv.post(t, "/groups/g1/stop", url.Values{}); rr.Code != 303 { t.Fatalf("stop = %d", rr.Code) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestCreateGroup|TestDeleteGroup|TestStartAndStop' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- All POSTs require auth, CSRF and Origin.
- Create/delete/start/stop call the app interface.
- Errors re-render or redirect with a flash message; no panic.
- Confirmation page for delete.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestCreateGroup|TestDeleteGroup|TestStartAndStop' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_group.go server/internal/webui/templates/confirm.html server/internal/webui/group_actions_test.go
git commit -m "Added web host group actions"
```

---

## Task 14: Service actions

**Files:**
- Create: `server/internal/webui/handlers_service.go`
- Create: `server/internal/webui/templates/service_form.html`
- Create: `server/internal/webui/service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestAddService(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
    rr := srv.post(t, "/groups/g1/services", url.Values{
        "name": {"Minecraft"}, "proto": {"tcp"}, "target": {"127.0.0.1:25565"},
        "ports": {"25565"}, "hint": {"Java edition"},
    })
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if len(srv.fake.groups[0].Services) != 1 { t.Fatal("service not added") }
}

func TestServiceValidation(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
    rr := srv.post(t, "/groups/g1/services", url.Values{"name": {"Bad"}, "proto": {"icmp"}, "target": {"127.0.0.1"}, "ports": {"99999"}})
    if rr.Code != 400 { t.Fatalf("status = %d", rr.Code) }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestAddService|TestServiceValidation' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Form with preset picker (reuse `internal/preset`), name, protocol, target,
  ports, optional friend port, hint.
- Parse with `group.ParsePorts`; validate protocol, target, friend port rules.
- Add/edit/remove; live session gets the update through the app.
- Confirmation for remove.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestAddService|TestServiceValidation' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_service.go server/internal/webui/templates/service_form.html server/internal/webui/service_test.go
git commit -m "Added web host service actions"
```

---

## Task 15: Invite actions

**Files:**
- Create: `server/internal/webui/handlers_invite.go`
- Create: `server/internal/webui/templates/invite.html`
- Create: `server/internal/webui/invite_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateInviteShowsCodeOnce(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
    rr := srv.post(t, "/groups/g1/invites", url.Values{"peerName": {"Ash"}})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if srv.fake.lastInvite == "" { t.Fatal("no invite code") }
    rr = srv.get(t, "/groups/g1")
    if !strings.Contains(rr.Body.String(), srv.fake.lastInvite) { t.Fatal("code not shown") }
}

func TestInviteRequiresSession(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost})
    rr := srv.post(t, "/groups/g1/invites", url.Values{"peerName": {"Ash"}})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if !srv.fake.started { t.Fatal("session not started") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestCreateInvite|TestInviteRequiresSession' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Create/remove invite; display code once via `InviteCode`.
- If the session is off, show a warning and "Start session and invite".
- If another group is live, show the app's error instead of switching sessions.
- Confirmation for cancel.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestCreateInvite|TestInviteRequiresSession' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_invite.go server/internal/webui/templates/invite.html server/internal/webui/invite_test.go
git commit -m "Added web host invite actions"
```

---

## Task 16: Approvals and members

**Files:**
- Create: `server/internal/webui/handlers_member.go`
- Create: `server/internal/webui/templates/member.html`
- Create: `server/internal/webui/templates/approval.html`
- Create: `server/internal/webui/member_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestApprovalAllowAndDeny(t *testing.T) {
    srv := newTestServer(t)
    srv.fake.approval = &app.Approval{ID: "a1", PeerName: "Ash", DeviceName: "Laptop", Address: "10.0.0.5", Fingerprint: "ABCD"}
    rr := srv.post(t, "/approvals/a1", url.Values{"allow": {"yes"}})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if !srv.fake.resolved || !srv.fake.allowed { t.Fatal("approval not resolved") }
}

func TestRemoveMember(t *testing.T) {
    srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost, Members: []group.Member{{ID: "m1", Name: "Ash", State: group.MemberApproved}}})
    rr := srv.post(t, "/groups/g1/members/m1/remove", url.Values{})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if len(srv.fake.groups[0].Members) != 0 { t.Fatal("member not removed") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestApproval|TestRemoveMember' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Approval banner on authenticated pages; allow/deny POST.
- Member details with device, address, fingerprint, first/last seen, state.
- Remove with confirmation.
- No relay reconnect fields.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestApproval|TestRemoveMember' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_member.go server/internal/webui/templates/member.html server/internal/webui/templates/approval.html server/internal/webui/member_test.go
git commit -m "Added web host approvals and members"
```

---

## Task 17: Settings and password change

**Files:**
- Create: `server/internal/webui/handlers_settings.go`
- Create: `server/internal/webui/templates/settings.html`
- Create: `server/internal/webui/templates/password.html`
- Create: `server/internal/webui/settings_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestSavePublicAddress(t *testing.T) {
    srv := newTestServer(t)
    rr := srv.post(t, "/settings", url.Values{"publicAddress": {"192.168.1.20"}, "approveNewDevices": {"on"}, "forgetDays": {"5"}})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if srv.fake.settings.PublicAddress != "192.168.1.20" { t.Fatal("address not saved") }
}

func TestAppPortIsReadOnly(t *testing.T) {
    srv := newTestServer(t)
    rr := srv.get(t, "/settings")
    if !strings.Contains(rr.Body.String(), "managed by this deployment") { t.Fatal("missing deployment note") }
    if strings.Contains(rr.Body.String(), `name="appPort"`) { t.Fatal("app port is editable") }
}

func TestPasswordChangeInvalidatesSessions(t *testing.T) {
    srv := newTestServer(t)
    rr := srv.post(t, "/settings/password", url.Values{"current": {"test-password-123"}, "new": {"a new long password"}, "confirm": {"a new long password"}})
    if rr.Code != 303 { t.Fatalf("status = %d", rr.Code) }
    if !srv.sessions.Empty() { t.Fatal("sessions survived password change") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestSavePublicAddress|TestAppPort|TestPasswordChange' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Public address field + save; detected LAN addresses as suggestions.
- App port read-only with the deployment note.
- Approve new devices and forget days.
- Password change form: current, new, confirm; min 12; destroys all sessions;
  redirect to login.
- About: version, data dir, log path.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestSavePublicAddress|TestAppPort|TestPasswordChange' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_settings.go server/internal/webui/templates/settings.html server/internal/webui/templates/password.html server/internal/webui/settings_test.go
git commit -m "Added web host settings"
```

---

## Task 18: Guide, about and audit log

**Files:**
- Create: `server/internal/webui/handlers_guide.go`
- Create: `server/internal/webui/templates/guide.html`
- Create: `server/internal/webui/audit.go`
- Create: `server/internal/webui/audit_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestAuditRingIsBoundedAndRedacts(t *testing.T) {
    a := NewAuditLog(2)
    a.Record("login", "1.2.3.4", "ok")
    a.Record("invite", "1.2.3.4", "created")
    a.Record("settings", "1.2.3.4", "saved")
    if len(a.Recent()) != 2 { t.Fatal("ring not bounded") }
    if strings.Contains(a.Recent()[0].Detail, "invite-code") { t.Fatal("secret logged") }
}

func TestGuideRenders(t *testing.T) {
    srv := newTestServer(t)
    rr := srv.get(t, "/guide")
    if rr.Code != 200 || !strings.Contains(rr.Body.String(), "Forward UDP") { t.Fatal("guide missing") }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/internal/webui -run 'TestAudit|TestGuide' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Audit ring with max 100 entries, log lines to the app logger.
- Record auth, settings, session, group, invite, approval and member events.
- Guide page with the server-specific steps.
- About section on settings.

- [ ] **Step 4: Run the tests**

Run: `go test ./server/internal/webui -run 'TestAudit|TestGuide' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/webui/handlers_guide.go server/internal/webui/templates/guide.html server/internal/webui/audit.go server/internal/webui/audit_test.go
git commit -m "Added web host guide and audit log"
```

---

## Task 19: Docker packaging

**Files:**
- Create: `server/docker/Dockerfile`
- Create: `server/docker/.dockerignore`
- Create: `server/docker/compose.yaml`
- Create: `server/docker/.env.example`
- Create: `server/docker/setup.sh`
- Create: `server/docker/webctl`
- Create: `server/docker/README.md`

- [ ] **Step 1: Write the Dockerfile**

- Stage 1: `golang:1.27` (or the pinned toolchain) copies `go.mod`, `go.sum`,
  `internal/`, `server/`, builds
  `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X ...buildinfo.Version=$VERSION" -o /out/lan-party-web ./server/cmd/lan-party-web`.
- Stage 2: `gcr.io/distroless/static-debian12:nonroot`.
- Copy the binary to `/lan-party-web`.
- `USER nonroot`, `ENTRYPOINT ["/lan-party-web"]`, `CMD ["run", "--data-dir", "/data", "--listen", "0.0.0.0:8080"]`.

- [ ] **Step 2: Write Compose and scripts**

- Compose as in the design, with `PUID`/`PGID`, ports, `/data`, read-only root,
  tmpfs, cap drop, no-new-privileges, limits, healthcheck, restart policy.
- `setup.sh` creates `./data` `0700`, writes `.env` with `id -u`/`id -g`,
  preserves existing values, prints the first-run instructions.
- `webctl` wraps `docker compose exec lan-party-web lan-party-web "$@"`.

- [ ] **Step 3: Verify**

Run: `docker build -f server/docker/Dockerfile -t lan-party-web:dev .`
Run: `cd server/docker && ./setup.sh --check` (a dry-run mode that only checks
and prints)
Run: `docker compose config`
Expected: build succeeds, config valid.

- [ ] **Step 4: Smoke test**

Run: `cd server/docker && docker compose up -d`
Run: `curl -fsS http://127.0.0.1:8080/healthz`
Expected: `ok`. Then `docker compose down`.

- [ ] **Step 5: Commit**

```bash
git add server/docker
git commit -m "Added web host Docker packaging"
```

---

## Task 20: Native packaging

**Files:**
- Create: `server/native/run.sh`
- Create: `server/native/lan-party-web.service`
- Create: `server/native/web.env.example`
- Create: `server/native/README.md`
- Create: `build/web.sh`

- [ ] **Step 1: Write `run.sh`**

- Resolve `./data` beside the script, create it `0700`.
- Read `web.env` if present.
- On first run, prompt for listen address, public address, app port and
  password (or generate one).
- Show a small menu: start, show info, set public address, set app port,
  change password, tail log, quit.
- Start the binary in the foreground; Ctrl+C stops it.

- [ ] **Step 2: Write the systemd unit**

- Dedicated user, `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome=true`,
  `ReadWritePaths` for the data dir, `Restart=on-failure`.
- No timers.

- [ ] **Step 3: Write `build/web.sh`**

- Build Linux amd64 and arm64 static binaries.
- Create `lan-party-web_<version>_linux_<arch>.zip` with the binary, `README.md`,
  `docker/`, `native/`, `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`.

- [ ] **Step 4: Verify**

Run: `bash -n server/native/run.sh && bash -n server/docker/setup.sh`
Run: `VERSION=dev ./build/web.sh`
Run: `unzip -l dist/lan-party-web_dev_linux_amd64.zip`
Expected: scripts parse, zip contains the expected files.

- [ ] **Step 5: Commit**

```bash
git add server/native build/web.sh
git commit -m "Added web host native packaging"
```

---

## Task 21: CI, release and docs

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Create: `docs/releases/v1.3.0.md`
- Create: `docs/web-host.md`
- Modify: `docs/security.md`
- Modify: `docs/development.md`

- [ ] **Step 1: Makefile**

- `make web` builds `./server/cmd/lan-party-web`.
- `make web-test` runs `go test ./server/...`.
- `make web-docker` builds the image.
- Include `./server/...` in `make test` automatically via `go test ./...`
  (verify the existing target).

- [ ] **Step 2: CI**

- Add a `web` job that builds the binary, runs the server tests and the Docker
  build.
- Add tag steps for the web zips and the multi-arch GHCR image
  (`ghcr.io/desynkro/lan-party-web:<version>` and `:1.3`).
- Add a Compose smoke test: start, `/healthz`, stop.
- Add a native `run.sh` smoke test with a temp data dir.

- [ ] **Step 3: Docs**

- `server/README.md`, `server/docker/README.md`, `server/native/README.md`.
- `docs/web-host.md`: homelab setup, router forward, first group, invite,
  reverse proxy, backups, troubleshooting.
- `docs/security.md`: web UI auth, sessions, storage, exposure.
- `docs/development.md`: build/test/release commands.
- `README.md`: a short 1.3 section.
- `CHANGELOG.md` and `docs/releases/v1.3.0.md`.

- [ ] **Step 4: Verify**

Run: `make test`
Run: `make vet`
Run: `make test-race`
Run: `make vuln`
Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add Makefile .github/workflows/ci.yml README.md CHANGELOG.md docs server
git commit -m "Prepared the web host release"
```

---

## Task 22: Hardening and end-to-end verification

**Files:**
- Modify: tests as needed.
- No production code unless a check finds a bug.

- [ ] **Step 1: Automated checks**

Run:

```sh
go test ./...
go vet ./...
go test -race ./...
make vuln
```

- [ ] **Step 2: Security checklist**

- Unauthenticated GET and POST to every route.
- CSRF missing/wrong.
- Foreign and missing Origin.
- Login rate limit and `Retry-After`.
- Cookie flags with and without TLS.
- Session idle and absolute expiry.
- Logout destroys the session.
- Password change destroys all sessions.
- Oversized body and header.
- XSS payloads in group, member, service, invite and hint fields.
- Path traversal on `/static/`.
- Open redirect on `next` parameters (there should be none).
- No invite codes, hashes, keys or session IDs in logs or logged-out HTML.
- CSP has no inline script.
- `Cache-Control: no-store` on authenticated pages.

- [ ] **Step 3: Deployment checks**

- Docker Compose up, first-run password in logs, login, create group, add
  service, invite, start session.
- Native `run.sh` with a temp data dir, same flow.
- Restart both and confirm data persists.
- JavaScript disabled: create group, add service, invite, start/stop, settings,
  logout.
- Reverse proxy with TLS and `--trusted-proxies`; confirm `Secure` cookies and
  correct client IP in the audit log.
- Real friend joins with the native app and reaches a loopback test service.

- [ ] **Step 4: Commit fixes**

Commit any fixes found by the checklist with a focused message.

---

## Compaction recovery

If this session is compacted, resume here:

1. Read this plan and the design doc.
2. Check the current branch and `git status`.
3. Find the first unchecked task above.
4. Run the task's tests before changing code.
5. Continue in order. Do not skip the auth, CSRF and headless-isolation tests.

The project is on branch `feat/web-host`. The user handles pushes and tags.
Do not push or create release tags without being asked.
