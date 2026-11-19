# Changelog

## 1.1.0 — Unreleased

- Host groups can use their own public address and UDP app port; blank values
  use the Settings defaults.
- Changing a live group's app port stops the session, saves, and restarts on
  the new port. The dialog warns that everyone disconnects briefly.
- Invites now expire after 1 hour instead of 24 hours.
- Creating an invite starts the host session first when it is off; a session
  for another group blocks the invite instead of being ended silently.
- New services default to `127.0.0.1`, accept `localhost`, and explain when to
  use a LAN IP instead.
- Published services can be edited without removing and recreating them.

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
