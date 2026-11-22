# LAN Party 1.3 web host

Status: implementation complete on branch `feat/web-host`; Docker image build
and a real friend join remain to be verified outside this session.

This document records the direction and the decisions for the 1.3 web host so a
later session does not have to reconstruct them from chat. The task-by-task
implementation plan is in
[2026-11-22-lan-party-web-host-implementation.md](2026-11-22-lan-party-web-host-implementation.md).

## Goal

Run the host side of LAN Party on a homelab or VPS and control it from a
browser. The web app uses the same Go engine as the desktop host, without GTK.
Friends keep using the native desktop app and paste an invite exactly as they
do today. The game still connects to `127.0.0.1:<port>` on the friend's PC.

This is a host-only, direct-mode product. There is no relay, no join flow, no
tray and no desktop autostart. It keeps one live host session at a time,
matching the desktop app in 1.3. Concurrent multi-group hosting is deferred.

## Why this shape

- A server should not need a display server, GTK, a tray or a desktop session.
- A browser is the lowest-friction way to administer a headless host from a
  laptop, phone or another machine on the LAN.
- Reusing `internal/app` avoids a second implementation of groups, keys,
  invites, approvals and the tunnel.
- Server-rendered HTML keeps the stack Go-only, the container small and the
  page weight low. The app has a handful of screens; a SPA is not needed.
- The relay already proves the deployment pattern: a self-hosted binary, a
  Docker image, a no-root console runner, a local data directory and strict
  file modes.

## Scope

- Host-only web UI: groups, services, invites, members, approvals, session
  start/stop and settings.
- One live host session, as in the desktop app.
- Direct mode only.
- Zero-JavaScript core. JavaScript is an optional enhancement for live updates
  and copy.
- One static Linux binary, one small container image, and a Compose file that
  keeps all state in `./data` beside it.
- A no-root native console runner and an optional systemd unit. No timers, no
  installer and no root requirement for the normal path.
- Efficient operation: no polling loops, no external CDN, no network font
  requests and no idle chatter added to the tunnel.

## Non-goals

- Concurrent multi-group sessions.
- Relay support in the web host. Relay settings, passwords, Relay IDs and
  reconnect-token UI are absent.
- Joining a friend group from the web UI.
- Multiple admin accounts, roles or per-user permissions.
- Built-in ACME / Let's Encrypt. Use a reverse proxy or supplied certificate
  files.
- App-level encryption of host keys. Keys are owner-only files; disk and
  backup encryption are documented instead.
- Windows-native server packaging. Linux amd64/arm64 and Docker first.
- Tray, start on login, close-to-tray, start minimised, desktop notifications.
- Automatic updates.
- Changing the UDP app port or per-group IP/port from the web UI.

## Decisions

| Decision | Choice |
| --- | --- |
| Product shape | Host-only web app; friends use the native app |
| Session model | One live host session, same as the desktop app |
| Relay | Not included |
| UI technology | Server-rendered Go templates, responsive CSS, no framework |
| JavaScript | Zero for core flows; a small SSE/copy enhancement |
| Public address | Editable in the web UI, saved to `settings.json` |
| UDP app port | Deployment-owned; read-only in the UI; set by Compose/env or `run.sh` |
| Per-group IP/port | Not exposed in the server UI |
| Data location | `./data/` beside `docker-compose.yml`; `--data-dir` override |
| Key storage | Owner-only files (`0700` dir, `0600` files); no passphrase layer |
| Admin auth | One password, PBKDF2-SHA256, sessions, CSRF, rate limiting |
| TLS | Optional cert/key flags, or a reverse proxy; plain HTTP on LAN with a warning |
| Container user | Non-root `PUID`/`PGID`; read-only root; dropped capabilities |
| Native packaging | Static binary, `run.sh`, optional systemd unit; no timers |
| Default auto-approve | On, matching the desktop default; configurable |
| Default forget window | 5 days, configurable; blank means never |

## Architecture

