# LAN Party relay

The relay lets a LAN Party host accept friends when the host is behind CGNAT or
cannot forward a UDP port. It is optional. Direct connections remain the
default and need no relay.

The relay is an **opaque UDP forwarder**. It does not terminate, inspect or
decrypt the host/friend QUIC/TLS connection. The existing host identity and pin
still authenticate the host end to end.

It is self-hosted: you run it on your own VPS or spare Linux machine. There is
no project-operated public relay.

Treat it as your private infrastructure, not a public service. Friends join
through invites. There is no share-relay action, and lending the relay to
another host means sharing the relay password; that is not encouraged.

## What is in the relay release

```text
lan-party-relay_1.2.0_linux_amd64/
  lan-party-relay              static Linux amd64 binary
  README.md                    this file
  docker/
    compose.yaml
    Dockerfile
    .env.example
    relayctl
    README.md
  native/
    lan-party-relay.service
    relay.env.example
    README.md
  .dockerignore
  LICENSE
  NOTICE
  lan-party-relay-build.txt
```

An arm64 release uses the same layout with the arm64 binary.

The relay is also published as a container image (amd64 for 1.2):

```text
ghcr.io/desynkro/lan-party-relay:1.2
```

On arm64, use the native arm64 binary, or build the image locally from the
arm64 zip.

The image publishes from the same tag as the release. A tag push produces both
the relay zips and the amd64 image.

## Two ways to run it

Both modes run the same binary and have the same security model.

### Docker Compose

For a VPS with Docker. Use the files in `docker/`.

```sh
cd docker
cp .env.example .env
# edit LANPARTY_RELAY_PUBLIC_ADDRESS for your public address
sudo docker compose up -d
sudo ./relayctl showinfo
```

From a source checkout, use `sudo docker compose up --build -d`; the local
`compose.override.yaml` builds the image from source. See
[docker/README.md](docker/README.md) for the full steps.

### Native Linux binary plus systemd

