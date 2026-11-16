# LAN Party design

Written while building. If something here turns out to be wrong, the document
changes with the code.

## The problem

Hosting a LAN game for friends over WireGuard gives away two things you did not
mean to give away.

The first is the network. To let a friend reach `192.168.1.50:25565` you put
`192.168.1.0/24` into `AllowedIPs`, and now your NAS, your router's admin page
and your printer are addressable from their machine.

The second is your connection. A `0.0.0.0/0` route or a stray masquerade rule
turns your box into their gateway, and their browsing becomes your bandwidth.

Both come from the same root cause. `AllowedIPs` is a routing table and a source
address check. It is not a port firewall. Hamachi, Radmin, ZeroTier and Tailscale
inherit the same shape: one flat network, protected by a password, where anything
listening on anyone's machine is reachable by everyone.

## What this is

A desktop app where the host publishes specific services and friends connect to
them as though they were local. Your app is the centre of the group. Nobody can
reach anything you did not publish.

One app, two roles. Host and client are the same binary. First run offers "start
a group" or "join a group", and the interface adapts from there. Roles swap week
to week, so a separate host app would only duplicate packaging and updates.

## How a packet moves

```
game -> 127.0.0.1:25565 -> friend's app -> [tunnel] -> 100.64.0.1:25565
     -> host's app -> 192.168.1.50:25565
```

The friend's game opens a real TCP connection to loopback. The app terminates it,
dials the box address through its own WireGuard device, and the host app accepts
it, then dials the LAN target. Three hops, and the game only sees the first one.

## Addresses

The box lives at `100.64.0.1`, inside carrier grade NAT space. Not `10.0.0.1`,
which is the default gateway on a large number of home routers and would invite
a friend to type their own router's admin page.

Peers get `100.64.0.2`, `.3` and so on, assigned by the host and carried in the
invite. Each peer is pinned to a `/32`, so a friend cannot claim another friend's
address.

## Roles

### Host

One userspace WireGuard device with one peer per friend. One gVisor stack on top.
The stack listens only on published services, on the box address alone.

Nothing on the host binds a LAN or wildcard address. For each accepted
connection the app dials the target from config, an IP literal and port, never a
name and never something the caller supplied. Bytes are copied both ways.

UDP services use a session table keyed by the five tuple, with an idle timeout.

### Friend

One peer. One local listener per published service. The listener tries
`127.0.0.1:<port>` first, then other loopback aliases, then the next port, and
shows whichever it won. Accepted connections are dialled through the stack to
`100.64.0.1:<port>`.

The listener is loopback only, never `0.0.0.0`.

## Groups, invites and the control channel

Groups persist. Ending a session tears down the tunnel, not the membership. Next
week the host starts a session and everyone's app finds it again.

An invite is a one use code carrying the group name, the host's public key, the
endpoint, the peer's box address and its assigned name. The host creates the peer
ahead of time, key pair included, so redeeming an invite needs no call back to a
server. After the first successful connection both ends rotate that peer onto a
key only the friend holds.

Because the host can add, grant and revoke while a session is live, there is a
small control channel on a reserved box port, separate from published services.
Over it the host pushes the service list, per peer grants, revocation notices and
key rotation. It also carries presence and latency, so the roster shows what is
actually true rather than a guess.

Revocation is authoritative: the host says so and then removes the peer. If the
message does not arrive, the friend sees host offline, which is a fair
description of their situation.

## Endpoints and reachability

The invite stores a `host:port`, and the host part should be a name, not an IP,
so a dynamic address survives. The app fills in the detected public address as a
starting point and lets the host replace it with a name they control.

Nothing inside a network can prove a port forward works. The evidence ladder is:

1. Optionally ask the router over UPnP or NAT-PMP.
2. Ask a public STUN server what it sees. If the mapped port differs from the
   listening port, inbound is definitely dead.
3. Trust a peer handshake, which is the only real proof.

If no friend completes a handshake, the host sees "nobody has reached UDP 51820".
No green tick that means nothing.

## VPNs

A host behind a full tunnel VPN cannot be reached at their home address, because
the public address belongs to the VPN. The app detects a VPN adapter when a group
is created and says so plainly, and suggests excluding LAN Party from the VPN if
the client supports per app exclusion.

