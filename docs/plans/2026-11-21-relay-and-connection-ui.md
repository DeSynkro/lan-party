# LAN Party 1.2 relay and connection UI

Status: implementation in progress. The standalone relay server, setup CLI,
native/Docker packaging, release zip build and relay tests are in place. Desktop
app integration (invite v2, relay transport, host profile import, Settings and
group UI) is next.

This document records the relay direction and the connection-UI decisions so the
next session does not have to reconstruct them from chat.

## Goal

Add an optional self-hosted relay for hosts behind CGNAT or on networks where a
UDP port cannot be forwarded. The relay must be secure by default, optional, and
simple enough that the common direct path stays unchanged.

## Scope

- Self-hosted relay server, run by the user on a VPS or spare machine.
- Host and friends connect out to the relay. No inbound port on the host.
- The host/friend QUIC/TLS connection stays end to end. The relay forwards
  encrypted packets and cannot read game traffic.
- Relay defaults in Settings.
- Per-group Override Settings gate for direct or relay connections.
- Docker Compose recipe.
- Relay identity key, host-only password, and short-lived friend tokens.
- Hardening, limits and tests.

## Non-goals

- A public relay operated by the project.
- Multiple live host sessions in 1.2.
- Per-friend service grants.
- Windows installer, single-file launcher, Flatpak or auto-update.
- Windows code signing (tracked separately).
- A native Win32 UI.

## Security model

- The relay is an opaque UDP forwarder, not a VPN and not a decryption point.
- The existing host/friend QUIC/TLS handshake remains end to end. The host key
  pin still authenticates the host.
- The relay has its own identity keypair. The app pins the relay fingerprint,
  like an SSH host key. A changed key warns and requires a re-pin.
- The relay password is host-only. It authorizes the host to create a session.
- Friends do not receive the relay password. Each invite carries a short-lived
  friend token.
- The relay stores only token hashes and expiry. Tokens are single-use.
- The relay never logs payloads, tokens, passwords or invite codes.
- The relay is never an open service. No default credentials.
- Rate limits, connection caps, packet-size caps and idle timeouts are
  required.
- Unknown or malformed packets are dropped without a response, to avoid
  amplification.
- Direct connections remain the default. Relay mode is explicit.

## Relay data path

The spike path is implemented and covered by tests. It preserves the existing
QUIC/TLS stack and avoids a QUIC-aware relay.

```text
friend -> relay:7443 (friend token) -> relay (friend id) -> host
host   -> relay:7443 (data token, friend id) -> relay -> friend
```

- The host opens a TLS control connection over TCP, authenticates with the
  relay password, and keeps the session alive with control pings.
- The host also opens one UDP data socket to `relay:7443`, sends a keepalive
  immediately and every 20-30 seconds, and uses that socket for all relay
  friend traffic.
- Each friend joins with an invite token.
- The relay assigns a 16-bit friend ID. It does not allocate a host-facing UDP
  port per friend.
- The friend sends a one-byte friend-data type, its 16-byte raw token and the
  inner QUIC packet to the relay's main UDP port.
- Packets to the host carry a one-byte relay-data type, the friend ID and the
  inner QUIC packet. The app's transport wrapper strips the header and presents
  a distinct synthetic UDP address per friend to the existing QUIC server, so
  multiple friends work without changing QUIC routing.
- The host sends a one-byte host-data type, its 16-byte data token, the friend
  ID and the inner QUIC packet. The relay strips the header and forwards the
  packet to that friend from the main UDP port. A host keepalive omits the
  friend ID and payload.
- The relay never terminates the inner QUIC/TLS connection.
- Only `7443` TCP and UDP need to be open. There is no per-friend port range.
- On a friend join, the relay sends the host a `friend` control message with
  the numeric friend ID. If the host data path is not registered yet, the
  relay buffers one first packet per friend and flushes it on the first host
  keepalive.
- The control `friend` message and the first relay-to-host data packet can
  arrive in either order. The host wrapper allocates the synthetic address from
  the packet header and treats the control message as friend-list metadata.

### App transport hook

The relay protocol needs two `net.PacketConn` wrappers around quic-go's UDP
socket:

- **Host relay connection.** `ReadFrom` accepts `0x03`, strips the 2-byte
  friend ID, and returns a stable synthetic `*net.UDPAddr` per friend.
  `WriteTo` maps that synthetic address back to the friend ID and prepends
  `0x02` plus the data token. A `SendKeepalive` method writes `0x02` plus the
  data token immediately after `welcome` and every 20-30 seconds.
- **Friend relay connection.** `ReadFrom` returns relay packets unchanged.
  `WriteTo` prepends `0x01` plus the friend token.