For a machine without Docker, or when you prefer a normal service.

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay
```

Then install the binary and unit from `native/`. See
[native/README.md](native/README.md) for the full steps.

## Setup flow

`lan-party-relay setup` asks for the public relay address, then generates:

- the relay identity key (`relay.key`)
- the relay config and salted password hash (`relay.json`)
- the host profile (`relay-profile.json`)

The host profile contains the relay address, Relay ID and relay password. It
stays in the relay data directory or volume. Run
`lan-party-relay show-info --dir <relay-dir>` (or `sudo ./relayctl showinfo` for
Docker) to display the three values, enter them in LAN Party under **Settings >
Relay**, and use **Test relay**. The app pins the Relay ID, which is the
relay’s public-key fingerprint. The password is printed to the terminal, not
exported to a file.

The relay key never leaves the relay machine. The relay password is host-only
and never goes into an invite. Friends receive a short-lived friend token in
the invite.

## Password and key rotation

- `LANPARTY_RELAY_PASSWORD` or `setup --password` chooses the initial relay
  password. If omitted, setup generates one.
- `passwd` changes the password later without rotating the key. Restart the
  relay and re-enter the new password in the app; the Relay ID stays the same.
- The Relay ID is not a secret, but this relay is private infrastructure. Do
  not post it. It is an identifier, not an access grant.
- `rotate-key` replaces the private key and updates the profile Relay ID. Use it
  if the private key is compromised. Restart the relay, re-enter the new Relay
  ID and issue new invites; the old Relay ID stops working.
- Docker users get `relayctl setpass`, `relayctl relayreset` and
  `relayctl showinfo`; the commands prompt without echoing and never write the
  password to a file.

## Security model

- The host and friends connect **out** to the relay. No inbound port is needed
  on the host.
- The relay sees only encrypted packets.
- The relay password authorizes a host session.
- Friend tokens are short-lived, single-use and validated by hash.
- The relay service stores only token hashes and a password hash. The setup
  profile is the one file with the plaintext password and stays in the relay
  data directory or volume. Use `show-info` to view the values; keep that
  terminal private.
- The relay is private host infrastructure. There is no share-relay action;
  friends join through invites.
- The Relay ID is a public identifier, not a secret and not an access grant.
  The password and private key are the secrets.
- Do not give friends the relay password. Lending the relay to another host is
  a manual password share and is discouraged.
- Control connections are capped globally and per IP; friend traffic has a
  per-friend packet-rate cap.
- Unknown and malformed packets are dropped without a response.
- A Relay ID change requires a re-pin in the host app.
- Direct mode stays the default.

## Friend data path

```text
friend -> relay:7443 (friend token) -> relay (friend id) -> host
host   -> relay:7443 (data token, friend id) -> relay -> friend
```

The relay assigns a 16-bit friend ID. Packets to the host carry a small relay
header with that ID. The host transport wrapper strips the header and presents
a synthetic UDP address per friend to the existing QUIC server. The host's
replies carry the friend ID back, and the relay strips it and sends the inner
packet to that friend.

This keeps one public UDP port. Only `7443` TCP and UDP need to be open; there
is no per-friend port range.

The host wrapper must be able to allocate the synthetic address from the relay
data header even if the control `friend` message has not arrived yet; the
control and data transports are independent.

The friend token is in the header of friend-to-relay packets. The relay strips
it before forwarding. Packets going to a friend are sent from the relay's main
UDP port, so the friend only accepts traffic from the address it dialed.

### Wire format

Control messages use TLS 1.3 over TCP. Each message is a 4-byte big-endian
length followed by a JSON object, with a 64 KiB maximum.

Host to relay:

```text
hello      { version, role: "host", password }
authorize  { token, expires? }
ping       {}
bye        {}
```

Relay to host:

```text
welcome     { sessionId, dataToken }
ok          {}
error       { message }
pong        {}
friend      { friendId }            # friend joined
friendLeft  { friendId }            # friend idle or removed
```

UDP data:

```text
friend -> relay:7443   0x01 + 16-byte raw friend token + inner QUIC packet
host   -> relay:7443   0x02 + 16-byte data token + 2-byte friend id + inner QUIC packet
host   -> relay:7443   0x02 + 16-byte data token                       # keepalive
relay  -> host         0x03 + 2-byte friend id + inner QUIC packet
relay  -> friend:7443  inner QUIC packet
```

Tokens are 128-bit random values. The copyable friend token is 22 base64url
characters. The compact header keeps the relay overhead small enough for a
normal 1500-byte path.

The host data token comes from `welcome`. The friend token is the invite token
decoded from its text form.

The host must send the UDP keepalive immediately after `welcome` and every
20-30 seconds so its NAT mapping stays open. The app should also send one when
it receives a `friend` message if no friend traffic is flowing yet. Friend
packets that arrive before the first host keepalive are buffered (one packet per
friend) and flushed when the host data path registers.

The relay may resend a `friend` message while the host data path is not
registered. Treat it as idempotent.

## Configuration

`relay.json` contains:

```json
{
  "version": 1,
  "listen": ":7443",
  "publicAddress": "relay.example.com:7443",
  "keyFile": "relay.key",
  "passwordSalt": "...",
  "passwordHash": "...",
  "maxSessions": 16,
  "maxFriends": 64
}
```

- `listen` is the local TCP and UDP bind address.
- `publicAddress` is written to the host profile for the app.
- `maxSessions` defaults to 16 host sessions. The desktop app currently uses one
  live session, so the default is generous.
- `maxFriends` defaults to 64 per session.

The relay never needs the plain-text password at runtime; it only stores the
salted hash.

The host control connection must send a `ping` at least every 90 seconds. The
desktop app should ping every 30 seconds. A silent control connection is closed
and its friends are dropped.

A friend that sends no data for five minutes is removed and the host receives a
`friendLeft` control message.

## Commands

```sh
lan-party-relay setup --dir /etc/lan-party-relay
lan-party-relay run --config /etc/lan-party-relay/relay.json
lan-party-relay relay-id --config /etc/lan-party-relay/relay.json
lan-party-relay passwd --dir /etc/lan-party-relay
lan-party-relay rotate-key --dir /etc/lan-party-relay
lan-party-relay show-info --dir /etc/lan-party-relay
lan-party-relay show-profile --dir /etc/lan-party-relay
lan-party-relay version
```

`passwd` changes the relay password without rotating the key. `rotate-key`
generates a new relay identity and updates the host profile Relay ID. Both
require a relay restart and a re-entry in the app. `show-info` is the human
copy/paste view; `show-profile` is the machine-readable JSON and is not needed
for normal setup. `fingerprint` is an alias for `relay-id`.

Docker has friendlier wrappers for the same operations:

```sh
sudo ./relayctl setpass
sudo ./relayctl relayreset
sudo ./relayctl showinfo
```

`setup` is also available as `init`.

`LANPARTY_RELAY_CONFIG` sets the default config path for `run` and
`relay-id`.

Environment during setup:

- `LANPARTY_RELAY_PASSWORD` can supply a password instead of prompting. If it
  is empty, setup generates a strong password. Neither mode prints the password;
  it is stored in the host profile.
- `--non-interactive` requires `--address` and never prompts.

## Building

From the source tree:

```sh
make relay
make relay-native
make relay-image
```

- `make relay` builds a host-arch relay binary and the release zip.
- `make relay-native` builds Linux amd64 and arm64 release zips.
- `make relay-image` builds the container image from the host binary.

The relay does not link GTK and builds with `CGO_ENABLED=0`. It is a separate
release artifact from the desktop app and can be installed without it.
