# Interface

One window, fixed at 400×640. The native title bar stays; the app does not
pretend to be frameless. Content scrolls under a 48px app bar.

This is a compact utility that sits beside a game. Density is deliberate:
small type, short labels, one primary action per screen.

## Rules

- Window: 400×640, `SetFixedSize(true)`, centred on first open. No maximise, no
  resize.
- App bar: 48px, a back button or the brand on the left, help and settings on
  the right. One hairline under it.
- Page padding: 12px top and bottom, 16px sides. Cards: 10px inner padding
  and 12px radius. Buttons: 8px radius.
- Space: 4px grid. Sections are 12–14px apart; rows inside a section are
  4–6px apart.
- One primary button per screen. Everything else is secondary or quiet.
- No gradients, glows, coloured shadows, looping motion or emoji.
- Sentence case. No exclamation marks.
- The roster never shows the friend's local LAN IP or a tunnel address. The
  host sees the source address it actually has, in member details.

## Palette

The palette below is the current baseline from the mockup. It is isolated in
`internal/ui/theme.go`; the final palette and logo are still open, so a change
should not touch screen code.

| Role | Value |
| --- | --- |
| Background | `#0f1013` |
| Surface / card | `#17181c` |
| Raised | `#1d2025` |
| Hairline | `#2a2d34` |
| Text | `#e8eaee` |
| Secondary | `#a3a7b0` |
| Muted | `#7d8390` |
| Accent / loopback | `#5ad1e6` |
| Live | `#4ade80` |
| Warning | `#e0b341` |
| Danger | `#e0575c` |
| Primary button fill | `#12778a` |
| Primary button ink | `#eafcff` |

The contrast test in `internal/ui/theme_test.go` requires at least 4.5:1 for
the text, secondary, muted, accent, live, warning, danger and primary-button
pairs.

## Type

- IBM Plex Sans for the interface; JetBrains Mono for addresses, ports and the
  version.
- App bar title and names: 14–15, semibold.
- Headline: 22, semibold.
- Body: 13, regular.
- Meta: 12, muted.
- Addresses: 14; the large friend address: 16.
- Section labels: 12, muted.

## Components

- **App bar:** brand or back + title, then `?` and `Settings`. The current
  screen hides its own destination (no Settings button on Settings, no help
  button on the guide).
- **Card:** a surface step with a hairline. Depth comes from the surface, not a
  shadow.
- **Row button:** a full-width tappable row with an optional live dot, a title,
  a trailing line and a chevron.
- **Status dot:** 8px; green only when a real connection is live, muted when
  idle or offline.
- **Tag:** a small outlined label for a protocol or role.
- **Primary / secondary / quiet / danger buttons:** the custom button widget in
  `widgets.go`; not stock Fyne buttons.
- **Copy feedback:** the button label changes to `Copied` for 1.5 seconds.

## Screens

### Home

- Headline “A LAN cable across the internet.”
- One line explaining the loopback idea.
- `Host a group` (primary) and `Join with an invite` (secondary).
- A first-run link to the setup guide.
- `Your groups` and `Groups you joined`, each row showing the name, role,
  member count or host name, a live dot when the group has a session, and a
  quiet `Delete` action on host groups.
- An empty-state card under `Your groups` when nothing is saved.

### Host group

- App bar: back, group name, help, settings.
- Session panel: status dot and `Session live` / `Session off`, start/end
  button, `Hosting as …`, public address (or an amber “no public address”
  warning), app port, and a truthful connected count.
- Published: count, `Add service`, one card per service with the name, protocol,
  `Friends open 127.0.0.1:<ports>`, `On your LAN <target>` and the hint. Tapping
  a card opens details and Remove.
- People: count, `Invite`, a `You (host)` row and one row per member. A live dot
  and RTT come from the actual session; idle members are grey. Tapping a member
  opens the host-only details.
- Invites: open invite rows with name and expiry. A code created in this run
  has a Copy button; a code from a previous run is only a detail, because it is
  not written to disk.
- `Delete group` at the bottom, with a confirmation that explains it ends the
  session and revokes every member.

