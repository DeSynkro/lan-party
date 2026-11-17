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
internal/secure/        keyring, identities, certificates, pins
internal/session/       host and friend sessions
internal/tunnel/        QUIC, TLS pinning, control stream
internal/ui/            Fyne theme, widgets and screens
build/                  AppImage and icon helpers
docs/                   design, architecture, security, UI, guide
docs/plans/             the rewrite implementation plan
```

## Build

```sh
make run
make test
make build
make appimage
```

The first build installs the Go version from `.go-version` into `.tools/go`.
`make env` prints the paths. Everything stays inside the project:

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

Dependencies are Fyne for the window, quic-go for the encrypted transport,
and go-keyring for the OS secret store. There is no webview, no bundled browser
and no npm tree.

## Tests

```sh
make test
make test-race
make vuln
```

The tests cover:

- settings defaults, validation and file permissions
- keyring identity creation, certificates, pins and fingerprints
- groups, services, target validation, port conflicts, invites and the store
- invite encoding, expiry, normalization and tampering
- QUIC handshake, pin mismatch, denial, control messages, streams and datagrams
- TCP and UDP forwarding, loopback fallback and half-close
- host/friend sessions, presence, service updates, revocation and resume
- the app policy: invite redemption, approval, new devices, rate limiting
- an end-to-end app test: host creates a group, invite, approval, friend joins,
  TCP echo, revoke
- UI screen text and a render smoke test

The end-to-end app test uses in-memory keyrings. The real keyring is only used
at runtime, so tests never write to the user's keyring.

### UI previews

```sh
mkdir -p .tmp/preview
LANPARTY_PREVIEW="$PWD/.tmp/preview" go test ./internal/ui -run TestPreviewScreens -v
```

This writes 400×600 PNGs for Home, the host group, the friend screen, Settings
and the guide.

## Runtime data

Production:

- `~/.config/lan-party/settings.json`
- `~/.local/share/lan-party/groups.json`
- host and device private keys in the OS keyring

Development sets `XDG_CONFIG_HOME`, `XDG_DATA_HOME` and `XDG_CACHE_HOME` to
`.tmp`, so running `make run` does not touch your real config.

### Migrating from the first version

The new app uses a versioned group file. The old app wrote a plain JSON array
of WireGuard groups in the same `~/.local/share/lan-party` directory. There is
no migration: the identities and invite format changed. If the old file is
still there, the new app shows a clear error and does not overwrite it. Move
the old folder aside (`mv ~/.local/share/lan-party ~/.local/share/lan-party-old`)
and start fresh. The old code and design are kept in the `legacy` branch.

## Packaging

`build/appimage.sh` assembles an AppDir with the binary, desktop file and icon,
downloads a pinned `appimagetool` into `.tools`, verifies its checksum and
writes `dist/lan-party_<version>_linux_amd64.AppImage`.

`build/icon/main.go` redraws the placeholder keyhole icon with `make icon`.
The final logo is still being designed; the generator is only a placeholder.

## Releases

CI runs vet, race tests, `govulncheck` and a binary build on every push. The
Linux AppImage is built on a tag or by hand. A tag also creates a draft release
with the AppImage and `SHA256SUMS`.

Version strings come from `git describe` and are passed to
`internal/buildinfo.Version` with `-ldflags`. Untagged builds say `dev`.

## Style

- Small commits with past-tense messages: “Added…”, “Fixed…”, “Removed…”.
- Keep packages focused; avoid a package that only wraps another.
- No silent fallbacks for secrets. A keyring error is an error.
- No invented state in the interface. A live light means a real connection.
- Prefer standard library crypto and the QUIC/TLS stack. Do not hand-roll a
  cipher or a certificate chain.
- Keep the build, caches and generated files inside the repository.

## Windows

The code is written so the UI and group logic are not tied to Linux. The
remaining work is a platform keyring implementation, a Windows autostart entry,
a MinGW build and a tray check. It is not part of this version.

## Design and plan

- [design.md](design.md) — the rewrite decisions
- [plans/2026-11-17-rewrite.md](plans/2026-11-17-rewrite.md) — the task-by-task
  plan used while building
- [architecture.md](architecture.md) — how it works
- [security.md](security.md) — what is protected
- [ui.md](ui.md) — the interface rules
