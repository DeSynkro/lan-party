# Invite and service tweaks

Status: approved, 2026-11-19

## Goal

Make hosting less error-prone and publishing local game servers easier:

- an invite cannot be created while its host session is off without starting
  the session first;
- a new service targets this PC by default and explains the LAN IP case;
- a published service can be edited instead of removed and recreated.

## Scope

### Invite guard

- The invite dialog checks the live session when it opens.
- If this group has no session, show: “Session is off. It will start when you
  create this invite.”
- The primary action becomes “Start session and invite”.
- On click, start the session before creating the invite. A start failure keeps
  the dialog open, shows the error, and creates no code.
- If another group's session is live, show which group and disable the primary
  action. Do not end the other session.
- If this group's session is already live, keep the existing flow.

### Game server IP

- Prefill `127.0.0.1` when publishing a new service.
- Accept `localhost` case-insensitively and store it as `127.0.0.1`.
- Keep rejecting arbitrary hostnames.
- Show visible helper text under the entry:

  ```text
  Use 127.0.0.1 for a server or container on this PC.
  For another PC, use that PC’s LAN IP.
  ```

- Keep Docker-specific wording out of the UI; the guide may use Docker as an
  example.

### Edit a service

- Add **Edit** to the service details dialog.
- Reuse the publish form with the existing name, protocol, target, ports and
  hint filled in.
- Keep the service ID.
- Re-check port conflicts against the other services, not the service itself.
- A live session receives the updated service list immediately.
- A failed save leaves the stored service unchanged.

## Not in scope

- Auto-starting a session when a group is created.
- A setting for group-creation auto-start.
- Accepting arbitrary hostnames or resolving DNS.
- Probing whether the game server is reachable.

## Decisions

- The session starts when the invite is created, not when the dialog opens.
- Another group's live session is never ended silently.
- `127.0.0.1` is the saved default; the machine's LAN IP is only shown as
  context in the helper.
- The UI says “container”, not “Docker”.
- Editing keeps the service ID and reuses the publish form.
