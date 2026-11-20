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

Building needs GTK4 and GLib 2.72+ development headers (`libgtk-4-dev`,
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
sizes and component styling. Brand fonts and PNGs live in `brand/`. Interface
fonts and icons live in `internal/gtkui/assets/`.

The app loads `style.css` at startup and finds the asset root from
`LANPARTY_ASSET_ROOT`, the current directory, or the executable directory. The
AppImage uses the executable-relative lookup. `docs/tuner.html` is retained as
a historical design reference. It is not connected to the GTK renderer.

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
that build and create it again. The app will not silently replace a missing
identity.

## Packaging

`build/appimage.sh` assembles an AppDir with the binary, brand assets, GTK
styles, desktop file and icon, downloads a pinned `appimagetool` into `.tools`,
verifies its checksum and writes
`dist/lan-party_<version>_linux_amd64.AppImage`.

The AppImage currently uses the system GTK4 runtime. It builds on every push
to main and on tags. The artifact is retained for 14 days. Builds use the
Ubuntu 22.04 runner and gotk4 v0.2.2, which sets the runtime floor at GTK 4.6,
GLib 2.72, GLIBC_2.34 and the x86-64 baseline ISA. Each build is launched under
Xvfb on that floor before it is uploaded. Bundling GTK remains a separate
packaging decision.

## Releases

CI runs vet, race tests, `govulncheck` and a binary build on every push. The
Linux AppImage is built on every push to main, on tags and by hand, then smoke
tested under Xvfb. The Windows portable zip is built in the same workflow on
Windows and is uploaded as the `windows-portable` artifact. A tag also creates
a draft release with the AppImage, Windows zip and `SHA256SUMS`. If
`docs/releases/<tag>.md` exists, it becomes the release body. Otherwise the
notes are generated. The AppImage installs the GPLv3 license, font licenses and
third-party notices under `usr/share/licenses/lan-party/`.

Version strings come from `git describe` and are passed to
`internal/buildinfo.Version` with `-ldflags`. Untagged local builds say `dev`.
CI uses `dev-<sha>` for branches.

### Tagging a release

1. Make sure `main` is clean and the tests pass.
2. Add or update `docs/releases/<tag>.md`. CI uses it as the release body.
3. Create an annotated tag:

   ```sh
   git tag -a v1.1.0 -m "LAN Party 1.1.0"
   ```

4. Push `main` first, then the tag:

   ```sh
   git push github main
   git push github v1.1.0
   ```

5. CI builds the AppImage and Windows zip, writes `SHA256SUMS`, and creates a
   **draft** release.
6. Open the draft on GitHub, check the notes and all assets, then publish it.

Never move or delete an existing release tag. If a build is wrong, fix it and
publish a new patch version instead.

## Style

- Small commits with past-tense messages: “Added…”, “Fixed…”, “Removed…”.
- Keep packages focused. Avoid a package that only wraps another.
- No silent fallbacks for secrets. A key file error is an error.
- No invented state in the interface. A live light means a real connection.
- Prefer standard library crypto and the QUIC/TLS stack. Do not hand-roll a
  cipher or a certificate chain.
- Keep the build, caches and generated files inside the repository.

## Windows

The Windows build is a portable zip, not an installer. It is built on the
Windows runner with MSYS2 UCRT64:

```sh
# Inside an MSYS2 UCRT64 shell
export CC=gcc
export CGO_ENABLED=1
export PKG_CONFIG_PATH=/ucrt64/lib/pkgconfig
export VERSION=dev
bash build/windows.sh
```

`build/windows.sh` needs GTK4, `gcc`, `binutils` (`windres`), `pkg-config` and
`zip`. It embeds the app icon, version information and manifest into the
executable, then writes
`dist/lan-party_<version>_windows_amd64.zip`. The release archive contains the
GUI executable only and is uploaded as the `windows-portable` CI artifact.

The **Windows debug build** workflow is manual-only. It sets
`INCLUDE_CONSOLE=1`, adds `lan-party-console.exe`, writes
`dist/lan-party_<version>_windows_debug_amd64.zip`, and uploads it as
`windows-debug`. Use it when a user needs startup logs beyond the normal log
file.

The app writes `lan-party.log` under its data directory:
`%LocalAppData%\lan-party\lan-party.log` on Windows and
`~/.local/share/lan-party/lan-party.log` on Linux. Logs rotate at 5 MB,
keeping one previous file.

Code signing is not configured yet. Unsigned builds can trigger SmartScreen or
a Defender false positive. A certificate or Azure Trusted Signing is the
long-term fix.

## Design and plan

- [design.md](design.md) — the rewrite decisions
- [plans/2026-11-17-rewrite.md](plans/2026-11-17-rewrite.md) — the task-by-task
  plan used while building
- [architecture.md](architecture.md) — how it works
- [security.md](security.md) — what is protected
- [ui.md](ui.md) — the interface rules
