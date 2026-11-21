# Interface

One window, fixed at 400×640. It is borderless and floats as a normal window.
The app bar is the drag surface. Content scrolls under a 48px app bar.

This is a compact utility that sits beside a game. Density is deliberate:
small type, short labels, one primary action per screen.

## Rules

- Window: 400×640, fixed size, borderless, no OS title bar or resize handles.
  The window floats under tiling window managers such as Krohnkite.
- App bar: 38px minimum, a back button or the brand on the left, help and
  settings on the right. The bar, title row and top strip are draggable.
- Page padding: 20px. Cards: 14px inner padding and 12px radius. Buttons: 9px
  radius.
- Space: 4px grid. Sections are 14px apart. Rows inside a section are
  4–8px apart.
- One primary button per screen. Everything else is secondary or quiet.
- Gradients are limited to the window background and primary buttons. No
  glows, coloured shadows, looping motion or emoji.
- Sentence case. No exclamation marks.
- The roster never shows the friend's local LAN IP or a tunnel address. The
  host sees the source address it actually has, in member details.

## Palette

The palette below is the current baseline and lives in
`internal/gtkui/style.css`.

| Role | Value |
| --- | --- |
| Background | `#08040F` |
| Deep / card | `#120A2E` |
| Surface / popover | `#150C2E` |
| Raised | `#1A1035` |
| Hairline | `#2C1A55` |
| Text | `#FDF4FF` |
| Secondary | `#C9B8E8` |
| Muted | `#907FAF` |
| Accent | `#FF3DBE` |
| Violet | `#8B5CF6` |
| Cyan | `#22D3EE` |
| Live | `#4ADE80` |
| Warning | `#F5B942` |
| Danger | `#FF6B81` |

`internal/gtkui/style_test.go` rejects any `@colour` reference that is not
defined in the stylesheet.

## Type

- Space Grotesk for the brand and titles. IBM Plex Sans for the interface.
  JetBrains Mono for addresses, ports and the version.
- App bar title and names: 14–15, semibold.
- Headline: 22, semibold.
- Body: 13, regular.
- Meta: 12, muted.
- Addresses: 14. The large friend address: 16.
- Section labels: 12, muted.

## Components

- **App bar:** brand or back + title, then `?` and `Settings`. The current
  screen hides its own destination (no Settings button on Settings, no help
  button on the guide).
- **Card:** a surface step with a hairline. Depth comes from the surface, not a
  shadow.
- **Row button:** a full-width tappable row with an optional live dot, a title,
  a trailing line and a chevron.
- **Status dot:** 8px. Green only when a real connection is live, muted when
  idle or offline.
- **Tag:** a small outlined label for a protocol or role.
- **Primary / secondary / quiet buttons:** GTK buttons styled in `style.css`,
  not the stock theme buttons.
- **Copy feedback:** the button label changes to `Copied` until the screen is
  rebuilt.

## Screens

### Home

- Horizontal brand lockup and the loopback explanation.
- `Host a group` (primary) and `Join with an invite` (secondary).
- A first-run link to the setup guide.
- `Your groups` and `Groups you joined`, each row showing the name, role,
  member count or host name, and a live dot when the group has a session.
- An empty-state card under `Your groups` when nothing is saved.

### Host group

- App bar: back, group name, help, settings.
- Session panel: status dot and `Session live` / `Session off`, start/end
  button, `Hosting as …`, the effective address with a `Default`/`Custom` tag
  and a `Relay` tag when relay mode is active (or an amber “no public address”
  warning), app port or relay route, `Change` opening the group-only connection
  dialog, and a truthful connected count. An unexpected relay loss shows
  `Relay connection lost. Reconnecting…`; the host retries on the fixed
  5s/10s/30s schedule and says when automatic recovery stopped.
- Published: count, `Add service`, one card per service with the name, protocol,
  `Friends open 127.0.0.1:<ports>`, `Game server <address>` and the hint.
  Tapping a card opens details with Edit and Remove.
- People: count, `Invite`, a `You (host)` row and one row per member. Member
  rows show a device-name tag next to the name. The host row stays `[host]`.
  A live dot and RTT come from the actual session. Idle members are grey.
  Tapping a member opens the host-only details.
- Invites: open invite rows with name and expiry. A code created in this run
  has a Copy button. A code from a previous run is only a detail, because it is
  not written to disk.
- `Delete group` at the bottom, with a confirmation that explains it ends the
  session and revokes every member.

### Friend group

- App bar: back, group name, help, settings.
- Status panel: `Host live` / `Reconnecting` / `Host offline`, hosted-by line,
  RTT, a note when the host removed the device or the connection ended, the
  next retry while reconnecting, and a Disconnect/Reconnect now/Reconnect
  button. A relay group reconnects with its saved device key and reconnect
  token, using the same automatic retry schedule as direct mode. A group with
  no reconnect token yet shows `New invite`; the screen only asks for a new
  invite when the token is missing or was reset.
- Ready to play: one card per service. Each address is `127.0.0.1:<port>` with
  Copy. Multi-port services list each address. A local port that fell back from
  the published port says so. The host's hint is shown under the address.
- If the host is offline, the saved services are shown as unavailable. The
  addresses are not claimed to work.
