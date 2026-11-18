# Windows foundation implementation plan

Goal: make the Go backend build and test on Windows, split the Linux-only
pieces behind build tags, and add a Windows CI spike that builds the GTK UI.

Spec: `docs/plans/2026-11-18-windows-port.md`

## Constraints

- Linux behaviour stays unchanged.
- Windows 10/11 x86-64 is the target.
- No new UI work in this slice. The GTK UI compile is the spike.
- The workspace and test commands stay project-local.
- Commit messages are past tense.

## Review focus

1. Windows paths never fall back to `.config` or `.local/share`.
2. The Windows autostart entry is removable and does not break when the Run key
   is missing.
3. The tray is enabled on Windows without a D-Bus check.
4. Windows key files do not fail the Unix `0600` check; profile ACLs are the
   protection.
5. The Linux path, autostart, tray and permission tests still pass.
6. The Windows CI job actually compiles the gotk4/GTK4 UI, not just the backend.

## Task 1: Platform paths

**Files:**
- Modify: `internal/config/paths.go`
- Create: `internal/config/paths_linux.go`
- Create: `internal/config/paths_windows.go`
- Move: `internal/config/paths_test.go` → `internal/config/paths_linux_test.go`
- Create: `internal/config/paths_windows_test.go`

**Interfaces:**
- `config.Paths{ConfigDir, DataDir string}`
- `config.Resolve() Paths` remains the only caller-facing API.

- [ ] Keep the `Paths` type in `paths.go`; move `Resolve` to the OS files.
- [ ] Linux keeps XDG rules.
- [ ] Windows uses `os.UserConfigDir()` for settings and `os.UserCacheDir()` for
  data and keys.
- [ ] Add `//go:build linux` to the moved Linux test file and a Windows test
  that sets `AppData` and `LocalAppData`.
- [ ] Run `go test ./internal/config` on Linux.
- [ ] Commit.

## Task 2: Windows autostart

**Files:**
- Create: `internal/autostart/autostart_windows.go`
- Move: `internal/autostart/autostart_test.go` →
  `internal/autostart/autostart_linux_test.go`
- Create: `internal/autostart/autostart_windows_test.go`

**Interfaces:**
- `autostart.Set(configHome string, enabled bool) error` stays the same.

- [ ] Windows writes `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
  value `LAN Party` as the quoted executable path.
- [ ] Disabling removes the value; a missing key or value is not an error.
- [ ] Add a test helper that uses a test registry key, so the tests never touch
  the real Run key.
- [ ] Run the Linux autostart tests; the Windows test compiles for the CI job.
- [ ] Commit.

## Task 3: Windows tray notifier

**Files:**
- Modify: `internal/gtkui/tray.go`
- Create: `internal/gtkui/notifier_linux.go`
- Create: `internal/gtkui/notifier_windows.go`

- [ ] Move `statusNotifierAvailable` and the godbus import out of `tray.go`.
- [ ] Linux keeps the StatusNotifier check; Windows returns true.
- [ ] Run `go test ./internal/gtkui` on Linux.
- [ ] Commit.

## Task 4: Windows test and key-permission compatibility

**Files:**
- Modify: `internal/secure/file.go`
- Create: `internal/secure/check_unix.go`
- Create: `internal/secure/check_windows.go`
- Modify: `internal/secure/file_test.go`
- Modify: `internal/config/settings_test.go`
- Modify: `internal/group/store_test.go`
- Modify: `internal/app/app_test.go`
- Modify: `internal/gtkui/assets_test.go`

- [ ] Move the owner-only check behind `checkKeyFile(path, info)`: Unix keeps
  the `0600` check; Windows accepts the file because the per-user profile ACLs
  protect it.
- [ ] Skip the Unix mode assertions on Windows in the settings, store, app and
  secure tests.
- [ ] Replace `os.MkdirTemp("/tmp", ...)` with a portable temp directory.
- [ ] Run the Linux suite.
- [ ] Commit.

## Task 5: Windows CI compile spike

**Files:**
- Create: `.github/workflows/windows-spike.yml`

- [ ] Add MSYS2 with GTK4, gobject-introspection, pkgconf and GCC.
- [ ] Build `lan-party.exe` with the pinned Go toolchain and
  `CC=x86_64-w64-mingw32-gcc`.
- [ ] Run the backend tests that do not need a display.
- [ ] Upload the exe as a temporary artifact.
- [ ] Trigger on `workflow_dispatch` and pushes to `windows-spike`, not every
  main push.
- [ ] Commit.

## Task 6: Verification

```sh
make fmt
make vet
make test-race
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  ./internal/app ./internal/config ./internal/group ./internal/invite \
  ./internal/secure ./internal/session ./internal/tunnel ./internal/forward \
  ./internal/preset
```

- [ ] Linux suite green.
- [ ] Windows backend cross-compile green.
- [ ] `GOOS=windows go test -c` compiles the Windows tests for the backend
  packages.
- [ ] Commit any remaining changes.

## After the spike

The Windows workflow answers whether gotk4/GTK4 compiles. If it does, the next
slice is the portable zip with the GTK runtime and assets. If it does not, the
UI decision moves to a native Win32 front end.
