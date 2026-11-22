# LAN Party web host — Docker

Run the host side of LAN Party on a homelab or VPS and control it from a
browser. Friends use the native desktop app as before.

## Ports

| Port | Protocol | Purpose |
| --- | --- | --- |
| 8080 | TCP | Web UI (change with `LANPARTY_WEB_PORT`) |
| 51820 | UDP | Host tunnel and game traffic (change with `LANPARTY_APP_PORT`) |

Forward the **UDP** port on your router. The web UI port is separate; expose it
only when you need remote administration, and put it behind TLS.

The UDP port is deployment-owned. If you change `LANPARTY_APP_PORT`, also change
the router forward and restart the container. The web UI shows the port
read-only for that reason.

## First run

```sh
cd server/docker
cp .env.example .env
./setup.sh
docker compose up -d
```

`setup.sh` fills `PUID`/`PGID` from your user, creates `./data` and `./tls` with
owner-only permissions, and sets the admin password. The password is never
written to `.env` or the container logs.

Open `http://<host>:8080` and sign in. Then set the public address, create a
group, add a service, and invite a friend.

## Data

Everything lives in `./data` beside `docker-compose.yml`:

```text
data/
  settings.json    0600
  groups.json      0600
  web.json         0600  admin password hash
  keys/            0700  host keys
  lan-party.log    0600
```

Back up the whole `./data` directory while the container is stopped, or use a
filesystem snapshot. **Encrypt the backup**: it contains the host private keys.
A lost `keys/` directory means existing groups and invites stop matching.

## TLS

Two supported paths:

1. **Reverse proxy (recommended).** Terminate TLS in Caddy or nginx and forward
   to `127.0.0.1:8080`. Set `LANPARTY_WEB_BIND=127.0.0.1` in `.env` and
   `LANPARTY_WEB_TRUSTED_PROXIES=127.0.0.1/32` so the audit log sees the real
   client address. Caddy example:

   ```caddy
   party.example.com {
       reverse_proxy 127.0.0.1:8080
   }
   ```

2. **Direct TLS.** Put `cert.pem` and `key.pem` in `./tls`, then set:

   ```sh
   LANPARTY_WEB_TLS_CERT=/tls/cert.pem
   LANPARTY_WEB_TLS_KEY=/tls/key.pem
   ```

   Restart with `docker compose up -d`. The certificate files are mounted
   read-only; keep the key at mode `0600` on the host.

Plain HTTP is allowed for a trusted LAN, but the app logs a warning when it
listens on a non-loopback address without TLS.

## Commands

```sh
./webctl show-info
./webctl passwd
./webctl healthcheck --url http://127.0.0.1:8080/healthz
```

## Upgrades

```sh
docker compose pull
docker compose up -d
```

The data directory persists. There is no migration for 1.3; the settings loader
tolerates missing fields.

## Building from source

```sh
cd server/docker
docker compose build
docker compose up -d
```

The build context is the repository root. The root `.dockerignore` keeps the
cache small. `compose.override.yaml` adds the build; the release zip omits that
file and pulls the published image, so change to `cd docker` when working from
a zip.