```text
server/
  cmd/lan-party-web/          CLI entry point
  internal/webui/             HTTP server, auth, handlers, templates, assets
    app.go                    narrow interface over *app.App
    server.go                 routes, middleware, lifecycle
    auth.go                   login, sessions, cookies, CSRF
    credentials.go            password hash file, first-run, passwd
    ratelimit.go              login limiter
    security.go               headers, body limits, origin checks
    handlers_*.go             page and action handlers
    events.go                 SSE state snapshots
    views.go                  view models
    audit.go                  in-memory audit ring + log lines
    templates/                html/template files
    static/                   CSS, tiny JS, self-hosted fonts
  docker/                     Dockerfile, compose.yaml, setup.sh, webctl, docs
  native/                     run.sh, systemd unit, env example, docs
  README.md
```

### Reuse boundaries

- `internal/app` owns groups, keys, invites, approvals and sessions. The web
  binary must not duplicate that logic.
- `internal/group`, `internal/session`, `internal/invite`, `internal/secure`,
  `internal/config`, `internal/preset`, `internal/applog` and
  `internal/buildinfo` are reused directly.
- `internal/gtkui` is **not** imported. The web binary must not link GTK or
  cgo.
- The relay package is not imported by the web host.
- Password hashing moves to a new shared `internal/password` package so the web
  host does not import `internal/relay` just for PBKDF2. The relay keeps thin
  wrappers for compatibility.

### Headless seam

`internal/app.New` currently calls `autostart.Set`, and `saveSettingsLocked`
calls it again. A headless server must not write desktop autostart entries.

```go
type Options struct {
    DisableAutostart bool
    DisableRelay     bool
}

func New(paths config.Paths) (*App, error)
func NewWithStore(paths config.Paths, kr secure.SecretStore) (*App, error)
func NewWithOptions(paths config.Paths, kr secure.SecretStore, opts Options) (*App, error)
```

- `DisableAutostart` guards both `autostart.Set` calls.
- `DisableRelay` forces the direct connection path. If a copied data directory
  contains relay settings or group relay overrides, the app normalizes them to
  direct mode at load and logs a warning. The web host never presents relay
  fields.

### Request and state flow

```text
browser
  -> net/http (TLS optional)
  -> security middleware: headers, auth, CSRF, origin, body limit
  -> handler
  -> webui.App interface
  -> *app.App
  -> session.Host / tunnel
```

- Pages are rendered from view models, not raw `group.Group` structs, so the
  templates do not accidentally expose fields the server should hide.
- Actions are POST + redirect. Destructive actions get a separate confirmation
  page.
- `GET /events` is an SSE stream. A one-second server ticker builds a compact
  snapshot, hashes it, and sends only when the hash changes. The tiny client
  script updates text and classes. No page polling.

### Test seam

Define a narrow `webui.App` interface with the methods the UI uses. The real
`*app.App` satisfies it. Handler tests use a fake implementation. A small
number of integration tests use the real app with a temp data directory and a
loopback tunnel, following the existing `internal/app` integration tests.

## Data layout

Native default: `./data` beside `run.sh`. Docker: `/data` mounted from
`./data` beside `docker-compose.yml`.

```text
data/
  settings.json         0600
  groups.json           0600
  web.json              0600  admin password salt/hash and metadata
  keys/                 0700  Ed25519 private keys, one file per group, 0600
  lan-party.log         0600  rotating application log
  tls/                  0700  optional cert.pem / key.pem, key 0600
```

- `config.Paths{ConfigDir: dataDir, DataDir: dataDir}` keeps the engine's
  existing layout in one folder.
- The app creates the data directory with `0700`; `applog.Open` enforces it.
- The secure store creates `keys/` with `0700` and each key with `0600`, and
  refuses to load a key readable by another user.
- No global paths and no `$HOME` dependency in Docker.
- `--data-dir` and `LANPARTY_WEB_DATA_DIR` override the default.

## Security model

### Threat model

The web UI controls who may reach LAN services, so it is a high-value surface.

In scope:

- Another local user on the server.
- A curious or malicious device on the LAN.
- A port-forwarded web UI reachable from the internet.
- Accidental exposure through a reverse proxy.
- A stolen disk or backup.
- A compromised container with limited privileges.

