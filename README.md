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

Your app listens on one tunnel address, in carrier grade NAT space, and only on the ports you published. For each connection it dials the LAN target and copies bytes both ways.

Friends cannot reach each other, cannot reach anything else on your network, and cannot use you as a gateway. There is nothing to configure and nothing to forget.

## Status

Early. Nothing here works yet. The design is in [docs/design.md](docs/design.md).

## Platform support

Linux and Windows. macOS is not supported because the author cannot test it.

## Building

Eventually: `make dev` for a local run, `make package` for the AppImage and the Windows exe. For now the Makefile will tell you what is missing.

## Licence

GPLv3. See [LICENSE](LICENSE).

Built by [DeSynkro](https://desynkro.com) with assistance from OpenCode.
