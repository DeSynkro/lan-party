# LAN Party

A virtual LAN where a friend gets one port on one machine. Not your whole network, not your internet connection.

## What it is

You run a game server. A few friends want to play. Normally you would reach for WireGuard or Hamachi, and to let them reach `192.168.1.50:25565` you end up handing over `192.168.1.0/24`, which means your NAS, your router admin page and your printer come with it. Or you add a default route and discover your upload is now their internet.

LAN Party does the opposite. You publish one service. Your friends get exactly that service, on loopback, as though it were running on their own machine.

```
friend's game  ->  127.0.0.1:25565
                 their app, encrypted
              ->  100.64.0.1:25565
                 the box, inside the tunnel
              ->  192.168.1.50:25565
                 your LAN
```

The friend's game never leaves their computer. It talks to a real local socket, so any game with a direct connect field works unchanged.

## How it works

There is no network adapter, no TUN device and no admin prompt. The app carries its own WireGuard device and its own TCP/IP stack (gVisor) inside the process, and asks the OS for exactly one ordinary UDP socket.

The interface is written in Go with Fyne. No webview, no bundled browser, no npm tree.

Your app listens on one tunnel address, in carrier grade NAT space, and only on the ports you published. For each connection it dials the LAN target and copies bytes both ways.

Friends cannot reach each other, cannot reach anything else on your network, and cannot use you as a gateway. There is nothing to configure and nothing to forget.

## Status

Works end to end on Linux and Windows for the core of it: create a group, publish services, start a session, send an invite, join from another machine, and reach the published ports through loopback. TCP and UDP both forward.

Not built yet: the tray icon and start on boot, the live control channel that pushes changes to a running session, port conflict warnings on the friend side, and the self hosted relay.

The design is in [docs/design.md](docs/design.md).

## Platform support

Linux and Windows. macOS is not supported because the author cannot test it.

## Building

You need a C toolchain and the GL headers, which most desktops already have.

```
make run     # build and open the app
make test    # tests
make build   # binary in bin/
```

The first run downloads the pinned Go toolchain into `.tools/`, so nothing is installed system wide. Config written while developing stays inside the project too.

## Licence

GPLv3. See [LICENSE](LICENSE).

Built by [DeSynkro](https://desynkro.com) with assistance from OpenCode.