- People: the host and you. No other members and no host-only device data.
- `Leave group` at the bottom, with a confirmation that explains the saved
  group and device key are removed.

### Settings

- Startup: Close to tray (on by default. Unticking makes closing the window
  quit the app), Start on login, Start minimised. Each option is a card with
  the tickbox and title on one line and a plain description underneath, aligned
  with the title. There is no `?` on individual settings. The description is
  always visible. Descriptions are smaller text under the title, not beside the
  tickbox.
- Network defaults: a `Use Relay` tickbox swaps between the direct fields
  (public address, app port) and the relay fields (relay host, relay port,
  Relay ID, relay password). Both sets stay saved while hidden. The preview
  says which address or relay goes into invites. `Test relay` and Save sit
  under the fields, with inline errors. The relay password is never shown after
  saving; leave the password field blank to keep it. These are the defaults for
  host groups without their own connection. A host group can override them in
  its own connection dialog. Changing a live group's app port or relay stops and
  restarts the session after confirmation. Open invites whose effective
  connection changed are cancelled.
- Connection: Reconnect automatically, on by default. It retries a dropped
  friend connection for up to 30 minutes.
- Friends: `Forget inactive friends after [N] days`. Blank means never. New
  installs default to 5; upgraded settings files start blank so nobody is
  forgotten before the host chooses. Friends who have not connected in that
  window are removed and need a new invite. Live friends are kept.
- Security: Approve new devices.
- About: version, data directory, key location, log path.
- Links to the setup guide.

### How to Use

A scrollable offline guide. It opens with the `How it works` card: an example
LAN address, an encrypted hop, and the friend-side loopback address, with a note
that only published ports cross. Then Hosting, Joining, Problems and Security.
The hosting section reads the current app port and local IPv4 address so the
router line is concrete, for example `Forward UDP 51820 to 192.168.1.20`, with
Copy. It says plainly that only the app port is forwarded and points a CGNAT
host at the self-hosted relay.

### Dialogs

- Create group: group name and your name.
- Join: invite code, then a preview of the host-assigned name and group/host,
  followed by a read-only detected device name and the waiting card while the
  host decides.
- Invite: when the session is off, a warning and `Start session and invite`
  action. Otherwise friend's name, the direct endpoint or relay route, then the
  full code with Copy. Another group's live session blocks the action instead of
  switching sessions.
- Group connection: an `Override Settings` tickbox. Off shows a read-only
  summary of the inherited direct or relay connection. On shows `Use Relay`, an
  optional custom relay for the group, direct fields when relay is off, a live
  preview and inline errors. `Reset` clears all group connection overrides and
  the custom relay password; it is a form action committed by Save. The dialog
  warns before a live restart and when open invites will be cancelled.
- Publish a service: preset picker, name, protocol, game server IP, game server
  port(s), optional friend port, hint, and a live route preview. The IP starts
  at `127.0.0.1` with a visible helper explaining when to use another PC's LAN
  IP. Edit reuses the form prefilled. Inline validation. A friend port is only
  valid with a single game port, and a game port range needs the friend port
  left blank. Loopback targets and administrative ports show a non-blocking
  warning.
- Member details: device, address, fingerprint, first/last seen, approval
  state, automatic reconnect state, inactivity deadline, `Reset reconnect`
  (revokes the token; the friend needs a new invite from then on) and Remove.
- Device approval: friend, device, address, key fingerprint, Allow/Deny. It
  appears wherever the host is in the app and brings the window forward.
- Confirmations for delete group, leave group, end session, disconnect, remove
  service, remove member and cancel invite.

## States and copy

- `Live` means a QUIC connection is up. No invented reachability counts.
- The friend screen says `Host offline`, not `Connected`, when the host is
  unreachable. While retrying it says `Reconnecting`.
- The friend screen says `Host live` only after the connection is established.
- Auto-reconnect stops after 30 minutes or when the host revokes the device.
  The stopped message says so and keeps Reconnect available.
- An invite that is redeemed or expired is not offered for copying.
- A code is only shown again while the app is running. After a restart it is
  gone, and the host creates a new one.
- The guide and settings say what is actually forwarded and what is not.
- On Windows the tray icon lives in the notification area. On Linux it needs a
  StatusNotifier host.

## Styling

`internal/gtkui/style.css` holds the colours, spacing, type sizes and component
styles. Brand fonts and PNGs live in `brand/`. Interface fonts and icons live
in `internal/gtkui/assets/`.

`docs/tuner.html` is retained as a historical design reference. It is not
connected to the GTK renderer.

## Previewing

`make gtk-probe` opens the GTK interface with the project-local configuration.
To open a screen directly, set `LANPARTY_GTK_SCREEN`:

```sh
LANPARTY_GTK_SCREEN=group:<group-id> make gtk-probe
LANPARTY_GTK_SCREEN=service:<group-id> make gtk-probe
```

Supported values include `settings`, `guide`, `create`, `join`, `group:<id>`,
`invite:<id>`, `service:<id>`, `service-detail:<id>`, `member:<id>` and
`delete:<id>`. `LANPARTY_GTK_APP_ID` lets a preview run beside the normal app.
Screenshots for the docs are written to `docs/mockups/`.