### Friend group

- App bar: back, group name, help, settings.
- Status panel: `Host live` / `Host offline`, hosted-by line, RTT, a note when
  the host removed the device or the connection ended, and a
  Disconnect/Reconnect button.
- Ready to play: one card per service. Each address is `127.0.0.1:<port>` with
  Copy; multi-port services list each address; a local port that fell back from
  the published port says so. The host's hint is shown under the address.
- If the host is offline, the saved services are shown as unavailable; the
  addresses are not claimed to work.
- People: the host and you. No other members and no host-only device data.
- `Leave group` at the bottom, with a confirmation that explains the saved
  group and device key are removed.

### Settings

- Startup: Close to tray (on by default; unticking makes closing the window
  quit the app), Start on login, Start minimised. Each option is a card with
  the tickbox and title on one line and a plain description underneath, aligned
  with the title. There is no `?` on individual settings; the description is
  always visible. Descriptions are smaller text under the title, not beside the
  tickbox.
- Network: public address and app port, with a live preview of the endpoint
  that will go into invites, a Save button and inline errors. The public
  address is usually just a hostname or IP; a `:port` suffix is only for an
  outside port that differs from the app port. The app port cannot change
  while a host session is live.
- Security: Approve new devices.
- About: version, data directory, key location.
- Links to the setup guide.

### How to Use

A scrollable offline guide. It opens with the `How it works` card: an example
LAN address, an encrypted hop, and the friend-side loopback address, with a note
that only published ports cross. Then Hosting, Joining, Problems and Security.
The hosting section reads the current app port and local IPv4 address so the
router line is concrete, for example `Forward UDP 51820 to 192.168.1.20`, with
Copy. It says plainly that only the app port is forwarded and that a CGNAT host
cannot be reached without a relay, which this version does not have.

### Dialogs

- Create group: group name and your name.
- Join: multi-line invite code and device name, then a waiting card while the
  host decides.
- Invite: friend's name, then the full code with Copy.
- Publish a service: preset picker, name, protocol, LAN target, friend ports,
  hint. Inline validation; a target with a port and multiple published ports is
  rejected.
- Device approval: friend, device, address, key fingerprint, Allow/Deny. It
  appears wherever the host is in the app and brings the window forward.
- Confirmations for delete group, leave group, end session, disconnect, remove
  service, remove member and cancel invite.

## States and copy

- `Live` means a QUIC connection is up. No invented reachability counts.
- The friend screen says `Host offline`, not `Connected`, when the host is
  unreachable.
- The friend screen says `Host live` only after the connection is established.
- An invite that is redeemed or expired is not offered for copying.
- A code is only shown again while the app is running; after a restart it is
  gone, and the host creates a new one.
- The guide and settings say what is actually forwarded and what is not.

## Tuning the interface

The measurements live in `internal/ui/tokens.json`. The app embeds that file at
build time, so the real screens and the tuner use the same values.

Open `docs/tuner.html` in a browser. It shows a live 400×640 reference with
Home, Host, Friend and Settings tabs and sliders for window size, spacing,
type sizes and the palette. Use **Download tokens.json**, replace
`internal/ui/tokens.json` with the file, and run `make run`.

If the fonts do not load when the page is opened directly, serve the repository
locally:

```sh
python3 -m http.server 8000
# http://localhost:8000/docs/tuner.html
```

The tuner is a browser reference, not a Fyne render. Text metrics, the native
checkbox, scrollbars and dialogs differ slightly. The numbers are exact, and
the real window is one `make run` away.

The token file controls measurements and colours, not structure. Section order,
layout, wording and new components still live in the Go screens.

## Previewing

The UI tests render screens with Fyne's test driver. To write PNGs:

```sh
mkdir -p .tmp/preview
LANPARTY_PREVIEW="$PWD/.tmp/preview" go test ./internal/ui -run TestPreviewScreens -v
```

The files are `home.png`, `host.png`, `friend.png`, `settings.png` and
`guide.png` at 400×640. These are useful when changing spacing, type or the
palette.
