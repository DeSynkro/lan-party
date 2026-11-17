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

## Per-group public address and app port

**Status:** idea, not scheduled

Hosted groups currently share the global public address and app port from
Settings. That is awkward when different groups are hosted over different
networks, VPNs or router forwards, or when each group should keep a stable
invite address.

Add optional overrides on a host group:

- Public address (hostname or IP, with an optional outside port)
- App port (the local UDP port the host listens on)
- Leave either blank to use the global setting.
- The group screen should show which address and port it will use.
- Invites for that group should use the overrides.
- Start session must bind the group's app port, not the global one.
- Only one host session runs at a time, so overlapping ports are not an
  issue yet.

Why it helps: a group can stay tied to a particular network or port forward,
and changing global settings will not change that group's invites. The outside
port override already exists globally through `host:port` in the public
address; this makes it per group.

Things to work out later:

- Whether the app port override is worth the extra UI given only one session
  can run at a time.
- What happens when the overridden port is already in use.
- Whether changing the address or port while a session is live should be
  blocked or require ending the session.
- How this interacts with relay mode when that exists.

## Friend-side bind address

**Status:** idea, not scheduled

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