Both wrappers are passed to `quic.Transport{Conn: ...}` instead of using
`quic.ListenAddr` / `quic.DialAddr` directly. The wrapper must not expose the
underlying `*net.UDPConn` as an `OOBCapablePacketConn`, or quic-go could bypass
the relay header. If a plain `net.PacketConn` is exposed, quic-go falls back to
normal `ReadFrom` / `WriteTo`, which is what the wrappers need.

Spike status:

- Friend IDs and two friends on one session: tested locally.
- Host data keepalive and buffered first friend packet: tested locally.
- Host NAT mapping and keepalive: unit-tested on loopback; still to verify
  through a real CGNAT/NAT path.
- Relay restart and host re-registration: still to test with the app.
- Direct mode unaffected: app integration not started yet.

## Relay control and authentication

- Relay control channel is TLS 1.3 over TCP, pinned to the relay key.
- Host authenticates with the relay password and creates a session.
- The host authorizes each invite token over the control channel with an
  expiry. The relay caps the expiry at one hour.
- Friends have no control channel. Each friend data packet carries its token,
  and the relay assigns the friend ID and data binding on the first valid
  packet.
- Tokens are single-use and bound to one session. A token cannot create a
  second friend after the first one idles out or disconnects.
- Tokens are stored as SHA-256 hashes. Raw tokens and the relay password are
  never stored or logged.
- Relay address is `host:port`. The address includes the port. No separate relay
  port field.
- Relay fingerprint/key is printed by setup and in the relay logs, and is
  written into the host profile.

## UI: Network Defaults

`Use Relay` is off by default. The checkbox swaps the active field set. Both
direct and relay values stay saved while hidden.

Direct mode:

```text
Network Defaults
Host groups use these defaults unless they set their own connection.

[ ] Use Relay

Public Address  [ home.example.net ]
App Port        [ 51820 ]

Use a hostname or IP. Add an outside port only when your
router forwards a different public port.

Friends connect to home.example.net:51820.

[ Save Network Settings ]
```

Relay mode:

```text
Network Defaults
Host groups use these defaults unless they set their own connection.

[x] Use Relay

Relay Address     [ relay.example.net:7443 ]
Relay Fingerprint [ ABCD…                  ]
Relay Password    [ ••••••••               ]

The relay is a server you run. Friends do not need the relay
password. The invite carries a short-lived token.

Friends connect through relay.example.net:7443.
No port forward is needed.

[ Test Relay ]
[ Save Network Settings ]
```

The password is stored in an owner-only secret file, not in the ordinary
settings JSON. It is never displayed after saving and never placed in an invite.

## UI: Group connection

The group dialog uses a single `Override Settings` gate.

Override off: read-only inherited connection.

```text
Connection for Friday Night
This changes only this group.

[ ] Override Settings
    Using Settings: Relay
    Relay: relay.example.net:7443 · fingerprint ABCD…
    Friends connect through the relay.

[ Cancel ]  [ Save ]
```

Override on: editable connection.

```text
[x] Override Settings

[x] Use Relay
    [ ] Custom relay for this group
        Relay Address     [ … ]
        Relay Fingerprint [ … ]
        Relay Password    [ … ]

    Relay: from Settings / custom
    Friends connect through relay.example.net:7443.

[ Reset ]  [ Cancel ]  [ Save ]
```

Direct mode when Override Settings is on:

```text
[ ] Use Relay

Direct Endpoint
Public Address  [ blank = use Settings ]
App Port        [ blank = use Settings ]

Default: home.example.net:51820
Friends connect to home.example.net:51820.
Host listens on UDP 51820.
```

Rules:

- Override off uses Settings. Existing group values are kept but inactive.
- Override on uses the group connection. Direct fields and relay fields are
  independent. Blank direct fields use Settings.
- `Use Relay` appears only when Override Settings is on.
- A group can use the relay config from Settings, or a custom relay.
- A custom group relay stores its own password in an owner-only secret file.
- `Reset` appears only while Override Settings is on. It clears all group
  connection overrides and the custom relay password, and unchecks Override.
  It is a form action: Save commits, Cancel discards.
- Unchecking Override alone keeps the saved group values for later. To clear
  them, check Override, click Reset, then Save.
- No “saved override” announcement.
- Remove the old `Use defaults` button. `Reset` replaces it.
- The group card shows `Default` when Override is off and `Custom` when on.

## State and migration

- Settings gains relay address, relay fingerprint, relay password secret, and a
  `UseRelay` default flag. Default is off.
- Group gains:
  - `OverrideSettings bool`
  - `UseRelay bool`
  - optional relay override fields
  - custom relay password in a separate secret store
- Existing 1.1 groups with a public-address or app-port override migrate to
  `OverrideSettings=true`. Groups with no overrides stay false.