A friend on a VPN is usually fine. The two things that bite are kill switches and
MTU, which is why the tunnel defaults to 1280 rather than 1420.

## Security

The exposure is small by construction.

- The host never listens on the LAN.
- The only internet facing surface is one UDP socket, and WireGuard answers
  unauthenticated packets with silence.
- Peers are pinned to `/32`, so source spoofing fails.
- No forwarding between peers, so a friend cannot reach another friend.
- No default route exists in the tunnel, so a friend cannot use the host as an
  exit node.
- The proxy dials a fixed target from config, so the box cannot be used as a
  pivot.

Threat model:

| Adversary | Mitigation |
| --- | --- |
| Internet scanner | WireGuard drops unauthenticated packets silently. The port does not confirm what it is. |
| Leaked invite | Single use, expiry, revocable before and after redemption, key rotated after first connect. |
| Curious friend | Only published ports exist. `/32` binding and no peer forwarding. |
| Malicious friend | Cannot reach anything else. Per peer connection caps and idle timeouts. |
| Malware on a friend's machine | Inherent. Their local listener is only reachable from their own loopback. |
| A malicious host | The host is the gateway and can see what it publishes. Documented, not hidden. |
| Supply chain | Few dependencies, pinned and verified builds, no runtime downloads. |

The app refuses to publish loopback targets unless the host explicitly chose one,
refuses wildcard targets, refuses its own ports, and warns on administrative
services such as 22, 3389 and 445. Keys live in the vault, encrypted at rest. The
app never asks for elevation.

What the app cannot fix: the game server's own vulnerabilities. An offline mode
Minecraft server is still an offline mode Minecraft server.

## What the interface has to say

Host: no group, group idle, session live with nothing published, publishing,
unreachable, VPN detected.

Friend: host offline, host live with nothing published, service available,
unreachable, port conflict, invite expired or revoked.

Port conflicts are found by trying the bind, never by scanning ahead, so a
harmless outgoing connection cannot raise a false alarm. On failure the app
identifies the owner where the platform allows it, then waits for the port to
free rather than quietly moving somewhere else.

Names and presence in the roster. No IP addresses. The friend's card reads
"Desynkro · Minecraft" with a copyable address behind it and one line saying
where to paste it.

## Data model

```
Group   { id, name, hostName, endpoint, peers[], services[], invites[] }
Peer    { name, publicKey, boxAddress, grants[], lastHandshake, state }
Service { id, presetId, name, proto, ports[], target, grants[] }
Invite  { id, peerRef, expiresAt, redeemedAt, state }
Preset  { id, name, proto, ports[], connectHint, builtin }
```

Built in presets are immutable. A service can override its ports without editing
the preset. Custom presets live in the same store and are marked custom.

## Build and packaging

Go core and a Fyne interface. One binary, no webview, no npm, no bundled browser.

- Linux: the binary plus an AppImage. A small script assembles the AppDir and
  runs `appimagetool`, pinned by version and checksum, because `fyne package`
  produces a tarball and not an AppImage.
- Windows: one exe with the icon embedded, built on a Windows CI runner. Fyne
  draws through cgo, so crossing from Linux needs a mingw toolchain; the runner
  is the sane path.
- Runtime needs OpenGL and X11 or Wayland, which every desktop already has. No
  package to install before the app will start.
- CI builds both on native runners and publishes checksums.

## Dependencies

Three matter for security: gVisor, wireguard-go and `golang.org/x`. Fyne pulls
GLFW and the GL bindings for the window, which is a build dependency rather than
a network one. There is no npm tree to audit. Dependabot keeps the modules
current, `govulncheck` and CodeQL run on a schedule, and nothing auto merges if
it touches the network stack.

There is no self updater. It is an attack surface for a project people can simply
download again.

## Not in this version

- Launching games or writing into their server lists.
- Answering LAN broadcast discovery, so older games without a direct connect
  field will not see the session.
- Friend to friend connections. The box is a star.
- More than one publishing host per session.
- A public relay service. A self hosted relay is a later milestone, and the
  invite format already allows for it.
- Hosting on Windows. Joining works everywhere.
