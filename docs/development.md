# Development

## Layout

```
cmd/lan-party/          main package
internal/app/           groups, settings, invites, approval, sessions
internal/autostart/     XDG autostart entry
internal/buildinfo/     version string
internal/config/        XDG paths and settings.json
internal/forward/       TCP pipes and UDP flows
internal/group/         group, service, member, invite, store
internal/invite/        one-time tokens and pasteable codes
internal/preset/        shipped game ports
internal/secure/        key files, identities, certificates, pins
internal/session/       host and friend sessions
internal/tunnel/        QUIC, TLS pinning, control stream
internal/gtkui/         GTK4 interface, style.css and UI assets
build/                  AppImage helpers
docs/                   design, architecture, security, UI, guide
docs/plans/             historical rewrite plan
```

## Build

```sh
make run
make test
make build
make appimage
```

Building needs GTK4 and GLib 2.84+ development headers (`libgtk-4-dev`,
`libglib2.0-dev` and `gobject-introspection` on Debian/Ubuntu), a C compiler
and `pkg-config`. The first build also installs the Go version from
`.go-version` into `.tools/go`. `make env` prints the paths. Everything stays
inside the project:

| Thing | Where |
| --- | --- |
| Go toolchain | `.tools/go` |
| AppImage tool | `.tools/appimagetool` |
| Go build cache | `.cache/go-build` |
| Go module cache | `.cache/go-mod` |
| Development config/data/cache | `.tmp/config`, `.tmp/data`, `.tmp/cache` |
| Binary | `bin/lan-party` |
| AppImage | `dist/` |

No global `go install`, npm or package is used.

Dependencies are GTK4 through gotk4, quic-go for the encrypted transport and
systray for the tray icon. There is no webview, no bundled browser, no npm tree
and no keyring daemon.

## Tests

```sh
make test
make test-race
make vuln
```

The tests cover:

- settings defaults, validation and file permissions
- key file identity creation, permissions, certificates, pins and fingerprints
- groups, services, target validation, port conflicts, invites and the store
- invite encoding, expiry, normalization and tampering
- QUIC handshake, pin mismatch, denial, control messages, streams and datagrams
- TCP and UDP forwarding, loopback fallback and half-close
- host/friend sessions, presence, service updates, revocation and resume
- the app policy: invite redemption, approval, new devices, rate limiting
- an end-to-end app test: host creates a group, invite, approval, friend joins,
  TCP echo, revoke
- GTK service-form composition, live-refresh signatures and CSS colour checks

The end-to-end app test uses in-memory key stores. Tests never write to the
real data or key directories.

### UI previews

```sh
make gtk-probe
```

The probe uses the GTK interface and the project-local XDG directories. To open
a screen directly, set `LANPARTY_GTK_SCREEN`:

```sh
LANPARTY_GTK_SCREEN=group:<group-id> make gtk-probe
LANPARTY_GTK_SCREEN=service:<group-id> make gtk-probe
```

Supported values include `settings`, `guide`, `create`, `join`, `group:<id>`,
`invite:<id>`, `service:<id>`, `service-detail:<id>`, `member:<id>` and
`delete:<id>`. `LANPARTY_GTK_APP_ID` lets a preview instance run beside the
normal app.

## Styling

`internal/gtkui/style.css` is the source of truth for colours, spacing, type
sizes and component styling. Brand fonts and PNGs live in `brand/`; interface
fonts and icons live in `internal/gtkui/assets/`.

The app loads `style.css` at startup and finds the asset root from
`LANPARTY_ASSET_ROOT`, the current directory, or the executable directory. The
AppImage uses the executable-relative lookup. `docs/tuner.html` is retained as
a historical design reference; it is not connected to the GTK renderer.

## Runtime data

Production:

- `~/.config/lan-party/settings.json`
- `~/.local/share/lan-party/groups.json`
- `~/.config/autostart/lan-party.desktop` when Start on login is on
- host and device private keys as `0600` files in
  `~/.local/share/lan-party/keys/`

Development sets `XDG_CONFIG_HOME`, `XDG_DATA_HOME` and `XDG_CACHE_HOME` to
`.tmp`, so running `make run` does not touch your real config.

### Migrating from the first version

The new app uses a versioned group file. The old app wrote a plain JSON array
of WireGuard groups in the same `~/.local/share/lan-party` directory. There is
no migration: the identities and invite format changed. If the old file is
still there, the new app shows a clear error and does not overwrite it. Move
the old folder aside (`mv ~/.local/share/lan-party ~/.local/share/lan-party-old`)
and start fresh. The old code and design are kept in the `legacy` branch.

The earlier build of this rewrite kept keys in the OS keyring. This build stores
them in `keys/` and does not import keyring entries. Delete a group created by
that build and create it again; the app will not silently replace a missing
identity.

## Packaging

`build/appimage.sh` assembles an AppDir with the binary, brand assets, GTK
styles, desktop file and icon, downloads a pinned `appimagetool` into `.tools`,
verifies its checksum and writes
`dist/lan-party_<version>_linux_amd64.AppImage`.

The AppImage currently uses the system GTK4 runtime. It builds on every push
to main and on tags; the artifact is retained for 14 days. Release builds use
the Ubuntu 26.04 runner, which has the GLib 2.84+ headers required by gotk4.
Bundling GTK and lowering the runtime GLib floor remain separate packaging
decisions.

## Releases

CI runs vet, race tests, `govulncheck` and a binary build on every push. The
Linux AppImage is built on a tag or by hand. A tag also creates a draft release
with the AppImage and `SHA256SUMS`.

Version strings come from `git describe` and are passed to
`internal/buildinfo.Version` with `-ldflags`. Untagged builds say `dev`.

## Style

- Small commits with past-tense messages: “Added…”, “Fixed…”, “Removed…”.
- Keep packages focused; avoid a package that only wraps another.
- No silent fallbacks for secrets. A key file error is an error.
- No invented state in the interface. A live light means a real connection.
- Prefer standard library crypto and the QUIC/TLS stack. Do not hand-roll a
  cipher or a certificate chain.
- Keep the build, caches and generated files inside the repository.

## Windows

The code is written so the UI and group logic are not tied to Linux. The
remaining work is a Windows autostart entry, a MinGW build and a tray check.
It is not part of this version.

## Design and plan

- [design.md](design.md) — the rewrite decisions
- [plans/2026-11-17-rewrite.md](plans/2026-11-17-rewrite.md) — the task-by-task
  plan used while building
- [architecture.md](architecture.md) — how it works
- [security.md](security.md) — what is protected
- [ui.md](ui.md) — the interface rules