Out of scope:

- An attacker with root on the host.
- Physical access with an unlocked session.
- A compromised browser or keylogger on the admin's machine.
- Nation-state supply-chain attacks.

### Authentication

- One admin password. No username field, so there is no account enumeration.
- PBKDF2-SHA256, 600,000 iterations, 32-byte key, 16-byte random salt,
  constant-time comparison. Reuse the relay's parameters through
  `internal/password`.
- First run: if `web.json` is missing and no password is supplied, generate a
  24-byte random password, print it once to stdout and the log, and store only
  the salted hash.
- `setup` and `passwd` commands set or change the password. `passwd` prompts
  twice with no echo. Minimum 12 characters for a user-supplied password.
- Password changes invalidate every session and require a fresh login.
- Login failures return a generic message and a fixed delay.
- Login rate limiting: per-IP token bucket plus a global cap, exponential
  backoff, `429` with `Retry-After`, and audit log entries. Behind a trusted
  proxy, use the forwarded client IP only from configured proxy CIDRs.

### Sessions and CSRF

- 256-bit random session IDs, stored in memory only. A restart logs everyone
  out; that is acceptable and avoids a session-signing secret on disk.
- Cookie: `lanparty_web_session`, `HttpOnly`, `SameSite=Strict`, `Path=/`,
  `Secure` when the request is HTTPS, no `Domain`.
- Idle timeout 12 hours, absolute timeout 7 days. Touch on authenticated
  requests. Rotate the ID on login.
- CSRF: a per-session token in a hidden form field for every authenticated
  POST. Reject missing or mismatched tokens.
- All POSTs also require an `Origin` header matching the request host (or a
  configured external origin). Missing or foreign origins are rejected.
  `SameSite=Strict` is defence in depth, not the only control.
- Logout destroys the session server-side.

### Transport

- `--listen` is user-chosen. Docker defaults to `0.0.0.0:8080` inside the
  container; Compose controls the published host interface and port.
- Plain HTTP is allowed for LAN use, with a visible banner and a log warning
  when the bind is not loopback and TLS is off.
- `--tls-cert` and `--tls-key` enable HTTPS directly. TLS 1.2 minimum, strong
  cipher suites, HSTS when enabled.
- The documented public path is a reverse proxy (Caddy or nginx) terminating
  TLS and forwarding to `127.0.0.1:8080`.
- `--trusted-proxies` accepts a comma-separated CIDR list. Only those peers may
  supply `X-Forwarded-For` / `X-Forwarded-Proto`. Without it, forwarded headers
  are ignored.
- No HTTP-to-HTTPS redirect is attempted unless the operator configures it; a
  reverse proxy should own that.

### Headers and request limits

- `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`
- `Permissions-Policy: camera=(), microphone=(), geolocation=()`
- `Cache-Control: no-store` on authenticated pages and anything containing an
  invite code.
- `Strict-Transport-Security` only when TLS is active.
- `http.Server` timeouts: `ReadHeaderTimeout` 5s, `ReadTimeout` 15s,
  `WriteTimeout` 30s, `IdleTimeout` 60s, `MaxHeaderBytes` 1 MiB.
- `http.MaxBytesReader` on every form POST (64 KiB).
- No directory listing, no source maps, no pprof, no debug endpoints.
- Static assets are embedded; no user-controlled file paths are served.

### Rate limiting and audit

- Login limiter as above.
- Audit log to `lan-party.log` and a bounded in-memory ring for a recent
  activity panel: login success/failure, logout, password change, settings
  change, session start/stop, group create/delete, invite create/remove,
  approval allow/deny, member remove.
- Never log passwords, invite codes, private keys or session IDs.
- Log lines include the effective client IP, action and outcome.

### Key storage

- Host keys remain owner-only files, exactly as the desktop app and relay.
- No private key is ever rendered, downloaded or logged.
- The web UI does not expose a profile export or key export.
- Document full-disk encryption and encrypted backups as the controls for a
  stolen disk or backup.
- The container runs as a dedicated non-root UID/GID. A read-only root
  filesystem and dropped capabilities limit what a compromise can do.
