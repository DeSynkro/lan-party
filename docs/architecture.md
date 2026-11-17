# Architecture

This is the plain version: what happens when a friend joins, where the bytes
go, and which package owns which part.

## The short version

The host listens on one UDP port. A friend connects over QUIC, verifies the
host's public key against the pin in the invite, and presents a device key. The
host approves that key once. For every published service, the friend app opens
a listener on `127.0.0.1`; a game connection becomes a QUIC stream, and the
host dials the LAN target and copies bytes. UDP game packets travel as QUIC
datagrams.

There is no virtual network adapter, no TUN device and no route table. The
tunnel carries only the services the host published.

## One connection, step by step

```
game -> 127.0.0.1:25565 -> friend app -> [QUIC stream] -> host app
     -> 192.168.1.50:25565
```

1. The host created a group, which created a host identity in an owner-only
   key file.
2. The host added a service: name, protocol, LAN target and the port friends
   see.
3. The host started a session. The app opened a UDP socket and a QUIC listener.
4. The host created an invite. The invite string contains the group id, the
   public endpoint, the host key pin, a one-time token and an expiry.
5. The friend pasted the invite. The friend app connected, checked the host
   pin, and sent a `hello` with the token and its device name. The host showed
   an approval card; the host allowed it.
6. The host sent `welcome` with the service list. The friend app bound one
   loopback listener per published port.
7. The game connected to `127.0.0.1:25565`. The friend app opened a QUIC stream
   with a small header: service id and published port.
8. The host checked the service against its list, dialled `192.168.1.50:25565`,
   and copied bytes both ways.
9. For UDP, the friend app kept a flow table keyed by the game's source
   address. Each flow got an id; packets carried the published port and flow
   id. The host kept one UDP socket per flow towards the target.

## Why there is no virtual network

The first version put WireGuard and a TCP/IP stack (gVisor) inside the process.
That works, but it creates a network, and a network needs a routing table, a
host address and rules for what may cross it. This version does not create a
network. It carries a list of published ports. There is no address a friend
could type to reach something else, because there is no address.

The trade-off is that this is not a general VPN. It is a port tunnel for a LAN
party, and it says so.

## Packages

| Package | Job |
| --- | --- |
| `cmd/lan-party` | entry point; calls `ui.Run` |
| `internal/app` | groups, settings, invites, approval, live sessions |
| `internal/config` | XDG paths and `settings.json` |
| `internal/secure` | owner-only key files, Ed25519 identities, certs, pins, fingerprints |
| `internal/group` | group, service, member and invite records; validation; store |
| `internal/invite` | one-time token and pasteable invite code |
| `internal/tunnel` | QUIC listener/dialer, TLS pinning, control stream, data streams, datagrams |
| `internal/forward` | TCP pipes and UDP flow forwarding |
| `internal/session` | host and friend session lifecycle, presence, service updates |
| `internal/ui` | Fyne screens, theme, widgets, in-app guide |
| `internal/autostart` | XDG desktop entry for login |
| `internal/buildinfo` | version string |
| `internal/preset` | the shipped game port table |

The dependency direction is roughly:

```
ui -> app -> session -> tunnel -> secure
             |  |
             |  -> forward
             -> group / invite
```

`app` is the only package that decides whether a device is allowed. `session`
does not know about invites or members; it forwards what `app` approved.

## The control stream

The first stream on a connection carries newline-separated JSON. It is used
for:

- `hello`, `welcome`, `denied` during the handshake
- `services` when the host adds or removes a service
- `revoked` when the host removes a device
- `bye` when the friend leaves

The host app can push a new service list while a session is live. The friend
app closes the old listeners and opens the current ones. A TCP stream that is
already open keeps going; the listener only stops accepting new games.

## UDP details

A QUIC datagram is unreliable and unordered, like UDP. The app datagram header
is:

```
[ published port: 2 bytes ][ flow id: 8 bytes ][ game payload ]
```

The friend's flow table maps the game's source address to a flow id. The host
maps `(published port, flow id)` to a UDP socket towards the target. Flows
expire after 60 seconds of silence. The payload cap is 1190 bytes, which keeps
the whole datagram under 1200 bytes.

## Data on disk

| Data | Where | Mode |
| --- | --- | --- |
| settings | `~/.config/lan-party/settings.json` | 0600 |
| groups, services, members, public keys, invite hashes | `~/.local/share/lan-party/groups.json` | 0600 |
| private host and device keys | `~/.local/share/lan-party/keys/` | 0600 files in a 0700 directory |

Private keys are never written to the JSON, an invite or a log. A key file that
other users can read is refused.

The group store uses a versioned JSON wrapper and an atomic replace: write a
temporary file in the same directory, chmod it, sync it, rename it. Directories
are `0700`.

There is no database, no local server and no daemon.

## Limits

- 16 services per group
- 8 ports per service
- 32 members per group
- 128 concurrent streams per peer
- 64 UDP flows per service per peer
- 60-second UDP idle timeout
- 5 failed enrollment attempts per address per minute
- 2-minute approval wait
- one control message of at most 64 KiB
- the friend accepts at most 16 services and 8 ports per service from a host

These are guards against a broken or malicious client, not billing limits.

## What the app does not do

- It does not route IP packets.
- It does not create a network interface.
- It does not forward between friends.
- It does not run a relay. A host behind carrier-grade NAT needs a reachable
  address, and this version cannot provide one.
- It does not resolve hostnames for LAN targets. The host dials exactly the IP
  it configured.
- It does not update itself. Download a new AppImage.

## Where to read next

- [security.md](security.md) for the key and invite lifecycle
- [ui.md](ui.md) for the window and screens
- [guide.md](guide.md) for the user-facing setup
- [development.md](development.md) for the build and tests
