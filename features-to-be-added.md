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