- No relay secrets exist in this binary.

### Container hardening

- Multi-stage static build, `CGO_ENABLED=0`.
- Distroless/static or scratch base, non-root user.
- `read_only: true`, `tmpfs: /tmp`, `cap_drop: [ALL]`,
  `security_opt: [no-new-privileges:true]`, `pids_limit`, memory and CPU
  limits.
- Only `/data` is writable.
- Healthcheck uses the binary's own `healthcheck` subcommand, not a shell.
- `restart: unless-stopped`.

### Deliberately absent

- No relay settings, password, Relay ID or reconnect-token UI.
- No join flow.
- No per-group connection override UI.
- No app-port editor.
- No multi-user accounts.
- No profile import/export.
- No remote shell, file browser or log download.

## Web UI

### Screens

1. **Login** — password only, no username, generic errors, rate-limit message.
2. **Dashboard** — groups list with role, member count, live dot, connected
   count, start/stop action, pending approval banner, link to create a group.
3. **Group** — session panel (status, start/end, effective public endpoint and
   app port, connected count, host note), published services, people, invites,
   delete group.
4. **Service form** — preset picker, name, protocol, game server target, game
   ports, friend port, hint, live route preview, validation.
5. **Invite form** — friend name, warning and "start session and invite" when
   off, the code once, expiry, copy enhancement.
6. **Member details** — device name, address, fingerprint, first/last seen,
   approval state, remove. Relay reconnect fields are absent.
7. **Approval** — friend, device, address, key fingerprint, allow/deny. Shown
   as a banner on every authenticated page until resolved.
8. **Settings** — public address with save and detected LAN suggestions, app
   port read-only with the deployment note, approve new devices, forget
   inactive friends, change password, about (version, data dir, log path),
   recent activity.
9. **Guide** — condensed hosting guide for the server: forward the UDP port,
   set the public address, create a group and service, invite a friend.
10. **Confirmation pages** — delete group, end session, remove service, remove
    member, cancel invite, change public address with open invites.
11. **Error pages** — 400, 403, 404, 405, 429, 500 with a safe message.

### Zero-JS strategy

- Navigation is ordinary links.
- Every mutation is a `<form method="post">` with a CSRF field and a
  POST/redirect/GET response.
- Destructive actions use a separate confirmation page with its own POST form.
- Collapsible sections use `<details>` / `<summary>`.
- Tabs are separate URLs.
- Validation is HTML5 attributes plus server-side checks; errors re-render the
  form with values preserved.
- The invite code is a read-only `<textarea>`; manual copy always works.
- A "Refresh" link is present on live screens. With JavaScript disabled, the
  page is still fully usable.

### Minimal JavaScript

One external `app.js`, no inline script, for:

1. `EventSource('/events')` to receive a compact snapshot and update status
   dots, connected counts, member rows, approval banner and invite expiry
   text. On error, retry with a small backoff. If `EventSource` is missing, do
   nothing and leave the Refresh link.
2. A copy button using `navigator.clipboard.writeText`, with a visible
   "Copied" state and a fallback that selects the textarea.

Target: under 80 lines, no dependencies. It is an enhancement, never a
requirement.

### Visual language

- Reuse the palette, spacing, radii and typography from
  `internal/gtkui/style.css`: ink `#08040F`, deep `#120A2E`, surface
  `#150C2E`, raised `#1A1035`, hairline `#2C1A55`, pale `#FDF4FF`,
  secondary `#C9B8E8`, muted `#907FAF`, magenta `#FF3DBE`, violet `#8B5CF6`,
  cyan `#22D3EE`, live `#4ADE80`, warning `#F5B942`, danger `#FF6B81`.
- Self-host the three fonts used by the desktop app (Space Grotesk, IBM Plex
  Sans, JetBrains Mono) as WOFF2 subsets. No external font requests. If the
  font files are too large, fall back to a system stack and document it.
- Desktop layout: centred column with a max width, cards, one primary action
  per screen. Mobile: single column, 16px page padding, 44px touch targets.
- No gradients beyond the existing background/primary button treatment, no
  glows, no looping motion, no emoji.

