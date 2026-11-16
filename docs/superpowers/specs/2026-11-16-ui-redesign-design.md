# LAN Party UI redesign

Date: 2026-11-16
Status: approved after review in conversation

## Intent

Rebuild the Fyne interface so the product looks like a finished desktop utility:
one calm screen per job, obvious state, and every existing feature reachable
without reading small grey text.

## Why now

The current screens are clean but unfinished. Home is a centred splash with a
large empty band. Group state is duplicated in three places. The public-address
action exists in code but is unreachable from the UI, so a host cannot make an
invite. Service cards claim reachability that was never measured, and the roster
shows tunnel IPs. The friend screen drops the invite's "where to paste" hint.

## Goals

- A compact single-column app at 500×640 with a real app bar and no footer.
- Clear hierarchy: one bright element per screen, quiet chrome.
- Every feature visible: groups, host/join, session start/stop, public address,
  services, invites, presence, friend addresses, settings and version.
- Truthful state only: no reachability or connection claim without evidence.
- Warm, product-specific visual identity rather than generic dark developer UI.

## Non-goals

- No new networking features, relay, port-conflict detection or live control
  channel.
- No second theme, no light mode.
- No new fonts or dependencies.
- No rewrite of the Fyne window chrome or the dialogs' native frame.
- No game launching or server-list writing.

## Constraints

- Go 1.27.4 pinned by `.tools`, Fyne 2.8.2, no new modules.
- Window stays fixed at 500×640 (`app.go`); content column max 500.
- Page padding 24, 4px spacing grid, cards 12px radius, buttons 8px.
- IBM Plex Sans for UI, JetBrains Mono for addresses and figures.
- Dark only. No gradients, glows, coloured shadows, emoji, or looping motion.
- All copy sentence case, no exclamation marks, no invented data.
- Roster never shows tunnel IP addresses; presence comes from handshakes only.

## Approaches considered

**A. Compact one column (chosen).** Keep the tray-utility character, widen the
window to 500×640, rebuild the shell and each screen around one column.
Fastest, fits beside a game, and the existing problem is hierarchy rather than
width.

**B. Wider two-column control panel (rejected).** About 760×640 with session
and services left, people and invites right. More visible at once, but changes
the product's feel and is significantly more work.

**C. Keep the current layout and polish (rejected).** Does not fix the splash
composition, duplicated state, missing address action or the debug footer.

## Direction

- **Product:** a desktop utility that gives a friend one loopback port onto your
  LAN game server, and nothing else.
- **Character:** precise, quiet, a little nostalgic.
- **World:** a network switch on a desk at 11pm: warm graphite case, copper
  patch cable, silkscreen labels, green link LEDs.
- **Moments:** the session/link panel with its live LED and mono port; the
  packet-path strip on home; the large loopback address on the friend screen.
- **Signature:** a 6px link LED plus mono port numbers, used on the session,
  service cards and roster, nowhere else.

### Palette

| Role | Value |
| --- | --- |
| Background | `#141210` |
| Card | `#1b1815` |
| Raised | `#24201c` |
| Hairline | `#2f2a24` |
| Text | `#efe9df` |
| Secondary | `#b3aa9e` |
| Muted | `#8b8378` |
| Copper accent | `#cf8b57` |
| Live | `#6fcf97` |
| Warning | `#d9b25c` |
| Danger | `#d46a63` |
| Primary button fill | `#ece5d9` |
| Primary button ink | `#1a1511` |

Copper marks loopback ports and links. Green means a verified live link.
Off-white is the primary action. No surface is tinted; depth is one surface step
plus a hairline.

### Type

- App bar title 15–16, semibold.
- Home headline 24, semibold.
- Names 14, regular or semibold.
- Body 13, regular.
- Meta and section labels 12, secondary or muted.
- Addresses 14; friend hero address 16; mono.

## Screens

### Shell

App bar 56px: small keyhole mark + `LAN Party` on home, back arrow + group name
on group/friend, one right-hand action (Settings or Disconnect). Content scrolls
under it. No footer.

### Home

App bar: keyhole mark, `LAN Party`, Settings.

Body: headline `One port, not the network.`, one short explanation, then
**Host a group** (primary) and **Join with an invite** (secondary). Saved groups
appear in `Your groups` and `Groups you joined`; each row shows `host · 2 members`
or `joined · hosted by DeSynkro`, plus a live LED when a session is running for
that group. Closing panel: `How it works`, with
`192.168.1.50:25565 ── encrypted ── 127.0.0.1:25565` and one line saying only
published ports cross. The empty state is vertically composed around this
panel, so there is no dead band.

### Group

App bar: back arrow, group name, Settings. Meta: `Hosting as DeSynkro · 2 members`.

**Session panel**

- Idle, no address: muted LED, `Session off`, **Start session**; row
  `Public address  Not set — friends can't join  [Set address]`.
- Idle, address: address in mono with Change; `UDP port 51820`.
- Live: green LED, `Session live`, End session; `Connected 0 of 2`, address,
  port.
- Start success sets `App.liveGroupID`; end clears it.
- The existing `setEndpoint()` becomes reachable from this panel.

**Published**

Section header with count. One card per service: name and protocol,
`Friends open 127.0.0.1:25565` with the port in copper, `On your LAN
192.168.1.50:25565`. Multi-port services use `group.FormatPorts`. No fake
"can reach" count. Add row reads **Publish a service**; while live it is
disabled with the reason. Removing a service asks for confirmation and closes
the detail dialog first.

**People**

Section header with count and an **Invite** action. Roster rows show host,
`you`, and peers with green/grey LEDs and `live`, `waiting` or `idle`. No
`100.64.0.x` tags. Open invites appear under the roster as
`Invite for Ash · expires in 23h` with Copy and `Copied` feedback. Expired
invites are hidden. The pinned bottom invite card is removed.

### Friend

App bar: group name, Disconnect. Status: muted LED, `Session open`, hosted by.
**Ready to play** groups bound ports by service; each card has the service name,
protocol, a 16px mono address with the port in copper, Copy with feedback, the
invite's hint (for example `Direct Connect in Minecraft`), and extra addresses
for multi-port services. Empty state: `No service open` and the host has not
published anything. `Forget this group` sits after a divider and asks for
confirmation. Disconnect also asks for confirmation.

### Settings

Themed dialog with a `Startup` section: switch rows for **Start minimised** and
**Start on login**, each with a short description. A note explains that closing
the window keeps a live session in the tray. Footer: `LAN Party · dev build`.
Autostart failure shows an error and reverts the switch.

## Truthfulness fixes

- `session.Bound` gains `Hint` so the friend screen can show where to paste.
- `App.liveGroupID` lets home show which saved group is currently live.
- `connectedLabel` counts handshakes within two minutes; it never uses member
  count.
- Remove `n of n can reach` and roster IP tags.
- Friend state says `Session open`, not `Connected`, until host reachability is
  actually known.
- Removing a service and forgetting a group are confirmed.
- Keyhole icon is recoloured copper/off-white; the mark shape stays.

## What is left to the builder

- Exact spacers and vertical centring of the empty home, tuned from rendered
  screenshots.
- Whether long names are shortened with an ellipsis (implemented) and the exact
  character budget.
- Toggle visual details and hover treatment inside the palette.
- Dialog copy refinements that keep the same actions and states.
