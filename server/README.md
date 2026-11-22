# LAN Party web host

Run the host side of LAN Party on a homelab or VPS and control it from a
browser. The engine is the same one the desktop host uses; only the UI is
different. Friends keep using the native desktop app and paste an invite. Their
games connect to `127.0.0.1:<port>` exactly as they do today.

This is a **host-only** build. There is no join flow, no relay and no tray. It
runs one live host session at a time, matching the desktop app.

## Quick start

Docker, from `server/docker`:

```sh
cp .env.example .env
./setup.sh
docker compose up -d
```

Native Linux, from a release zip: extract it, then run `./native/run.sh`.

Then open the web UI, sign in, set the public address, create a group, add a
service and invite a friend. Forward the UDP game port on your router first.

- [Docker guide](docker/README.md)
- [Native guide](native/README.md)
- [User guide](../docs/web-host.md)

## Ports

| Port | Protocol | Purpose |
| --- | --- | --- |
| 8080 | TCP | Web UI |
| 51820 | UDP | Host tunnel and game traffic |

The UDP port is owned by the deployment: set it in Compose or `web.env`, change
the router forward, and restart. The web UI shows it read-only.

## Data

The data directory sits beside `docker-compose.yml` or `run.sh`:

```text
data/
  settings.json    0600  includes the public address
  groups.json      0600
  web.json         0600  salted admin password hash
  keys/            0700  host private keys
  lan-party.log    0600
```

Host keys are owner-only files. There is no app-level passphrase in 1.3. Encrypt
the disk and any backup: a stolen `keys/` directory is a stolen host identity.
A lost `keys/` directory means existing groups and invites stop matching.

## Security

- One admin password, PBKDF2-SHA256, no username field.
- In-memory sessions, `HttpOnly` / `SameSite=Strict` cookies, CSRF tokens and
  Origin checks on every mutation.
- Login rate limiting and an audit log.
- Strict response headers, including CSP without inline script.
- `text/html` templates auto-escape every value; view models keep relay and key
  fields out of the templates entirely.
- Optional TLS, or run behind Caddy/nginx. Plain HTTP is for a trusted LAN and
  logs a warning when exposed.

## Development

From the repository root:

```sh
make test
go test ./server/...
go build ./server/cmd/lan-party-web
```

The web binary must not import `internal/gtkui`, and must build with
`CGO_ENABLED=0`. The web UI layer never imports `internal/relay`; the engine
links the relay package but the host forces direct mode, so no relay settings,
password or Relay ID are reachable from the browser.
