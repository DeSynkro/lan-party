# Features to be added

A lightweight list of ideas that are not scheduled. Nothing here is a
commitment.

## Relay mode

**Status:** idea, not scheduled

The host currently needs one forwarded UDP port and a reachable public
address. That does not work behind CGNAT or on networks where the router
cannot be configured.

Add an optional relay mode:

- A **Use relay** checkbox when hosting.
- Fields for the relay address and a relay password / safety key.
- The host and friends connect out to the relay; the relay passes the
  encrypted traffic between them. No inbound port on the host.
- Friends get the relay details from the invite instead of typing them in.
- Include a small Docker Compose setup so someone can run a relay on a VPS
  or another machine.

Why it fits: it is the standard answer for CGNAT and removes the hardest
part of setup for non-technical hosts. The tunnel is already encrypted and
pinned to the host, so the relay should be able to stay a dumb forwarder
that cannot read game traffic.

Things to work out later:

- Relay authentication: one shared password, per-group keys, or invite
  tokens.
- Whether the relay needs UDP only or also a TCP/TLS fallback for
  locked-down networks.
- Abuse and bandwidth limits for a public relay.
- Whether we ship the Compose file or only document a recipe.

Done when a host behind CGNAT can tick the box, point at a self-hosted
relay, and a friend can play without either side opening a port. The normal
direct path stays the default.
