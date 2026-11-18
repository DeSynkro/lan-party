# Invite lifetime

Goal: shorten one-time invite validity from 24 hours to 1 hour.

The host has to be online to approve a device anyway, and a fresh invite is
cheap to create. A shorter window reduces the value of a leaked code without
changing how joining works.

## What changes

- Add `group.InviteTTL = time.Hour`.
- `app.CreateInvite` uses that TTL instead of `24 * time.Hour`.
- `group.AddInvite` uses it for an invite created without an explicit expiry.
- Update the in-app guide, invite dialog, README, guide and security notes.
- Existing invites keep their stored `ExpiresAt`; there is no schema change or
  migration. New invites use one hour.
- Historical 1.0 release notes and the 1.0 changelog stay at 24 hours.

## Trade-off

A friend who waits more than an hour needs a new code. That is acceptable
because the host is online and the invite row already shows the countdown.
There is no setting for the TTL in this change; it stays a fixed one hour.

## Tests

- `CreateInvite` stores an expiry roughly one hour ahead.
- An invite 59 minutes old is still open; one 61 minutes old is expired.
- An old invite with a 24-hour expiry keeps its stored expiry.
- The UI and docs no longer say 24 hours.

## Files likely touched

- `internal/group/group.go`
- `internal/app/app.go`
- `internal/app/app_test.go` or `internal/group/group_test.go`
- `internal/invite/invite_test.go`
- `internal/gtkui/dialogs.go`
- `internal/gtkui/guide.go`
- `README.md`
- `docs/guide.md`
- `docs/security.md`
- `docs/design.md`
