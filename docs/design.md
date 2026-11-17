# LAN Party design

Written 17 November 2026, while the rewrite is being built. If something here
turns out to be wrong, the document changes with the code.

## The problem

Hosting a game for a friend over a normal VPN gives away more than the game.
To let them reach `192.168.1.50:25565` you usually hand them
`192.168.1.0/24`, which also contains your NAS, your router's admin page and
your printer. Or you hand them a default route and discover your upload is now
their internet.

The first version of this app solved that with a full WireGuard tunnel and a
TCP/IP stack (gVisor) running inside the process. It worked, but it was a lot
of machinery to move one game connection, and it made the project hard to
follow and hard to change safely.

This rewrite keeps the product and throws away the machinery. One encrypted
port tunnel, one service list, one host approval step.

## What the app does

The host runs a game server somewhere on their LAN. They add it to a group as a
service:

```
Minecraft — TCP
on my LAN        192.168.1.50:25565
friends open     127.0.0.1:25565
```

They start a session and create a one-time invite. The friend pastes the invite
into their app. The host approves the friend's device. The friend opens
Minecraft, chooses Direct Connect and enters `127.0.0.1:25565`.

The friend's game never leaves their machine. Their app accepts the connection
on loopback, carries it over one encrypted connection to the host, and the host
dials `192.168.1.50:25565`. Only services the host published exist. There is no
virtual network, no route and no peer-to-peer path.

## Goals

- One window, fixed at 400×640, that looks like a finished desktop utility.
- Publish any TCP and/or UDP port the host chooses, with an optional remap for
  the port friends see.
- Invites are one-time, expire, and never contain a private key.
- The host sees who joined and approves new devices.
- Keys stay out of plain JSON and out of the invite string.
- A connection can be followed from the game to the LAN target by reading the
  documentation, without knowing how a VPN works.
- Linux first. Windows later, once the Linux version is solid.

## Non-goals

- A public relay for hosts behind CGNAT. Direct hosting needs a reachable
  address or a port forward.
- Peer-to-peer connections between friends. The host is the star.
- More than one publishing host per session.
- Per-friend service grants in v1. Every member of a group sees every published
  service. This can be added later without changing the tunnel.
- Launching games or writing into their server lists.
- Answering LAN broadcast discovery. Games that only use a server browser will
  not see the session.
- Auto-update.

## Anti-goals

These would count as failures even if the app technically works:

- A key or invite token lying in a readable file.
- A friend reaching anything the host did not publish.
- A green "connected" indicator that is not based on a real connection.
- An interface that looks like a generic dark dashboard.
- A backend the owner cannot explain in a few sentences.

## Constraints

- Go 1.27.4, pinned in `.tools`, with the module and build caches inside the
  project.
- Go and Fyne only. No webview, no npm, no bundled browser.
- Fixed 400×640 window. No maximise, no resize.
- Linux runtime data in `~/.config/lan-party` and `~/.local/share/lan-party`.
  Development runs redirect those into `.tmp`.
- No artifact, cache or build tool outside this directory. CI is the only
  exception, because it runs on a machine we do not own.
- Commits are small and say what happened in the past tense.
- Documentation lives in `docs/`.

## Approaches considered

**A. One encrypted port tunnel.** Replace WireGuard and gVisor with one QUIC
connection. TCP travels as streams, UDP as datagrams. The friend app opens
loopback listeners; the host app dials fixed LAN targets. TLS 1.3 does the
crypto. Chosen.

**B. Keep WireGuard and gVisor, rebuild around them.** Lowest risk to the
networking code, but the two black boxes stay, the binary stays heavy, and the
part the owner cannot explain stays the core. Rejected.

**C. Refresh the UI and security only.** Least work, but not the clean
recreation asked for, and it leaves the core untouched. Rejected.

A is smaller, fits "lightweight and simple", and gives us a data path that can
be explained in one page. The risk is replacing working code; end-to-end tests
with real TCP and UDP echo through a real QUIC connection are the mitigation.

## How it works

### The tunnel

The host opens one UDP socket, by default on port 51820. That is the only
internet-facing port. Friends connect to the host's public address and that
port over QUIC. The QUIC handshake is TLS 1.3. The host certificate is
self-signed and pinned by its public key hash, which travels in the invite and
is stored in the friend's group entry.

