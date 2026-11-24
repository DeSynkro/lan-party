# Security

The short version: the host identity is pinned, each friend has a device key,
invites are single use and stored as hashes, and the host approves every new
device. The app only carries the ports the host published.

This document is honest about what that does and does not prove.

## The two keys

### Host identity

One Ed25519 key per group, created when the group is created. The private half
is an owner-only file in the keys directory (`~/.local/share/lan-party/keys/`
on Linux, `%LocalAppData%\lan-party\keys\` on Windows). The public half is used
to build a self-signed TLS certificate.

Friends store a SHA-256 pin of the host's public key. The pin travels in the
invite and is saved in the friend's group entry. Certificate renewal does not
change the pin because the pin covers the key, not the certificate.

If the host key is lost, existing invites and friends stop matching. The group
has to be recreated and friends re-invited. The app does not silently make a
new key for a saved group.

### Device identity

One Ed25519 key per group on each friend's machine, created when the friend
redeems an invite. The private half is an owner-only file in the same keys
directory (`~/.local/share/lan-party/keys/` on Linux,
`%LocalAppData%\lan-party\keys\` on Windows). The host stores only the public
half.

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

The app does not collect MAC addresses. A MAC never crosses the internet. An
app can only read its own and send it, and a modified client can send anything.
Showing it would look like verification without being verification.

## Invite lifecycle

An invite is a random 160-bit token, formatted as
`LP1-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX`. The full pasteable code wraps the
group details, endpoint, host pin, token, peer name and expiry. The token is
inside the code. There is no private key in it.

The host:

1. Generates the token and a SHA-256 hash
2. Stores only the hash in the group file
3. Keeps the full code in memory for the current run so it can be copied
4. Marks the invite redeemed and deletes it after a successful join

Rules:

- single use
- 1-hour expiry
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
deletes the member record. The green light goes out. They cannot reconnect
without a new invite.

## Transport

- QUIC with TLS 1.3.
- ALPN is `lanparty/1`.
- The host presents a self-signed certificate built from its identity key.
- The friend verifies the host key against the pin in the invite. There is no
  certificate authority and no hostname check. The pin is the identity.
- The host requires a client certificate. The client certificate is
  self-signed and built from the device key. The host checks the public key
  against the member list after the handshake.
- QUIC keepalive and idle timeout detect a peer that vanishes.

The transport is a standard library, not homemade crypto. The app's own crypto
code generates keys, builds certificates, hashes pins and hashes invite tokens.

### Through a relay

The optional self-hosted relay forwards UDP packets. It does not terminate the
QUIC/TLS connection, so the host pin and the host/friend encryption stay end to
end. The relay sees source addresses, timing, packet sizes and the fact that a
session exists; it cannot read game traffic.

- The host connects to the relay over TLS 1.3 on TCP, authenticates with the
  relay password, and pins the relay public-key fingerprint (the Relay ID).
- The host also opens one UDP socket to the same relay port and keeps the NAT
  mapping alive. All friend traffic shares that socket.
- Each invite carries a single-use 16-byte first-join token. The relay stores
  only its SHA-256 hash and expires it within one hour.
- After the first approved join, the host issues a separate per-device
  reconnect token inside the encrypted connection. The host and relay store
  only its hash; the friend stores the raw token owner-only. It is reusable
  only while a host session exists, can rebind to a new source address after a
  short silence, and is revoked when the member is removed or reset.
- The host refreshes the relay registration for current members whenever a
  relay session starts. Friends reconnect with the token and their device key,
  not a new invite.
- The relay assigns a 16-bit friend ID. The host app presents a synthetic UDP
  address per friend to QUIC, so no per-friend port range is needed.
- While no host session exists, the relay does not bind UDP at all. Only the
  TLS control listener remains.
- A changed relay key does not match the pinned Relay ID and the host refuses
  the connection until the user enters the new Relay ID.

The relay password authorizes a host to create a session. It is never put in an
invite and is stored in the owner-only secret store, not in `settings.json`.

## Network surface

Host, direct mode:

- one UDP socket for QUIC, default port 51820
- nothing bound to the LAN or to a wildcard address for game services
- for each accepted TCP stream, a dial to a fixed IP and port from the host's
  service configuration
- for each UDP flow, a UDP socket to that same fixed target

Host, relay mode:

- one outbound TLS control connection to the relay on TCP
- one outbound UDP socket to the relay's data port, opened only while a host
  session exists
- no inbound port on the host PC

Friend:

- one or two loopback listeners per published service, on `127.0.0.1` only
- one outgoing QUIC connection to the host, directly or through the relay

Relay:

- one TCP port for the control channel, `7443` by default, always listening
- one UDP port for data, the same number, bound only while a host session is
  active
- no inbound connection to the host PC and no per-friend port range

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
- Settings are JSON at `~/.config/lan-party/settings.json`, mode `0600`. Relay
  host, port and Relay ID live here; relay passwords do not.
- Private keys, relay passwords and relay reconnect tokens are separate files
  in `~/.local/share/lan-party/keys/` on Linux or
  `%LocalAppData%\lan-party\keys\` on Windows, mode `0600` where the OS
  supports it, in an owner-only directory. They are never written to the JSON,
  the invite, a log or the clipboard.
- On Windows, settings live under `%AppData%\lan-party` and group data, keys and
  the log under `%LocalAppData%\lan-party`. The per-user profile ACLs protect
  them.
- Start on login uses the user's `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
  key on Windows and `~/.config/autostart` on Linux. It never needs admin rights.
- `lan-party.log` contains startup and connection errors for support. It never
  contains invite codes, tokens or private keys.
- Invite tokens are hashed with SHA-256 before storage.
- There is no keyring daemon, session bus or unlock prompt. A key file that is
  readable by another user is refused instead of used.
- The key files are not passphrase-encrypted. Anyone who can read your user
  account can read them, so full-disk encryption is what protects a stolen
  disk. This is the same trade-off as an SSH private key.

The app writes no secrets to logs. It has no telemetry and no auto-update.

## Limits

- 16 services, 8 ports each, 32 members
- 128 concurrent TCP streams per peer
- 64 UDP flows per service per peer, 60-second idle timeout
- 64 host connections, with at most 8 simultaneous connections per member
- 32 pending device approvals
- 32 pending QUIC handshakes, with a 10-second hello deadline
- 5 failed enrollment attempts per address per minute, tracking at most 1024
  source addresses
- 2-minute approval wait
- 5-second control-stream write deadline
- one control message is capped at 64 KiB
- a friend accepts at most 16 services and 8 ports per service from the host,
  so a compromised host cannot fill the friend's loopback port space

## Threat model

| Adversary | What happens |
| --- | --- |
| Internet scanner | Sees one UDP port running QUIC. It gets no service list, no member list and no data without a valid device key. |
| Leaked invite | Single use, 1-hour expiry, host approval, rate limiting and revocation. A stranger's device details appear on the approval card. |
| Curious friend | Only published services exist. No route, no peer-to-peer path, no access to the host's router or other devices. |
| Malicious friend | Cannot reach anything else. Stream, flow, member and rate limits cap resource use. |
| Stolen device key | Can impersonate that member until the host revokes it. A changed source address may be visible, but the key is the credential. Revoke and issue a new invite. |
| Stolen host key | Can impersonate the host to friends who already have the pin. Owner-only file permissions and full-disk encryption are the protection. |
| Malicious host | The host is the gateway and can see the traffic it publishes. That is inherent in the design. |
| Malicious relay | Can drop, delay, reorder or observe packet metadata, but cannot read the end-to-end QUIC/TLS traffic or impersonate the host. |
| Leaked Relay ID | Identifies the relay but grants no access. The relay password and friend tokens are separate. |
| Stolen reconnect token | Can reach the relay only while a host session is live, but cannot enter the group without the device key. Revoke with Reset reconnect access, member removal, or the inactivity cleanup. |
| Stolen relay password | Can create host sessions on that relay, but cannot impersonate the real host to friends who pinned its key. Change the relay password and create new invites. |
| Same-user malware | Can read the key files, settings and groups as your user. A desktop app cannot defend against a compromised desktop. |
| Supply chain | Few dependencies, pinned versions, `govulncheck` in CI, no runtime downloads. |
| Game server vulnerability | Not fixed by this app. An offline-mode Minecraft server is still an offline-mode Minecraft server. |

## Web host (1.3)

The web host controls who may reach the LAN services, so it is treated as a
high-value surface. It is a separate binary and data directory from the desktop
app.

### Authentication

- One admin password. No username field, so there is no account enumeration.
- PBKDF2-SHA256, 600,000 iterations, 32-byte key, 16-byte random salt,
  constant-time comparison (shared with the relay).
- First run generates a 24-byte random password, prints it once to stdout and
  writes it once to the owner-only `data/lan-party.log`, then stores only the
  salted hash in `data/web.json` (`0600`). Remove or rotate the log after saving
  the password. `passwd` replaces it. A user-supplied password must be at least
  12 characters.
- Login failures return a generic message and a fixed delay. Per-address token
  buckets and a global failure cap return `429` with `Retry-After`.

### Sessions and CSRF

- 256-bit random session IDs, held in memory only. A restart signs everyone out;
  no session-signing secret is stored on disk.
- Cookie `lanparty_web_session`: `HttpOnly`, `SameSite=Strict`, `Path=/`,
  `Secure` on HTTPS, no `Domain`. The ID rotates on login.
- Idle timeout 12 hours, absolute timeout 7 days. Logout destroys the session
  server-side.
- Every authenticated mutation needs a per-session CSRF token and a matching
  `Origin` header. Browsers that omit `Origin` or send `null` on same-origin
  form posts (because the app sets `Referrer-Policy: no-referrer`) are accepted
  when `Sec-Fetch-Site` is `same-origin` or `none`; the CSRF token is still
  required. Cross-site requests and foreign Origins are rejected.
- A password change destroys every session.

### Headers and transport

- CSP without inline script, `nosniff`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`, restrictive `Permissions-Policy`,
  `Cache-Control: no-store` on authenticated pages.
- Request body limit 64 KiB, header limit 1 MiB, server read/write/idle
  timeouts.
- Optional direct TLS (1.2 minimum) or a reverse proxy. Plain HTTP on a
  non-loopback address logs a warning.
- `X-Forwarded-For` and `X-Forwarded-Proto` are honoured only from the
  configured trusted-proxy CIDRs.

### Storage and exposure

- Host keys stay owner-only files (`0700` directory, `0600` files) exactly as
  the desktop app. There is no app-level passphrase in 1.3: encrypt the disk and
  backups.
- Invite codes, password hashes, private keys and session IDs are never logged
  or rendered on a logged-out page. Invite codes are shown only while an invite
  is open and are deleted from memory when it is redeemed or cancelled.
- The web host does not create or expose relay secrets, Relay IDs or reconnect
  tokens. Relay settings copied into its data directory are normalized to
  direct mode at load.
- The container runs as a non-root UID with a read-only root filesystem, dropped
  capabilities, `no-new-privileges`, a PID limit and memory/CPU limits. Only
  `/data` is writable.
- Audit entries cover login success and failure, logout, password changes,
  settings changes, session start/stop, group changes, invites, approvals and
  member removal. Newlines are stripped so a name cannot forge a log line.

### What this does not protect

- A web UI exposed without TLS. Use a trusted certificate or a reverse proxy.
- A stolen `data/` directory or backup. Encrypt both.
- A host already compromised as the service user or root.
- Multiple administrators. There is one password and no per-user roles in 1.3.

## What this does not protect

- The game server's own bugs, weak passwords or missing whitelists.
- A machine that is already compromised as your user.
- A relay that is down or refuses service. The host retries for about 16
  minutes, then waits for a manual start; it never silently falls back to a
  direct connection.
- A friend who exceeds the host's inactivity window. Their member record and
  reconnect token are removed; they need a new invite and approval.
- Someone who reads the invite before the intended friend redeems it. The
  approval card is the last chance to deny a stranger.
- The host seeing which game traffic crosses the tunnel. That is the job.

## If a device is lost

1. Open the group and remove the member. This sends `revoked`, closes the
   connection and deletes the key.
2. Create a new invite when the friend has a replacement device.
3. Approve the new device when it joins.

The old invite, if one is still open, can be cancelled in the Invites section.