- The group store version can stay 1 because the new fields are optional JSON.
- Direct invites stay compatible with 1.1. Relay invites use a new invite
  version, and 1.2 decodes both. A 1.1 client rejects a relay invite.
- If a relay session fails to start, the session fails with a clear error. Do
  not silently fall back to direct unless the user chooses it.

## Docker Compose

Relay binary: Go, no GTK, separate `cmd/lan-party-relay`. See
`relay/docker/README.md` for the user-facing steps.

- TCP control port and UDP data port share the same number, `7443`.
- The container's first `run` auto-creates the relay key, config, password and
  host profile if the config is missing and `LANPARTY_RELAY_PUBLIC_ADDRESS` is
  set. The relay prints its fingerprint in its logs. No separate setup command
  is needed.
- No default credentials. No password in a committed env file. The generated
  password is only in the host profile inside the relay volume.
- Image published to GHCR: `ghcr.io/desynkro/lan-party-relay:1.2`.
- Compose example lives in `relay/docker/compose.yaml`.
- `compose.override.yaml` in a source checkout builds a local image
  automatically, so `docker compose up --build -d` is the whole build/start
  step. The release zip does not include the override.
- Container runs non-root, read-only root, `cap_drop: [ALL]`,
  `no-new-privileges`, resource limits and a named `relay-data` volume.
- The host profile is read with `docker compose run --rm relay show-profile`.

Actual Compose shape:

```yaml
services:
  relay-init:
    image: busybox:1.36
    user: "0:0"
    restart: "no"
    read_only: true
    volumes:
      - relay-data:/data
    command: ["sh", "-c", "chown -R 65532:65532 /data"]

  relay:
    image: "${LANPARTY_RELAY_IMAGE:-ghcr.io/desynkro/lan-party-relay:1.2}"
    restart: unless-stopped
    depends_on:
      relay-init:
        condition: service_completed_successfully
    environment:
      LANPARTY_RELAY_CONFIG: /data/relay.json
      LANPARTY_RELAY_PUBLIC_ADDRESS: "${LANPARTY_RELAY_PUBLIC_ADDRESS:-127.0.0.1:7443}"
    ports:
      - "${LANPARTY_RELAY_PORT:-7443}:7443/tcp"
      - "${LANPARTY_RELAY_PORT:-7443}:7443/udp"
    volumes:
      - relay-data:/data
    read_only: true
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    pids_limit: 128
    mem_limit: 128m
    cpus: 1.0

volumes:
  relay-data:
```

## Hardening checklist

Implemented:

- End-to-end host/friend TLS. The relay sees ciphertext only.
- Relay key pin and key-change warning in the app (pin verification in the
  relay client; UI warning comes with app integration).
- Host password with PBKDF2-SHA256, 600,000 iterations.
- Per-invite tokens stored as SHA-256 hashes with a one-hour cap.
- Constant-time token and password comparisons. No secret logging.
- Handshake cap per IP and globally, 500 ms denial delay.
- Max sessions and friends per session.
- Per-friend packet-rate cap.
- Friend and control idle timeouts.
- Packet-size caps. Unknown/malformed packets are dropped without a response.
- Single-use tokens, token replay rejection, expired token rejection.
- Tests for one/two friends, distinct friend IDs, wrong password, wrong pin,
  token replay, expiry, caps, malformed packets, idle cleanup and shutdown.
- Docker non-root, read-only root, capability drop, resource limits and a
  named `relay-data` volume.
- Systemd `ProtectSystem=strict`, `ProtectHome`, no capabilities, no new
  privileges, `SystemCallFilter=@system-service`, `MemoryDenyWriteExecute`.

Still to do:

- App-side relay invite v2, relay transport hook, host profile import and UI.
- Fuzz the packet and control parsers.
- End-to-end test through the desktop app, including a simulated CGNAT path.
- Relay restart/re-registration test with the real host app.
- Optional bandwidth accounting if per-friend packet-rate caps are not enough.

## Approach order

Relay first, UI after. The relay data path and control/auth model are settled
and tested on loopback. The remaining high-risk work is the app-side transport
wrapper and a real NAT/CGNAT test. Do not wire the UI until that works.

If a visual check is useful earlier, use a throwaway `gtk-probe` screen with
fake data.

## Setup and deployment UX

The relay ships as its own release zip and runs independently of the desktop
app. It has no GTK dependency. Most relay machines are headless VPSes reached
over SSH, so the dedicated setup interface is a small CLI with plain prompts,
not a GUI.

Relay zip contents:

- static Linux amd64/arm64 `lan-party-relay` binary
- `lan-party-relay setup` command
- `docker/compose.yaml` and env example
- `native/lan-party-relay.service`, env example and install notes
- `README.md` with the quick start