The host's identity key is generated when the group is created and kept in an
owner-only file. The certificate is built from that key, so re-generating the
certificate does not change the pin.

### Publishing a service

A service has a name, a protocol (TCP, UDP or both), a LAN target and one or
more ports friends see. The target is an IP literal; hostnames are not resolved
on the host's behalf.

- If the target is written as `192.168.1.50` and the service publishes ports
  `2456-2458`, friend port `p` goes to `192.168.1.50:p`. This covers games like
  Valheim that use a range.
- If the target is written as `192.168.1.50:25566` and the service publishes
  `25565`, friends see `25565` and the host dials `25566`. This is the remap
  case.
- A target with a port and more than one published port is rejected, because
  the mapping would be ambiguous.

Validation rejects wildcard, multicast and broadcast targets, warns about
loopback targets (a game server on the same PC is fine) and warns about
administrative ports such as 22, 3389 and 445. The host can still choose them;
the warning is not a silent block.

### TCP

For every published TCP port, the friend app listens on `127.0.0.1:<port>`
only. When a game connects, the app opens a QUIC stream, writes a small header
identifying the service and port, then copies bytes. The host checks that the
service exists, dials the target and copies bytes back.

Each accepted game connection gets its own stream, so one slow stream does not
block another. Half-close is preserved: the stream is closed for writing while
the other direction can finish.

### UDP

For every published UDP port, the friend app binds `127.0.0.1:<port>` and keeps
a flow table keyed by the game's source address. Each flow gets an id. Packets
travel as QUIC datagrams with the published port and flow id in front. The
host keys flows by that pair, keeps one UDP socket per flow towards the target
and sends replies back with the same id.
Flows expire after 60 seconds of silence.

Datagrams are limited to the QUIC datagram size (about 1200 bytes). Larger
packets are dropped rather than fragmented. Games that send larger UDP packets
are out of scope for v1.

### The control stream

The first stream on a connection is a control stream carrying newline-delimited
JSON. It handles hello, enrollment, the service list, ping/pong, presence and
revocation. The host pushes a new service list when the host adds or removes a
service; the friend opens or closes listeners to match. A service that is
removed stops accepting new connections; streams already open are allowed to
finish.

Presence is real: the host counts a peer as live while its QUIC connection is
up and a ping has succeeded recently. No handshake timestamps, no guesses. The
friend shows the host as live only while connected.

## Security

### Identities

There are two kinds of long-lived key, and neither is ever sent anywhere.

- **Host identity, one per group.** Ed25519. Generated when the group is
  created, stored in an owner-only key file. Its public key hash is the pin
  friends check.
- **Device identity, one per group on each friend's machine.** Ed25519.
  Generated when the friend redeems an invite, stored in an owner-only key file.
  The host stores the public key as the member's identity.

Both sides present a self-signed certificate built from their key. The TLS
handshake proves possession of the private key. The host compares the client
certificate's public key with the member list; a mismatch is a new device and
needs approval.

### Invites

An invite is a random 160-bit token, written as `LP1-XXXX-XXXX-XXXX-XXXX-...`
and normalized on paste. The host keeps only a SHA-256 hash, never the token
itself.

- Single use.
- Expires 24 hours after it is created.
- Only redeemable while the host is running the session.
- The host validates the token in constant time and rate-limits failed attempts.
- On success the token is marked redeemed and deleted. On failure or denial it
  stays valid until it expires.
- The friend app discards the token after joining.

The invite carries the group id and name, the host's display name, the public
endpoint, the host's key pin and the token. It carries no private key.

### Joining and approval

1. The host creates an invite for a named friend.
2. The friend pastes it. Their app connects, verifies the host pin, presents its
   device certificate and sends the token with a device name.
3. The host shows an approval card: friend name, PC name, the address the host
   actually sees, and a short device fingerprint. The host allows or denies.
4. On allow, the host stores the device public key, marks the invite used, and
   sends the group and service list.
5. On later connections the host matches the device key. No invite is needed.

A connection from a known member with a different key is shown as a new device
for that member and requires approval. The host can replace the old key or deny
the request.

