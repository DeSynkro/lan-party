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

## Changing ports

Both ports are set in `.env` and shown read-only in **Settings → Ports**,
because Docker fixes its mappings when the container is created.

```sh
sudo ./setup.sh --ports
sudo docker compose up -d
```

Or edit `.env` directly:

```sh
LANPARTY_WEB_PORT=9090
LANPARTY_APP_PORT=51821
```

If you change the UDP port, update the router forward too. `LANPARTY_WEB_BIND`
controls which host interface publishes the web UI (`0.0.0.0` by default,
`127.0.0.1` behind a reverse proxy).

## First run

From a source checkout, one command:

```sh
cd server/docker
sudo docker compose up --build -d
sudo docker compose logs lan-party-web | grep -A2 "admin password"
```

From the release zip, use the published image:

```sh
cd docker
sudo docker compose up -d
sudo docker compose logs lan-party-web | grep -A2 "admin password"
```

The first start generates a strong admin password and prints it once in the
logs. Save it in your password manager, open `http://<host>:8080`, and sign in.
After signing in you can recreate the container to drop the password from its
logs:

```sh
sudo docker compose down
sudo docker compose up -d
```

`sudo` is only needed to reach the Docker daemon; the app itself runs as a
non-root user. `setup.sh` is optional. Use it when you want a chosen password or
a different `PUID`/`PGID`:

```sh
cp .env.example .env
sudo ./setup.sh
sudo docker compose up --build -d
```

To change only the ports later:

```sh
sudo ./setup.sh --ports
sudo docker compose up -d
```

The web container waits for a short init container that makes `./data` writable
by `PUID`/`PGID` (default `1000`). No manual `mkdir` or `chown` is needed.

## Data

Everything lives in `./data` beside `compose.yaml`:

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
