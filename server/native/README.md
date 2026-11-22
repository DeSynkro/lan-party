# LAN Party web host — native Linux

No root, no display server and no desktop session. Run the host from a console
or a small systemd unit.

## Console runner

The release zip contains `lan-party-web` at its root, with `run.sh` and the
other native files under `native/`. Extract it into a directory you own:

```sh
mkdir -p ~/lan-party-web && cd ~/lan-party-web
unzip lan-party-web_1.3.0_linux_amd64.zip
cd lan-party-web_1.3.0_linux_amd64
./native/run.sh
```

On first run `run.sh` asks for the listen address and UDP game port, writes
`./native/web.env`, and sets the admin password. It then shows a menu:

```text
  1) Start
  2) Show info
  3) Change admin password
  4) Show log tail
  5) How to set the public address
  q) Quit
```

Start it directly without the menu:

```sh
./native/run.sh start
```

Everything lives beside the script:

```text
data/
  settings.json    0600
  groups.json      0600
  web.json         0600
  keys/            0700
  lan-party.log    0600
web.env            0600
```

Set the public address in the web UI, not here: it is stored in
`settings.json` and shown back to you with the invites.

Forward the UDP game port on your router. The web UI port is separate and does
not need to be public for friends to play.

## systemd (optional)

```sh
sudo useradd --system --home /var/lib/lan-party-web --shell /usr/sbin/nologin lanparty
sudo install -d -o lanparty -g lanparty -m 0750 /var/lib/lan-party-web /etc/lan-party-web
sudo install -m 0755 lan-party-web /usr/local/bin/lan-party-web
sudo install -m 0644 lan-party-web.service /etc/systemd/system/lan-party-web.service
sudo install -m 0600 web.env.example /etc/lan-party-web/web.env
sudo -u lanparty /usr/local/bin/lan-party-web setup --data-dir /var/lib/lan-party-web
sudo systemctl daemon-reload
sudo systemctl enable --now lan-party-web
```

The unit runs as `lanparty` with `NoNewPrivileges`, `ProtectSystem=strict`,
`ProtectHome=true` and only the data directory writable. There are no timers.

## TLS

Use a reverse proxy (Caddy or nginx) in front of `127.0.0.1:8080`, or set
`LANPARTY_WEB_TLS_CERT` and `LANPARTY_WEB_TLS_KEY` in `web.env`. Plain HTTP is
fine on a trusted LAN but logs a warning when bound to a non-loopback address.

## Backups

Stop the host, copy `./data`, and encrypt the copy. The `keys/` directory holds
the host private keys; losing it breaks existing groups and invites.
