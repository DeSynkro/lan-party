# Changelog

## 1.2.0 — 2026-11-22

- Optional self-hosted relay for hosts behind CGNAT, a VPN, or a network where a
  UDP port cannot be forwarded. Direct mode remains the default.
- The relay is an opaque IPv4/IPv6 UDP forwarder. Host/friend QUIC/TLS and the
  host pin stay end to end; the relay never reads game traffic.
- Relay ID pin, host-only password, short-lived first-join tokens and hashed
  single-use tokens.
- Per-device relay reconnect tokens, issued after the first approved join and
  stored owner-only. Friends reconnect through the relay without a new invite
  after network drops, host session restarts and relay restarts.
- Host relay recovery uses a 5s/10s/30s retry schedule for about 16 minutes.
- Relay data-plane idling: UDP is closed while no host session exists; only the
  TLS control port remains.
- Relay inactive-friend cleanup for relay groups, with a configurable window,
  a default of 5 days on new installs, and a blank/never option. Live friends
  are never removed.
- Member reconnect reset and revocation.
- Docker Compose and native Linux relay packaging, including a hardened systemd
  unit and a no-root console runner. The console runner keeps its data in one
  user-owned directory and stops with the terminal.
- `set-public` relay command for changing the advertised host/port without
  rotating the identity.
- Linux and Windows app builds updated with relay settings, group Override
  Settings, friend reconnect status, inactivity deadlines and improved error
  wrapping.

See [docs/releases/v1.2.0.md](docs/releases/v1.2.0.md) for the release notes.

## 1.1.0 — 2026-11-20

- Windows 10/11 x86-64 portable build with a GUI executable, bundled GTK4
  runtime, tray, start on login and start minimised.
- Host groups can use their own public address and UDP app port. Blank values
  use the Settings defaults.
- Changing a live group's app port stops the session, saves, and restarts on
  the new port. The dialog warns that everyone disconnects briefly.
- Invites now expire after 1 hour instead of 24 hours.
- Friend connections retry automatically after a drop for up to 30 minutes.
  This can be turned off in Settings.
- The app writes a rotating log under its data directory for support.
- The portable Windows zip contains one GUI executable. The console debug build
  is a separate manual CI artifact.
- Joining friends see the host-assigned name and a read-only detected device
  name. Host member rows show the device name as a tag.
- Creating an invite starts the host session first when it is off. A session
  for another group blocks the invite instead of being ended silently.
- New services default to `127.0.0.1`, accept `localhost`, and explain when to
  use a LAN IP instead.
- Published services can be edited without removing and recreating them.
- The endpoint-change warning only appears when an invite is open.

See [docs/releases/v1.1.0.md](docs/releases/v1.1.0.md) for the release notes.

## 1.0.0 — 2026-11-18

First stable release.

- Linux x86-64 AppImage built on Ubuntu 22.04 for GTK 4.6, GLib 2.72,
  glibc 2.34 and the x86-64 baseline ISA.
- Host and friend groups, TCP and UDP services, and optional port remapping.
- One-time invites, device approval, live presence and revocation.
- 16 game presets, including rAthena Ragnarok Online.
- Fixed 400×640 GTK4 interface, tray, start on login and start minimised.
- No virtual network, no accounts and no telemetry.

See [docs/releases/v1.0.0.md](docs/releases/v1.0.0.md) for the full release
notes.
