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
# edit LANPARTY_RELAY_PUBLIC_ADDRESS for your public address
sudo docker compose up --build -d
sudo docker compose logs -f
```

From a release zip, the published image is used:

```sh
cd docker
cp .env.example .env
# edit LANPARTY_RELAY_PUBLIC_ADDRESS for your public address
sudo docker compose up -d
sudo docker compose logs -f
```

The first start creates `relay.json`, `relay.key` and `relay-profile.json` in
the `relay-data` volume. No separate setup command is needed.

Set `LANPARTY_RELAY_PUBLIC_ADDRESS` to the hostname or IP that friends and the
host app use, with the public port:

```text
relay.example.com
relay.example.com:7443
203.0.113.10
203.0.113.10:8443
```

If the public port is not `7443`, set `LANPARTY_RELAY_PORT` in `.env` to the
same public port. The container listens on `7443` internally.

The address is written into the host profile on the first start. Changing it
afterwards does not update the existing profile; recreate the relay with
`sudo docker compose down -v` if you need to change it (this rotates the relay
identity).

## Host profile

Print the profile and save it on the host:

```sh
sudo docker compose run --rm relay show-profile > relay-profile.json
chmod 600 relay-profile.json
```

Import it in LAN Party under **Settings > Relay**, then use **Test relay**. The
app pins the relay fingerprint from the profile.

`show-profile` prints the relay password. Keep the file private. The password
cannot be recovered from `relay.json`; keep a copy in your password manager.

## Password and key rotation

To choose your own password before the first start, set it in `.env`:

```text
LANPARTY_RELAY_PASSWORD=your password
```

Then start normally. If the relay is already configured, use `passwd` instead.

Change the password without rotating the key:

```sh
sudo docker compose run --rm -e LANPARTY_RELAY_PASSWORD='new password' \
  relay passwd --dir /data
sudo docker compose restart relay
sudo docker compose run --rm relay show-profile > relay-profile.json
chmod 600 relay-profile.json
```

The fingerprint stays the same. Re-import the profile in the app so it gets the
new password.

The fingerprint is public and safe to share. Leaking it does not require a
reset. If the **private** `relay.key` is compromised, or you want a new relay
identity, rotate the key with the relay stopped:

```sh
sudo docker compose stop relay
sudo docker compose run --rm relay rotate-key --dir /data
sudo docker compose start relay
sudo docker compose run --rm relay show-profile > relay-profile.json
chmod 600 relay-profile.json
```

This changes the fingerprint. Re-import the profile and issue new invites; old
pins and invites that carry the old fingerprint stop working.

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
sudo docker compose ps
sudo docker compose logs -f
sudo docker compose restart relay
sudo docker compose run --rm relay fingerprint
sudo docker compose run --rm relay show-profile
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
port. Check with `sudo ss -lntup | grep 7443` or change
`LANPARTY_RELAY_PORT` in `.env` and `LANPARTY_RELAY_PUBLIC_ADDRESS` together.

**Friends cannot connect.** Confirm both TCP and UDP are open on the relay
machine and at the VPS provider. The relay also needs outbound UDP to reach
friends' advertised addresses, but no other inbound ports.

**The app says the fingerprint changed.** The relay key changed. Only re-pin if
you changed it intentionally. Otherwise treat it as a possible
man-in-the-middle and investigate.

**The relay uses too much memory.** The Compose file caps it at 128 MB and one
CPU. The relay is lightweight; lower the caps if you want, or raise them for a
very large group. `MaxFriends` in `relay.json` defaults to 64.
