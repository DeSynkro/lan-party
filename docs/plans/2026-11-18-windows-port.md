# Windows port

Goal: ship LAN Party 1.1 on Windows 10 and 11, x86-64, with both the host and
friend roles. Linux stays the reference build.

## Where we are

The Go backend is close to portable. A Windows build currently stops at
`internal/autostart`, which only has a Linux file. `syscall.EADDRINUSE` exists
on Windows, and `godbus` and `fyne.io/systray` build for Windows. The unknown is
the GTK4 UI: it uses gotk4 and cgo, and there is no Windows build yet.

The Linux-specific surface:

- `config.Resolve` uses XDG paths.
- `autostart` writes a `.desktop` file.
- `tray.go` checks the D-Bus StatusNotifier watcher and imports godbus.
- Tests assume XDG paths, `.desktop` autostart and Unix file modes.
- `secure` uses `0600`/`0700` modes.
- `build/appimage.sh` is Linux-only; there is no Windows package.
- CI is Linux-only.

## Decisions

1. **UI:** spike GTK4 on Windows through MSYS2 first. If it builds, launches and
   looks acceptable, reuse the current UI. If it does not, use a native Win32
   front end with the same Go backend. Do not maintain two UIs unless the spike
   fails.
2. **Paths:** `%AppData%\lan-party` for settings; `%LocalAppData%\lan-party` for
   groups and keys. Linux keeps XDG.
3. **Autostart:** the HKCU
   `Software\Microsoft\Windows\CurrentVersion\Run` key, using `os.Executable()`.
4. **Tray:** `fyne.io/systray` native. The D-Bus availability check is
   Linux-only; Windows assumes a tray.
5. **Single instance:** test GApplication on Windows. If it does not enforce a
   single instance, add a named mutex.
6. **File protection:** the per-user profile ACLs protect the data folder for
   1.1. DPAPI-encrypting the key files is later hardening, not a 1.1 blocker.
7. **Packaging:** portable zip first, with the exe, GTK runtime DLLs and
   `share/lan-party` assets. Installer and code signing after the first real
   test.
8. **Firewall:** the first host run triggers the Windows Defender prompt.
   Document allowing UDP on private networks; the app cannot safely add a rule
   without elevation.
9. **Console:** keep a console debug exe for logs. The release exe uses
   `-H=windowsgui`.

## What can be done without a Windows PC

- Add `autostart_windows.go` with the Run key and split the Linux autostart
  test.
- Split `config.Resolve` into Linux and Windows files, with per-OS tests.
- Split the tray notifier check; keep the systray code common.
- Make the `secure` tests tolerate Windows ACLs and document the protection.
- Add build-tagged tests for XDG paths, `.desktop` autostart and Unix modes.
- Add a `windows` CI job on `windows-latest` with MSYS2 GTK4; compile the UI and
  upload a portable zip.
- Add `build/windows.sh` to assemble the zip, copy the GTK DLLs, assets and
  licence files.
- Update the README, guide and security notes for Windows paths and the
  firewall prompt.

## What needs a Windows PC

- Launch and use the UI: window, fonts, CSS, drag, tray, settings.
- Single-instance behaviour.
- Autostart at login.
- Windows Defender prompt and host reachability.
- Installer and SmartScreen.
- A real host/friend test: Windows host with a Linux friend, then the reverse.
- The code-signing decision.

## Workstreams

### A. Cross-platform foundation

- `paths_linux.go` / `paths_windows.go`.
- `autostart_linux.go` / `autostart_windows.go`, plus per-OS tests.
- `notifier_linux.go` / `notifier_windows.go`; shared systray code.
- `secure` tests and permission documentation.
- Build-tagged tests that currently assume XDG, `.desktop` and Unix modes.

### B. Windows CI compile spike

- MSYS2 with `mingw-w64-x86_64-gtk4`, `mingw-w64-x86_64-gobject-introspection`,
  `mingw-w64-x86_64-pkgconf` and `mingw-w64-x86_64-gcc`.
- Use the pinned Go toolchain with `CC=x86_64-w64-mingw32-gcc`,
  `CGO_ENABLED=1` and `PKG_CONFIG_PATH=/mingw64/lib/pkgconfig`.
- Build `lan-party.exe`; the first answer is whether the GTK UI compiles.
- Upload a portable zip as a temporary artifact for the first real test.

### C. Windows runtime

- Window and UI appearance, fonts and CSS.
- Tray behaviour.
- Single instance.
- Autostart.
- Defender firewall prompt.
- Data and key paths.
- Asset root next to the exe.

### D. Packaging and release

- Portable zip first.
- Installer (Inno Setup or NSIS).
- Code signing and SmartScreen.
- Windows artefact in the tag workflow and release notes.

### E. Testing

- CI compile and headless tests.
- Wife's Windows PC: launch, tray, autostart, firewall.
- Cross-platform host/friend with a Linux machine.
- Windows 10 and 11.

## Order

1. Roadmap and backlog cleanup.
2. This audit.
3. Cross-platform foundation.
4. Windows CI compile spike.
5. Portable zip and the first launch on the Windows PC.
6. Runtime fixes.
7. Installer and code signing.
8. 1.1 release.

## Open questions

- Is a portable zip acceptable for the first test, or is an installer needed
  immediately? (Recommend zip first.)
- Do we want code signing for 1.1, or ship unsigned with a warning? (Recommend
  deciding after the first real test.)
- Windows 10 and 11 x86-64 only for 1.1? (Recommend yes.)
- If GTK4 on Windows looks bad or the bundle is too large, do we accept a native
  Win32 UI as a larger project? (Recommend deciding after the spike.)