### Accessibility

- Semantic landmarks, one `h1` per page, labelled inputs, visible focus rings.
- Colour is never the only status indicator; text says "Live" / "Offline".
- Confirmation pages name the object and the consequence.
- Forms work with keyboard only and with JavaScript disabled.
- Respect `prefers-reduced-motion`.

## Deployment

### Docker

```text
server/docker/
  Dockerfile
  .dockerignore
  compose.yaml
  .env.example
  setup.sh
  webctl
  README.md
```

Compose shape:

```yaml
services:
  lan-party-web:
    image: ghcr.io/desynkro/lan-party-web:1.3
    restart: unless-stopped
    user: "${PUID:-1000}:${PGID:-1000}"
    ports:
      - "${LANPARTY_WEB_BIND:-0.0.0.0}:${LANPARTY_WEB_PORT:-8080}:8080/tcp"
      - "${LANPARTY_APP_PORT:-51820}:${LANPARTY_APP_PORT:-51820}/udp"
    environment:
      LANPARTY_WEB_DATA_DIR: /data
      LANPARTY_WEB_LISTEN: 0.0.0.0:8080
      LANPARTY_APP_PORT: "${LANPARTY_APP_PORT:-51820}"
    volumes:
      - ./data:/data
    read_only: true
    tmpfs:
      - /tmp
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
```

- `setup.sh` creates `./data` with `0700`, writes `.env` with `PUID`/`PGID`
  from `id -u`/`id -g`, and prints the first-run instructions.
- `webctl` wraps `docker compose exec lan-party-web lan-party-web ...` for
  `passwd`, `show-info` and `healthcheck`.
- The UDP mapping uses the same `LANPARTY_APP_PORT` value as the app, so host
  and container stay in sync.
- The web UI shows the app port read-only with: "The UDP port is managed by
  this deployment. Change it in docker-compose.yml and restart."

### Native

```text
server/native/
  run.sh
  lan-party-web.service
  web.env.example
  README.md
```

- `run.sh` defaults to `./data` beside the script, creates it `0700`, prompts
  for the first-run password, listen address, public address and app port, and
  starts the binary in the foreground.
- A small menu offers show info, set public address, set app port, change
  password, show log tail and quit. It is a convenience, not a daemon.
- The optional systemd unit runs as a dedicated user with `NoNewPrivileges`,
  `ProtectSystem=strict`, `ProtectHome=true` and `ReadWritePaths` for the data
  directory. No timers.
- No root is needed for the normal console path (port 8080).

### Ports and network

- TCP: web UI. Default 8080.
- UDP: the app's tunnel and game traffic. Default 51820.
- The host must forward the UDP port on the router and allow it in the
  firewall. The web UI port is separate; port-forward it only if remote admin
  is intended, and put it behind TLS.
- Changing `LANPARTY_APP_PORT` in Compose also requires updating the router
  forward. Docs call this out.
- The web UI's public address is the address friends dial, not the web UI
  address. They can differ.

### Reverse proxy

Document Caddy and nginx examples. Require:

- TLS termination.
- `X-Forwarded-For` / `X-Forwarded-Proto`.
- `--trusted-proxies` or `LANPARTY_WEB_TRUSTED_PROXIES` set to the proxy's
  source CIDR (usually `127.0.0.1/32`).
- No caching of authenticated pages.
- Request body limit matching the app's 64 KiB.

### Backups and upgrades

- Back up `./data` while the container is stopped, or use a filesystem
  snapshot. The folder contains private host keys.
- Encrypt backups. Restore the whole folder to keep group IDs and pins.
- Upgrade by pulling a new image or replacing the binary; the data directory
  persists. No migration is expected for 1.3, but the settings loader already
  tolerates missing fields.
- A lost `keys/` means existing groups and invites stop matching. The UI must
  say so before a destructive restore.

## CLI and environment

```text
lan-party-web setup      --data-dir DIR [--password VALUE]
lan-party-web run        --data-dir DIR --listen ADDR [--tls-cert FILE --tls-key FILE] [--trusted-proxies CIDRS]
lan-party-web passwd     --data-dir DIR
lan-party-web show-info  --data-dir DIR
lan-party-web healthcheck --url URL
lan-party-web version
```

