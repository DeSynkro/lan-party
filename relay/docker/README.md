# LAN Party relay: Docker Compose

This runs the self-hosted LAN Party relay as a small, non-root container. The
relay forwards encrypted UDP packets between a host and its friends. It cannot
read the host/friend QUIC/TLS traffic.

The image is distroless. It has no shell, package manager or extra tools; the
only executable is `lan-party-relay`.

## Quick start

From a source checkout, `compose.override.yaml` builds the image locally:

```sh
cd relay/docker
cp .env.example .env
# edit LANPARTY_RELAY_PUBLIC_HOST and LANPARTY_RELAY_PORT
sudo docker compose up --build -d
sudo docker compose logs -f
```

From a release zip, the published image is used:

```sh
cd docker
cp .env.example .env
# edit LANPARTY_RELAY_PUBLIC_HOST and LANPARTY_RELAY_PORT
sudo docker compose up -d
sudo docker compose logs -f
```

The first start creates `relay.json`, `relay.key` and `relay-profile.json` in
the `relay-data` volume. No separate setup command is needed.

Then show the values for the host app:

```sh
sudo ./relayctl showinfo
```

Set `LANPARTY_RELAY_PUBLIC_HOST` to the hostname or IP that friends and the
host app use, and `LANPARTY_RELAY_PORT` to the public port:

```text
LANPARTY_RELAY_PUBLIC_HOST=relay.example.com
LANPARTY_RELAY_PORT=7443
```

For example, `203.0.113.10` on port `8443`:

```text
LANPARTY_RELAY_PUBLIC_HOST=203.0.113.10
LANPARTY_RELAY_PORT=8443
```

`LANPARTY_RELAY_PORT` is both the public port and the host port Docker
publishes. Docker maps it to the container's internal `7443`. Friends use
`host:port`.

The host and port are written into the host profile on the first start.
Changing them afterwards does not update the existing profile; recreate the
relay with `sudo docker compose down -v` if you need to change them (this
rotates the relay identity).

## Show the app values

The relay ships with `relayctl` in this directory. On a source checkout it is
`relay/docker/relayctl`; in the release zip it is `docker/relayctl`.

```sh
sudo ./relayctl showinfo
```

It prints the four values the host app needs:

```text
Relay host:       relay.example.com
Relay port:       7443
Relay ID:         774e2e…
Relay password:   …
```

Enter them in LAN Party under **Settings > Relay**. The app pins the Relay ID.

This relay is host-only infrastructure, not a public service. Do not post the
Relay ID or give out the password. Friends join through invites.

The password is printed only to the terminal, not written to a file. Keep the
terminal private and clear it after copying. The host profile stays inside the
`relay-data` volume; it is not exported.

If you need the machine-readable profile for a future file import, the
`show-profile` command still exists, but `showinfo` is the safer path for normal
use.

## Password and key rotation

To choose your own password before the first start, set it in `.env`:

```text
LANPARTY_RELAY_PASSWORD=your password
```

Then start normally. The safer path is to start with the generated password and
run `sudo ./relayctl setpass`; the password is not stored in `.env` or a file.

Change the password later without rotating the key:

```sh
sudo ./relayctl setpass
```

It prompts twice without echoing, updates the config and the host profile,
restarts the relay, and leaves the Relay ID unchanged. Run `showinfo` to copy
the new password into the app.

The Relay ID is not a secret, but this is private infrastructure. Do not post
it. It is an identifier, not an access grant. If the **private** `relay.key` is
compromised, or you want a new relay identity:

```sh
sudo ./relayctl relayreset
```

It asks for confirmation, replaces the key, updates the profile Relay ID and
restarts the relay. The password stays the same. Old pins and invites that carry
the old Relay ID stop working.

## Firewall

Allow both protocols. The control channel is TCP; friend traffic is UDP. Only
the relay port is needed; there is no per-friend port range.

With ufw:

```sh
sudo ufw allow 7443/tcp
sudo ufw allow 7443/udp
```

Also open the port in your VPS provider's firewall or security group. A TCP-only
check is not enough.

## Updating

Source checkout:

```sh
sudo docker compose up --build -d
```

Release zip or published image:

```sh
sudo docker compose pull
sudo docker compose up -d
```

The `relay-data` volume keeps the key and config across updates. Do not remove
it unless you intend to change the relay identity and re-import the profile.

## Common commands

```sh
sudo ./relayctl setpass
sudo ./relayctl relayreset
sudo ./relayctl showinfo
sudo docker compose ps
sudo docker compose logs -f
sudo docker compose restart relay
sudo docker compose run --rm relay relay-id
sudo docker compose run --rm relay version
```

## Build the image locally

From a source checkout, `sudo docker compose up --build -d` builds
`Dockerfile.dev` automatically. No separate build step is needed.

To build from an extracted relay zip (which contains the static binary):

```sh
cd lan-party-relay_1.2.0_linux_amd64
sudo docker build --build-arg VERSION=1.2.0 -f docker/Dockerfile -t lan-party-relay:local .
```

Then set this in `docker/.env`:

```text
LANPARTY_RELAY_IMAGE=lan-party-relay:local
```

and run `sudo docker compose up -d`.

The published 1.2 image is amd64. On arm64, use the native arm64 relay binary or
build the image locally from the arm64 zip. The GHCR package should be public;
if it is private, run `docker login ghcr.io` before pulling.

The `:1.2` image is only published after a release tag runs. Before that, a
release-zip `up -d` fails with a registry `denied` error. Use a source checkout
with `up --build`, or build a local image as above and set
`LANPARTY_RELAY_IMAGE`.

## Uninstall

```sh
sudo docker compose down -v
```

`-v` removes the `relay-data` volume, including the relay identity key and
config. This cannot be undone.

## Troubleshooting

**Registry denied.** The `:1.2` image is not published yet or the GHCR package
is private. From a source checkout use `sudo docker compose up --build -d`.
With a release zip, build the local image and set `LANPARTY_RELAY_IMAGE` as
shown above.

**The relay restarts or logs a config error.** Check the logs. The first start
must be able to write `/data`. If the named volume has the wrong ownership,
remove it and start again:

```sh
sudo docker compose down -v
sudo docker compose up --build -d
```

**Address already in use.** Another process on the relay machine owns the host
port. Check with `sudo ss -lntup | grep 7443` or change `LANPARTY_RELAY_PORT` in
`.env`.

**Friends cannot connect.** Confirm both TCP and UDP are open on the relay
machine and at the VPS provider. The relay also needs outbound UDP to reach
friends' advertised addresses, but no other inbound ports.

**The app says the Relay ID changed.** The relay key changed. Only re-enter it
if you changed it intentionally. Otherwise treat it as a possible
man-in-the-middle and investigate.

**The relay uses too much memory.** The Compose file caps it at 128 MB and one
CPU. The relay is lightweight; lower the caps if you want, or raise them for a
very large group. `MaxFriends` in `relay.json` defaults to 64.
