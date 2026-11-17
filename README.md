# LAN Party

A small Linux desktop app that gives a friend one local port onto a game server
on your LAN, and nothing else.

You run the game server. You add it to a group as a service, choose the port
friends should see, and start a session. A friend pastes a one-time invite; you
approve their device once. Their game then connects to `127.0.0.1:25565` as
though the server were on their own machine.

The traffic crosses one encrypted QUIC connection to your PC, where the app
dials the real LAN target. There is no virtual network, no route, and no
peer-to-peer path. Your router, your NAS and the rest of your LAN are never
reachable from the friend's machine.

## Status

Working on Linux:

- host and friend groups, saved locally
- any TCP and/or UDP service the host chooses, with an optional port remap
- one-time invites, hashed at rest, 24-hour expiry
- host approval for new devices and for changed device keys
- presence, live service updates and revocation
- a fixed 400×640 interface, tray icon, start on login and start minimised
- AppImage packaging and CI

Not in this version: a relay for CGNAT hosts, per-friend service grants,
Windows packaging, automatic public-address detection and auto-update.

The design is in [docs/design.md](docs/design.md). If you just want to use it,
read [docs/guide.md](docs/guide.md).

## What it needs

- Linux with X11 or Wayland and working OpenGL.
- A Secret Service keyring (GNOME Keyring, KWallet, or another provider) for
  the long-lived keys.
- One UDP port forwarded on the host's router. Friends do not need a forward.
- A public address that reaches the host: a hostname, a static IP, or a
  dynamic DNS name.

## Build and run

```sh
make run       # build and open the app with project-local config
make test      # unit and end-to-end tests
make build     # binary in bin/
make appimage  # Linux AppImage in dist/
```

The first run downloads the pinned Go toolchain into `.tools`. Build caches,
the module cache and development config stay in `.cache` and `.tmp`; nothing is
installed system-wide. Release builds use the system Go and the pinned
`appimagetool`.

At runtime the app writes:

- `~/.config/lan-party/settings.json`
- `~/.local/share/lan-party/groups.json`
- `~/.config/autostart/lan-party.desktop` when Start on login is on
- long-lived private keys in the OS keyring

Both JSON files are owner-only (`0600`). Invites contain no private keys.

The new group file is not compatible with the old WireGuard version. If an old
`groups.json` is still in the data folder, the app says so and refuses to
overwrite it. Move the old `~/.local/share/lan-party` folder aside and start
fresh.

## Docs

- [docs/guide.md](docs/guide.md) — setup and troubleshooting
- [docs/architecture.md](docs/architecture.md) — how a connection moves
- [docs/security.md](docs/security.md) — identities, invites, threat model
- [docs/ui.md](docs/ui.md) — the 400×640 rules and screens
- [docs/development.md](docs/development.md) — layout, build, tests, release
- [docs/design.md](docs/design.md) — the rewrite design
- [docs/mockups/ui-v1.html](docs/mockups/ui-v1.html) — the reference mockup

## Licence

GPLv3. See [LICENSE](LICENSE).

Built by [DeSynkro](https://desynkro.com).
