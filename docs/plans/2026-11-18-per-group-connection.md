# Per-group connection

Goal: let a host group use its own public address and app port instead of the
global Settings values. Settings stays the default; a group only changes when
the host asks it to.

## Current behaviour

- `settings.json` owns `publicAddress` and `appPort`.
- `CurrentEndpoint`, `CreateInvite` and `StartSession` read those global values.
- The host group card shows the global endpoint and port.
- Its Change button opens global Settings.

## What changes

Add two optional fields to a host group:

- `publicAddress` — hostname or IP, optionally with an outside `host:port`.
- `appPort` — the UDP port the host listens on.

Empty address / port `0` means use the global value. Existing groups load with
both empty and keep working from Settings. The group store version stays `1`,
so no migration is needed.

Invites use the group's effective endpoint. Starting a session binds the
group's effective app port. The global Settings preview keeps using the global
values.

## UI

The host group card keeps the existing endpoint row. The endpoint and the
"UDP ... on this PC" line show the group's effective values. A small `Default`
or `Custom` tag sits next to the endpoint.

Change opens a dialog instead of Settings:

**Connection for <group name>**

> This changes only this group. The defaults live in Settings.

- Public address, with the global value as placeholder.
- App port, with the global port as placeholder.
- A muted line: `Default: home.example.net:51820 · UDP 51820`.
- Preview: `Friends connect to: ...` and `Host listens on UDP ...`.
- Buttons: Use defaults, Cancel, Save. When the group is live and the effective
  app port changes, the primary action becomes **Stop session and apply**.
  Otherwise it stays **Save**.

Settings is renamed **Network defaults** and gets a line saying host groups use
these unless they set their own connection. Leave a field blank in the dialog
to inherit the default.

## Live sessions

- If only the public address changes, Save applies immediately. New invites use
  the new address; existing invites keep the old one. Existing connections are
  unaffected.
- If the effective app port changes while the group is live, the dialog shows a
  warning and the primary action becomes **Stop session and apply**. It only
  appears when the effective port actually changes; changing the address alone
  keeps the normal Save action.
- The warning says: "Changing the app port restarts the session. Everyone will
  disconnect briefly." That is the same disconnect you get from stopping the
  session manually; the warning makes it explicit, and the button is the
  confirmation. No second confirmation dialog.
- Validate the new address and port before stopping. Then stop the session,
  save the group, and start it again on the new port. If the restart fails,
  leave the session stopped and show the error.
- Compare the old and new effective endpoint:
  - **Unchanged**, for example a fixed outside port in the public address: "The
    advertised address stays the same. Update your router forward to the new
    local port." Friends can reconnect once the forward is updated.
  - **Changed**: "The advertised endpoint changes. Existing invites will stop
    working; create new invites after the restart."
- Use defaults can change the address, the port or both. It follows the same
  rules; the stop action appears only if the effective port changes.
- Changing the global app port in Settings while a session is live stays blocked
  when the live group inherits the global port. If the live group has its own
  app-port override, the global port can change safely.
- A group that is not live can change either value at any time; Save applies
  immediately.

## Validation

- Public address uses the existing `config.ValidateAddress`.
- App port is `0` for inherit, otherwise `1–65535`.
- An empty effective public address keeps the existing warning and invite error.
- A used port still fails when the session starts.
- Old invites keep their endpoint; new invites use the current group values.

## Tests

- Override and fallback combinations for the endpoint and port.
- `CreateInvite` uses the group endpoint.
- `StartSession` binds the group port.
- Live-session rules: address-only changes save without stopping; port changes
  stop and restart; `Use defaults` follows the same rule.
- Endpoint unchanged vs changed after a port change, including the warning.
- A failed restart leaves the session stopped and surfaces the bind error.
- Changing the global app port while a live group inherits it stays blocked; a
  group with its own port does not block the global change.
- Old `groups.json` without the fields still loads.
- UI signature includes the overrides so live refresh updates the card.
- Guide section for a group-specific address and port.

## Options considered

1. Reuse the existing Change button with a group dialog. Fits the card, uses the
   existing dialog pattern, adds no height. Chosen.
2. An inline override switch on the session card. Visible at a glance, but adds
   height and form controls to a dense 400×640 screen.
3. A separate Connection card. Clear, but duplicates the endpoint and port and
   makes the group screen longer.
