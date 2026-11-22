# Hosting from a server

The 1.3 web host runs the same LAN Party host engine as the desktop app, on a
homelab machine or VPS, controlled from a browser. Friends keep using the native
desktop app: they paste an invite and their game connects to `127.0.0.1:<port>`
exactly as before.

This build is **host-only**. There is no join flow, no relay and no tray. It
keeps one live host session at a time, like the desktop app.

## What you need

- A Linux amd64/arm64 machine, or any machine that can run Docker.
- One UDP port forwarded on the router to that machine.
- A public address friends can dial: a hostname, static IP or dynamic DNS name.
- The game server running somewhere this host can reach.

The web UI port is separate from the game port and does not need to be public
for friends to play.

## Docker

From a source checkout, one command:

```sh
cd server/docker
sudo docker compose up --build -d
sudo docker compose logs lan-party-web | grep -A2 "admin password"
```

The first start prints a generated admin password once in the logs. Save it,
then open `http://<server>:8080`. The release zip does the same with
`sudo docker compose up -d` from its `docker/` folder.

`./setup.sh` is optional; use it to choose the password and `PUID`/`PGID`.
See [server/docker/README.md](../server/docker/README.md) for ports, TLS and
backups.

## Native Linux

Download the 1.3 release zip for your architecture and extract it into a folder
you own:

```sh
mkdir -p ~/lan-party-web && cd ~/lan-party-web
unzip lan-party-web_1.3.0_linux_amd64.zip
cd lan-party-web_1.3.0_linux_amd64
./native/run.sh
```

The runner asks for a listen address and UDP port, writes `./web.env`, sets the
password and shows a small menu. No root and no systemd are required. An
optional hardened unit is in `native/lan-party-web.service`.

See [server/native/README.md](../server/native/README.md).

## First setup

1. **Forward the UDP port.** The default is `51820/udp`. Forward it on the
   router to this host. Changing `LANPARTY_APP_PORT` means changing the router
   forward too; the web UI shows the port read-only for that reason.
2. **Sign in** with the admin password. There is no username. A forgotten
   password can be reset from the console with `passwd` (native) or
   `./webctl passwd` (Docker).
3. **Set the public address** in Settings. This is what friends dial. It is not
   the web UI address. If you change it while invites are open, the UI asks you
   to confirm, because those invites point at the old address.
4. **Create a group**, for example "Friday night".
5. **Add a service.** Give it a name, the game protocol, the game server target
   as this host sees it (such as `127.0.0.1:25565`), and the ports. A preset
   fills the common games.
6. **Start the session and create an invite.** Send the code to your friend.
7. Your friend pastes it into the LAN Party desktop app and approves the device.
   Their game connects to `127.0.0.1:<port>`.

New devices are approved automatically by default. Turn that off in Settings to
approve each device by hand. An invite is single use and expires after an hour.

## Reverse proxy and TLS

Directly forwarding the web UI port to the internet without TLS is not
recommended. Terminate TLS in Caddy or nginx and forward to loopback.

**Caddy:**

```caddy
party.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Set in `.env`:

```sh
LANPARTY_WEB_BIND=127.0.0.1
LANPARTY_WEB_TRUSTED_PROXIES=127.0.0.1/32
```

The trusted-proxy setting is what lets the app use `X-Forwarded-For` and
`X-Forwarded-Proto` for the audit log and secure cookies. Without it those
headers are ignored.

**nginx:**

```nginx
server {
    listen 443 ssl;
    server_name party.example.com;

    ssl_certificate     /etc/letsencrypt/live/party.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/party.example.com/privkey.pem;

    client_max_body_size 64k;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;          # server-sent events
    }
}
```

`proxy_buffering off` keeps the live status stream working. The app also sends
`X-Accel-Buffering: no` for nginx.

Alternatively set `LANPARTY_WEB_TLS_CERT` and `LANPARTY_WEB_TLS_KEY` to serve
HTTPS from the app itself.

## Backups

Stop the host, copy the whole data directory, and encrypt the copy. It contains:

```text
data/
  settings.json    public address and preferences
  groups.json      groups, services, members, invites
  web.json         salted admin password hash
  keys/            host private keys
  lan-party.log    application and audit log
```

The `keys/` directory is the host identity. Losing it means existing groups and
invites stop matching; there is no silent key regeneration. Restoring a backup
restores the identity too.

A filesystem snapshot of a stopped container is fine. For Docker:

```sh
docker compose stop
tar -czf lan-party-web-data.tar.gz data
docker compose start
```

Encrypt the archive before it leaves the machine.

## Upgrades

Docker:

```sh
docker compose pull
docker compose up -d
```

Native: stop the runner, replace the binary, start it again. The data directory
is not touched. 1.3 needs no migration; the settings loader tolerates missing
fields.

## Troubleshooting

**The web UI does not load.** Check `docker compose ps` or the native log at
`data/lan-party.log`. If you bound to `127.0.0.1`, connect from the machine or
through the reverse proxy, not from another device.

**A friend cannot connect.** Confirm the UDP port is forwarded to this host and
allowed by the firewall. Confirm the public address in Settings is the address
they dial, and that the session is live.

**An invite stopped working.** Invites are single use and expire after one hour.
Create a new one. A changed public address also invalidates old invites.

**The app port in the UI is wrong.** It is read from the deployment. Open
**Settings → Ports** for the current values and the commands. Change them with
`sudo ./setup.sh --ports` in `server/docker` (or `LANPARTY_APP_PORT` in
`web.env` for a native install), update the router forward, and restart. They
cannot be edited in the UI because Docker's UDP mapping is fixed at container
creation.

**I forgot the admin password.** Run `./webctl passwd` in `server/docker`, or
`native/run.sh passwd` for a native install. Every browser is signed out.

**The password was printed once and lost.** Reset it with `passwd`; do not edit
`web.json` by hand.

**The browser shows a certificate warning.** You are probably using direct TLS
with a self-signed certificate, or a proxy with the wrong certificate. Use a
trusted certificate, or plain HTTP on a trusted LAN.

**Live status does not update.** The page is fully usable without JavaScript.
Use the Refresh link, or check that your reverse proxy is not buffering
`/events`.
