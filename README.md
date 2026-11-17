# LAN Party

A small Linux desktop app that gives a friend one local port onto a game server
on your LAN, and nothing else.

You publish a service. Your friend joins with a one-time invite and opens
`127.0.0.1:25565` in their game. Their traffic crosses one encrypted connection
to your machine. Their game never touches your network directly, and your other
devices are never on the path.

## Status

The app is being rebuilt from the ground up. The previous implementation and the
two interface attempts are kept in the `legacy` and `ui-redesign` branches.

The new design is in [docs/design.md](docs/design.md). The interface mockup is in
[docs/mockups/ui-v1.html](docs/mockups/ui-v1.html).

## Licence

GPLv3. See [LICENSE](LICENSE).

Built by [DeSynkro](https://desynkro.com).
