# Features to be added

A lightweight list of ideas that are not scheduled. Nothing here is a
commitment.

## Friend-side bind address

**Status:** deferred — not planned for 1.2. Revisit if a game rejects `127.0.0.1`.

The friend app always binds forwarded services to `127.0.0.1`. A few games may
reject localhost or expect a LAN address. Add an advanced option to choose
where the friend-side listener binds:

- `127.0.0.1` (default)
- another loopback address such as `127.0.0.2`
- the friend PC's LAN IP, with a clear warning that other devices on that LAN
  can reach the forwarded port while the session is live

The friend service card should show the actual address that was bound. The
host's "Friends open 127.0.0.1:..." text would need to become less specific or
show the configured value.

Caveats:

- Binding a LAN IP exposes the tunneled game port to the friend's network.
- This does not forward LAN discovery broadcasts or multicast. Games that only
  list servers through LAN discovery still need a direct-connect option.
- A virtual LAN interface is out of scope. It needs privileges and
  platform-specific setup.

Done when a friend can set a different local address for a service or for all
services, see the address in the UI, and still get the default loopback-only
behaviour unless they opt in.

## Single-file Windows launcher

**Status:** idea, not scheduled

The Windows build is portable but is a folder containing the exe, GTK DLLs,
pixbuf loaders, schemas, icons and assets. A small Go launcher can embed that
folder as a zip, extract it once to
`%LocalAppData%\lan-party\runtime\<version>`, and start `lan-party.exe` with the
GTK data, pixbuf and working-directory environment.

- One file to download and run.
- The runtime still exists under AppData, but the user only sees one exe.
- Reuse the extracted runtime on later runs. Remove old versions when the
  launcher version changes.
- A 7-Zip SFX is an alternative. The Go launcher is preferred for versioning,
  cleanup, error dialogs and signing.

Trade-offs:

- The first run is slower while it extracts.
- The launcher is large because it contains the runtime.
- Antivirus and SmartScreen can still flag self-extracting launchers,
  especially unsigned.
- An installer remains a separate option if installing into Program Files is
  acceptable.

Done when a signed single exe runs the portable app, reuses the extracted
runtime, and cleans up old versions.

## Multiple live host sessions

**Status:** idea, not scheduled. Revisit if users need to host two groups at
once.

Today the app has one live session slot, either one host group or one friend
group. Per-group connection settings let each group advertise a different
address and port, but they do not allow concurrent sessions.

Allow more than one host group to run at the same time, each on a different
local UDP app port. The public IP can stay the same if the router forwards each
outside port to the matching local port.

This is a structural change:

- replace the single host pointer and live group id with a collection
- add per-group start, stop, presence and approvals
- show multiple live groups in the UI and tray
- decide whether a host and a friend session may coexist
- add firewall/router guidance for each port

Done when two host groups can be live with independent invites and friends, and
stopping one leaves the other running.

## Windows code signing

**Status:** post-1.2, not scheduled

The 1.2 Windows build is unsigned. SmartScreen and Defender can warn about a
new portable executable with no reputation.

- Use an OV/EV code-signing certificate or Azure Trusted Signing.
- Sign the GUI executable, and the debug console executable when it is built,
  before the zip is created.
- Keep the certificate in CI secrets and timestamp the signature.

Done when a signed release shows the publisher in Windows and a clean machine
does not show a SmartScreen warning.

## Done

- Self-hosted relay with Docker Compose, native systemd and a no-root console
  runner (1.2).
- Relay reconnect tokens, host relay recovery and inactive-friend cleanup
  (1.2).
- Idle relay data plane that closes UDP while no host session exists (1.2).
- Per-group public address and app port (1.1).
- One-hour invite lifetime (1.1).
- Windows portable build with tray, autostart and logging (1.1).
- Automatic friend reconnect and service editing (1.1).
