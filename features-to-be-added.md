# Features to be added

A lightweight list of ideas that are not scheduled. Nothing here is a
commitment.

## Relay mode

**Status:** idea, not scheduled

The host currently needs one forwarded UDP port and a reachable public
address. That does not work behind CGNAT or on networks where the router
cannot be configured.

Add an optional **self-hosted** relay mode. The project does not run a
public relay; the user runs one on their own VPS or spare machine, similar
to self-hosting RustDesk.

- A **Use relay** checkbox when hosting.
- Fields for the relay address and its public key / fingerprint.
- The relay has an SSH-style keypair: the private key stays on the relay,
  and the app pins the public key so it can recognise that relay.
- An optional relay password on top of the key, for people who want it.
- The host and friends connect out to the relay; the relay passes the
  encrypted traffic between them. No inbound port on the host.
- Friends get the relay details and pinned key from the invite instead of
  typing them in.
- Include a small Docker Compose setup for a personal relay on a VPS or
  another machine.

Why it fits: it gives people an alternative to forwarding a port on their
home network without the project taking on the cost or trust burden of
running a public service. The tunnel is already encrypted and pinned to the
host, so the relay should be able to stay a dumb forwarder that cannot read
game traffic.

Things to work out later:

- How the relay key is shown during setup and how a changed key is reported.
- Whether the optional password is required for every connection or only
  for setup, and how it is stored.
- Whether the relay needs UDP only or also a TCP/TLS fallback for
  locked-down networks.
- Whether we ship the Compose file or only document a recipe.
- Resource limits for the relay owner (bandwidth, connections, idle time).

Non-goal: a public relay operated by the project.

Done when a host behind CGNAT can tick the box, point at their own relay,
and a friend can play without either side opening a port. The normal direct
path stays the default.

## Friend-side bind address

**Status:** deferred — not planned for 1.1. Revisit if a game rejects `127.0.0.1`.

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
- A virtual LAN interface is out of scope; it needs privileges and
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
- Reuse the extracted runtime on later runs; remove old versions when the
  launcher version changes.
- A 7-Zip SFX is an alternative; the Go launcher is preferred for versioning,
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

## Done

- Per-group public address and app port (1.1).
- One-hour invite lifetime (1.1).