Native path:

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay
```

The setup asks for the public relay address (port defaults to `7443`), generates
the relay identity key and password, writes `relay.json`, `relay.key` and
`relay-profile.json`, and prints the profile path and fingerprint.

Docker path:

```sh
cd relay/docker
cp .env.example .env
# set LANPARTY_RELAY_PUBLIC_ADDRESS
sudo docker compose up --build -d
```

The first start auto-creates the config, key and host profile in the
`relay-data` volume. The host profile is read with:

```sh
sudo docker compose run --rm relay show-profile > relay-profile.json
```

The container is the same relay binary.

The host profile is an owner-only JSON file. It is copied to the host desktop
with `scp` and imported into the main app's Relay settings. The relay private
key never leaves the relay machine and is never in the profile or an invite.

The main app does not generate relay bundles. It imports the host profile or
accepts the relay address, key and password manually, pins the key and offers
**Test relay**.

Advanced/manual commands:

```sh
lan-party-relay setup --dir /etc/lan-party-relay
lan-party-relay run --config /etc/lan-party-relay/relay.json
lan-party-relay fingerprint --config /etc/lan-party-relay/relay.json
lan-party-relay show-profile --dir /etc/lan-party-relay
lan-party-relay version
```

`setup` is also available as `init`. `LANPARTY_RELAY_CONFIG` sets the default
config path for `run` and `fingerprint`.

Friends never configure the relay. They paste the invite, which carries the
relay address, pin and short-lived friend token.

No public relay and no automatic deployment over SSH in 1.2.

## Repository layout

```text
relay/
  README.md
  .dockerignore
  docker/
    compose.yaml
    Dockerfile
    compose.override.yaml   # source checkouts only; builds Dockerfile.dev
    Dockerfile.dev          # source checkouts only; not in the release zip
    .env.example
    .gitignore
    README.md
  native/
    lan-party-relay.service
    relay.env.example
    README.md
build/relay.sh
cmd/lan-party-relay/
internal/relay/
```

The project keeps one Go module. The relay is a separate binary and release
artifact, built with `CGO_ENABLED=0`. The desktop app only imports the host
profile. `relay/` holds the deployment templates and documentation. Build
targets can produce the relay zip, native binary and Docker image without
building the desktop app.

## Testing plan

Implemented:

- Setup tests: generated key, password, host profile permissions, setup refusal
  on existing config/profile/key, public-address normalization, insecure file
  modes.
- Relay server tests: TLS host hello/password, wrong password, wrong fingerprint,
  authorize/expire/replay friend tokens, one and two friends, distinct friend
  IDs, buffered first packet, host data-address binding, malformed and
  oversized packets, session/friend/handshake/per-IP caps, idle friend cleanup,
  idle control cleanup and shutdown during a handshake.
- `go test -race ./internal/relay`.

Remaining:

- Setup CLI tests for the interactive prompt and generated password.
- Host profile import and manual relay-config tests in the desktop app.
- End-to-end host/friend relay test with the app's real QUIC transport.
- Relay restart and host re-registration.
- Manual Docker Compose test on the user's machine.
- Manual VPS test and a simulated CGNAT host that allows only outbound relay
  traffic.
- Confirm the relay sees only ciphertext and never logs tokens or payloads in a
  full app session.
- Fuzz the packet and control parsers.
- UI state tests for Settings direct/relay, group inheritance, Override
  Settings, Reset and the custom relay path.

## Open questions

- Should the app show the full relay fingerprint or a short grouped form?
- Where exactly to store the relay password secret: a new `relay.json`, the
  existing key store, or a dedicated secret file. Per-group custom relays need
  multiple entries.
- How to show relay status in the group card without adding clutter.
- How the app displays a relay key-change warning and re-pin flow.
- Whether a failed relay session offers a direct retry when a valid public
  endpoint exists, or always fails until the user switches mode.

Resolved:

- Relay data path: one public UDP port, friend IDs in a small header, token
  prefixed friend packets and a host data token. Tested on loopback.
- Tokens are generated by the host, sent to the relay as hashes and are
  single-use.
- Relay port is `7443`, with TCP control and UDP data on the same number.
- Relay failures fail the session with a clear error; no silent direct fallback.
- Docker image publishes to GHCR; Compose lives in `relay/docker/`.

## Next step

The relay is ready for its Docker Compose test. After that, start app
integration in this order:

1. Invite v2 with relay address, fingerprint and friend token.
2. Host/friend relay transport hook around the existing QUIC socket:
   token/header wrapper on the friend side and a synthetic per-friend address
   wrapper on the host side.
3. Host profile import and relay test in Settings.
4. Settings and group UI for direct/relay and Override Settings.
5. End-to-end host and two friends through a real relay, then a simulated
   CGNAT path.