An invite always needs the host to be online. The host has to be present to
play, so this costs nothing in practice, and it makes the token burn
trustworthy.

### What the host sees

The host's roster can show, for each member:

- display name and PC name (reported by the friend's app)
- the public address the host sees for the connection (observed)
- a short device-key fingerprint (cryptographic)
- first seen and last seen

The data is host-only. It never goes into invites or to other friends. The
friend's app says what is shared on the join screen.

MAC addresses are not collected in v1. A MAC is self-reported, often
randomised, and never crosses the internet, so it looks like verification
without being one. IP and PC-name changes are shown as information, not used to
kick anyone. The key is the identity.

### Storage

| Data | Where | Mode |
| --- | --- | --- |
| Groups, services, members, public keys, invite hashes | `~/.local/share/lan-party/groups.json` | 0600 |
| Settings | `~/.config/lan-party/settings.json` | 0600 |
| Host and device private keys | `~/.local/share/lan-party/keys/` | 0600 files in a 0700 directory |

Writes are atomic: temp file in the same directory, then rename. Directories
are 0700. Nothing writes a private key or an invite token to JSON or logs.

There is no keyring daemon, session bus or unlock prompt. A key file that other
users can read is refused. The key files are not passphrase-encrypted, so
full-disk encryption is what protects a stolen disk; this is the same trade-off
as an SSH private key.

### Limits

- 16 services per group.
- 8 ports per service.
- 32 members per group.
- 128 concurrent TCP streams per peer.
- 64 UDP flows per peer.
- 60-second UDP idle timeout.
- Enrollment rate limit per source address.
- QUIC idle timeout with keepalive; the connection drops if the peer vanishes.

### Threat model

| Adversary | Mitigation |
| --- | --- |
| Internet scanner | QUIC/TLS hides everything before authentication. Unknown peers get no service list and no data. |
| Leaked invite | Single use, 24-hour expiry, host approval, rate limiting, revocable member. |
| Curious friend | Only published services exist. No routing, no peer-to-peer, no access to the host's other devices. |
| Malicious friend | Cannot reach anything else. Stream, flow and connection limits cap resource use. |
| Stolen device key | Can impersonate that member until revoked. A changed source address is visible; the host can revoke and issue a new invite. |
| Stolen host key | Can impersonate the host to friends who already have the pin. Owner-only file permissions and full-disk encryption are the protection. |
| Malicious host | The host is the gateway and can see what they publish. Documented, not hidden. |
| Same-user malware | Can read the key files, groups and settings as your user. Out of scope for a desktop app. |
| Supply chain | Few dependencies, pinned versions, `govulncheck` in CI, no runtime downloads. |

What the app cannot fix: the game server's own vulnerabilities. An offline-mode
Minecraft server is still an offline-mode Minecraft server.

## Data model

```
Group {
  id, name, role (host|friend), createdAt, hostName
  endpoint              // friend groups: public host:port
  services[]
  members[]
  invites[]             // host groups only
}

Service {
  id, name, proto (tcp|udp|both)
  target                // "192.168.1.50" or "192.168.1.50:25566"
  ports[]               // ports friends see
  hint                  // optional, e.g. "Direct Connect in Minecraft"
}

Member {
  id, name, deviceName
  publicKey             // base64, Ed25519
  fingerprint           // short display form
  firstSeen, lastSeen
  lastAddress
  state                 // approved | pending
}

Invite {
  id, name
  tokenHash             // SHA-256, base64
  createdAt, expiresAt, redeemedAt
}

Settings {
  closeToTray, startOnLogin, startMinimised
  approveNewDevices
  publicAddress          // host or host:port
  appPort                // default 51820
}
```

Private keys are not part of any struct that is written to disk. The store only
knows public keys and hashes.

## Interface

One window, 400×640, fixed. Content scrolls under a 48px app bar. The native
title bar stays; the app does not pretend to be frameless.

The working palette comes from the mockup: near-black background, slightly
raised cards, a cool cyan accent, green for a live link, amber for a warning,
red for danger. The final palette and logo are still being decided; the theme
is kept in one place so changing it does not touch screen code.

Screens:

- **Home.** Mark, name, one-line explanation, Create a group and Join with an
  invite, then saved groups. Each row shows host or joined, member count and a
  live light. A first-run link opens the guide. Delete is on host groups only,
  with a confirmation that ends the session and revokes everyone.
- **Host group.** Session panel with public address, app port, start/stop and
  connected count. Published services as cards, each showing the LAN target and
  the address friends open. People with approval state and a device-details
  view. Invite rows with copy and expiry.
- **Friend group.** Host status, then one card per published service with the
  loopback address, a copy button and the game's hint. People shows the host and
  you. Leave asks for confirmation.
- **Settings.** Startup (close to tray on by default; unticking makes closing
  the window quit, start on login, start minimised),
  Network (public hostname or IP, optional outside port in the address, app
  port, live endpoint preview), Security (approve new devices), About
  (version, data location, key location).
- **How to Use.** Offline guide with Hosting, Joining, Problems and Security
  sections. It reads the real app port and the host's local address, so the
  port-forward line is concrete, e.g. “Forward UDP 51820 to 192.168.1.20”, with a
  copy button. A longer version lives in `docs/guide.md`.
- **Dialogs.** Create group, join invite, add/edit service, device approval,
  confirm delete/revoke/leave.

Truthfulness rules:

- “Live” means a real connection and a recent ping.
- No invented reachability counts.
- Roster never shows the friend's local LAN IP; the host sees the connection
  address it actually has.
- An expired or redeemed invite is never offered for copy.

## Repository and history

The rewrite started on an orphan branch and its history now sits on the local
`main` branch. Its first commit is **Project Reset**. The old `main` and
`ui-redesign` branches remain as `legacy` and `ui-redesign`; nothing is
force-pushed to GitHub.

Layout:

```
cmd/lan-party/          entry point
internal/app/           wiring and application state
internal/config/        XDG paths and settings
internal/secure/        key files, identities, fingerprints
internal/group/         groups, services, presets, store
internal/invite/        one-time codes
internal/tunnel/        QUIC listener/dialer, TLS, control stream
internal/forward/       TCP streams and UDP flows
internal/session/       host and friend session lifecycle
internal/ui/            Fyne screens, theme, widgets, guide
internal/ui/assets/     fonts and icon
internal/autostart/     XDG autostart
internal/buildinfo/     version string
docs/                   design, architecture, security, UI, guide, development
build/                  AppImage and icon helpers
```

Build and run use the pinned toolchain in `.tools`; Go's cache, module cache,
GOPATH and XDG directories point into `.cache` and `.tmp` for development, so
nothing lands in the home directory. `make test`, `make run`, `make build`,
`make appimage`.

## Build order

1. Project reset, design document, README, mockup. (this commit)
2. Project skeleton, local build tooling, config paths, bare 400×640 window.
3. Secure owner-only key storage, host and device identities, fingerprints.
4. Groups, services, presets, validation and storage.
5. One-time invites, hashing, expiry.
6. QUIC listener and dialer, pinned host identity, control stream.
7. Enrollment, host approval, presence, revocation, session lifecycle.
8. TCP forwarding and loopback listeners.
9. UDP forwarding and flow table.
10. Interface shell, theme and components.
11. Home, host and friend screens; settings; in-app guide.
12. Tray, autostart and settings persistence.
13. AppImage packaging, CI, final documentation.

Each step ends with tests and at least one commit. The first vertical slice that
can be tried end to end is step 9 plus a minimal host/friend screen; the polish
comes after.

## Where it is now

Working on Linux: groups, services and presets; one-time invites; host approval
of new devices; QUIC/TLS sessions; TCP and UDP forwarding; presence, service
updates and revocation; the 400×640 interface; the in-app setup guide; tray,
start on login and start minimised; AppImage packaging; CI.

The end-to-end tests start a host and a friend app on loopback, create an
invite, approve the device, forward a TCP echo, and revoke the member.

Not in this version: a relay for CGNAT hosts, per-friend service grants,
Windows packaging, automatic public-address detection, MAC addresses in device
details, and auto-update.

## Open items

- Final logo and palette. The mockup palette is the working baseline.
- Whether to show a reported MAC in device details later.
- Public-address detection. v1 is manual, with the local address shown to help
  with the router rule.
- Per-friend service grants.
- A self-hosted relay for CGNAT hosts.
- Windows packaging.