Environment:

| Variable | Purpose |
| --- | --- |
| `LANPARTY_WEB_DATA_DIR` | Data directory |
| `LANPARTY_WEB_LISTEN` | Listen address |
| `LANPARTY_WEB_PASSWORD` | First-run password; ignored once `web.json` exists |
| `LANPARTY_WEB_APP_PORT` | Effective UDP app port; applied on startup, read-only in UI |
| `LANPARTY_WEB_TLS_CERT` / `_KEY` | TLS certificate and key |
| `LANPARTY_WEB_TRUSTED_PROXIES` | Comma-separated proxy CIDRs |

The public host is **not** an environment variable. It is edited in the web UI
and saved in `settings.json`, so the invite endpoint always matches what the
operator sees.

## Docs to write

- `server/README.md` — what it is, quick start, data directory, security.
- `server/docker/README.md` — Compose, ports, `PUID`/`PGID`, TLS, backups.
- `server/native/README.md` — console runner, systemd, data directory.
- `docs/web-host.md` — user-facing guide: homelab setup, router forward, first
  group, invite, reverse proxy, troubleshooting.
- Main `README.md` — a short 1.3 section linking to the web host guide.
- `CHANGELOG.md` — 1.3.0 entry.
- `docs/releases/v1.3.0.md` — release notes.
- `docs/security.md` — web UI authentication, sessions, storage, exposure.
- `docs/development.md` — build, test and release commands for the web host.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Web UI exposed without TLS | Warning banner, log warning, docs recommend reverse proxy; optional cert flags |
| Weak admin password | Generated first-run password, 12-char minimum, rate limiting, audit log |
| CSRF or session theft | SameSite=Strict, HttpOnly, CSRF tokens, Origin checks, rotation |
| Docker port drift | App port is deployment-owned, read-only in UI, same env var for mapping and app |
| Bind-mount ownership | `setup.sh` writes `PUID`/`PGID`, creates `./data` `0700`, docs explain `id -u` |
| Key theft from disk/backup | Owner-only files, non-root container, full-disk and backup encryption docs |
| Headless app writing autostart | `DisableAutostart` option, test asserts no autostart file |
| Relay settings leaking into server | `DisableRelay` normalization at load; UI has no relay fields |
| Over-polling / wasted traffic | SSE with hash diff, no polling, no external assets |
| Template XSS | `html/template` auto-escaping, no `template.HTML` for user data, CSP |
| Large form bodies | `http.MaxBytesReader`, 64 KiB limit |
| Reverse proxy IP confusion | Trusted proxy CIDRs only; otherwise use the direct peer |
| Lost data directory | Backup docs, key-loss warning, no silent key regeneration |

## Open defaults

These are defaults, not blockers. Change them before implementation if needed.

- Auto-approve new devices: **on** (matches the desktop default; invites are
  one-time and expire in an hour).
- Forget inactive friends: **5 days**, configurable; blank means never.
- Listen: `0.0.0.0:8080` in Docker; `--listen` required or defaulted natively.
- Password: generated on first run if not supplied.
- Fonts: self-hosted WOFF2 if the existing font files are available;
  otherwise a system stack.
- Windows native server build: not in 1.3; Docker covers Windows hosts.
- TOTP / second factor: not in 1.3; noted as a future option.

## Definition of done

- A user can run `docker compose up -d`, open the web UI, log in, create a
  group, add a service, create an invite, start the session, and have a friend
  join with the native app and play.
- The same flow works with the native `run.sh` and no root.
- Restarting the container or binary preserves groups, keys, settings and the
  log.
- The UI works with JavaScript disabled for every action.
- `make test`, `make vet`, `make test-race` and `make vuln` pass.
- Security headers, cookies, CSRF, rate limiting and auth redirects are covered
  by tests.
- The release includes Linux amd64/arm64 web zips, a multi-arch GHCR image,
  Compose files, native runner and docs.
- No relay, join, tray or desktop autostart code is reachable from the web
  binary.
