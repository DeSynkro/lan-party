# Security

The short version: the host identity is pinned, each friend has a device key,
invites are single use and stored as hashes, and the host approves every new
device. The app only carries the ports the host published.

This document is honest about what that does and does not prove.

## The two keys

### Host identity

One Ed25519 key per group, created when the group is created. The private half
is in the OS keyring under `lanparty` / `host/<group id>`. The public half is
used to build a self-signed TLS certificate.

Friends store a SHA-256 pin of the host's public key. The pin travels in the
invite and is saved in the friend's group entry. Certificate renewal does not
change the pin because the pin covers the key, not the certificate.

If the host key is lost, existing invites and friends stop matching. The group
has to be recreated and friends re-invited. The app does not silently make a
new key for a saved group.

### Device identity

One Ed25519 key per group on each friend's machine, created when the friend
redeems an invite. The private half is in the OS keyring under
`lanparty` / `device/<group id>`. The host stores only the public half.

The TLS handshake proves the client owns that private key. The host compares
the public key with the member list. A new key for a known member is a new
device and needs approval.

### What the host sees

In the member details, the host sees:

- the display name the host gave the invite
- the device name the friend's app reported
- the source address of the connection, as the host sees it
- a short device-key fingerprint
- first seen and last seen

The device name is self-reported. The source address is observed, but it can
change with DHCP, mobile networks, VPNs and CGNAT. The key fingerprint is the
only part that is cryptographic.

The app does not collect MAC addresses. A MAC never crosses the internet; an
app can only read its own and send it, and a modified client can send anything.
Showing it would look like verification without being verification.

## Invite lifecycle

An invite is a random 160-bit token, formatted as
`LP1-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX`. The full pasteable code wraps the
group details, endpoint, host pin, token, peer name and expiry. The token is
inside the code; there is no private key in it.

The host:

1. generates the token and a SHA-256 hash
2. stores only the hash in the group file
3. keeps the full code in memory for the current run so it can be copied
4. marks the invite redeemed and deletes it after a successful join

Rules:

- single use
- 24-hour expiry
- only redeemable while the host's session is running
- failed attempts are rate limited: 5 per source address per minute
- a denial or timeout leaves the invite open until it expires
- a successful join burns it

The friend's app discards the token after joining. The host must be online
while the friend redeems the invite, which is the same time the host is
playing anyway.

Because the host stores only a hash, a code cannot be shown again after a
restart. Create a new invite for the same friend if you need to send it again.

## Device approval

The first join of a new device asks the host. The card shows the friend name,
device name, source address and key fingerprint. The host can allow or deny.

After approval, the device key is the membership. On later connections:

- same key, approved member: allowed without a prompt
- new key, same member id: "new device", always asks, and replaces the old key
  after approval
- new key, new member, valid invite: "join", asks if
  `Approve new devices` is on
- anything else: denied

A changed source address or device name is shown, not used to kick anyone. IP
addresses and device names change for ordinary reasons. A changed key is the
signal that matters.

Revoking a member sends a `revoked` control message, closes the connection and
deletes the member record. The green light goes out; they cannot reconnect
without a new invite.

## Transport

- QUIC with TLS 1.3.
- ALPN is `lanparty/1`.
- The host presents a self-signed certificate built from its identity key.
- The friend verifies the host key against the pin in the invite. There is no
  certificate authority and no hostname check; the pin is the identity.
- The host requires a client certificate. The client certificate is
  self-signed and built from the device key. The host checks the public key
  against the member list after the handshake.
- QUIC keepalive and idle timeout detect a peer that vanishes.

The transport is a standard library, not homemade crypto. The app's own crypto
code generates keys, builds certificates, hashes pins and hashes invite tokens.

## Network surface

Host:

- one UDP socket for QUIC, default port 51820
- nothing bound to the LAN or to a wildcard address for game services
- for each accepted TCP stream, a dial to a fixed IP and port from the host's
  service configuration
- for each UDP flow, a UDP socket to that same fixed target

Friend:

- one or two loopback listeners per published service, on `127.0.0.1` only
- one outgoing QUIC connection to the host

The app never asks for root, never creates a TUN device, and never changes a
route or firewall rule.

Target validation:

- the target must be an IP literal, not a name
- wildcard, multicast and broadcast addresses are rejected
- a bare IP plus a port list publishes the same port numbers
- an `IP:port` target plus one published port is a remap
- an `IP:port` target plus more than one published port is ambiguous and
  rejected
- loopback targets are allowed with a warning
- ports 22, 3389, 445 and 5900 get a warning, but the host can still choose
  them

## Storage

- Group data is JSON at `~/.local/share/lan-party/groups.json`, mode `0600`,
  directory `0700`.
- Settings are JSON at `~/.config/lan-party/settings.json`, mode `0600`.
- Private keys are in the OS keyring. They are never written to the JSON, the
  invite, a log or the clipboard.
- Invite tokens are hashed with SHA-256 before storage.
- The keyring is the only secret store. If it is unavailable or locked, the
  operation that needs a key fails with an error. The app does not fall back to
  a plaintext key file.

The app writes no secrets to logs. It has no telemetry and no auto-update.

## Limits

- 16 services, 8 ports each, 32 members
- 128 concurrent TCP streams per peer
- 64 UDP flows per service per peer, 60-second idle timeout
- 5 failed enrollment attempts per address per minute
- 2-minute approval wait
- one control message is capped at 64 KiB
- a friend accepts at most 16 services and 8 ports per service from the host,
  so a compromised host cannot fill the friend's loopback port space

## Threat model

| Adversary | What happens |
| --- | --- |
| Internet scanner | Sees one UDP port running QUIC. It gets no service list, no member list and no data without a valid device key. |
| Leaked invite | Single use, 24-hour expiry, host approval, rate limiting and revocation. A stranger's device details appear on the approval card. |
| Curious friend | Only published services exist. No route, no peer-to-peer path, no access to the host's router or other devices. |
| Malicious friend | Cannot reach anything else. Stream, flow, member and rate limits cap resource use. |
| Stolen device key | Can impersonate that member until the host revokes it. A changed source address may be visible, but the key is the credential. Revoke and issue a new invite. |
| Stolen host key | Can impersonate the host to friends who already have the pin. The keyring is the protection. |
| Malicious host | The host is the gateway and can see the traffic it publishes. That is inherent in the design. |
| Same-user malware | Can read the keyring while it is unlocked and can read the app's files. A desktop app cannot defend against a compromised desktop. |
| Supply chain | Few dependencies, pinned versions, `govulncheck` in CI, no runtime downloads. |
| Game server vulnerability | Not fixed by this app. An offline-mode Minecraft server is still an offline-mode Minecraft server. |

## What this does not protect

- The game server's own bugs, weak passwords or missing whitelists.
- A machine that is already compromised while the keyring is unlocked.
- A host behind carrier-grade NAT. This version has no relay.
- Someone who reads the invite before the intended friend redeems it. The
  approval card is the last chance to deny a stranger.
- The host seeing which game traffic crosses the tunnel. That is the job.

## If a device is lost

1. Open the group and remove the member. This sends `revoked`, closes the
   connection and deletes the key.
2. Create a new invite when the friend has a replacement device.
3. Approve the new device when it joins.

The old invite, if one is still open, can be cancelled in the Invites section.
