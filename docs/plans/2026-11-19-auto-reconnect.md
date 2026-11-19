# Automatic friend reconnect

Status: approved, 2026-11-19

## Goal

Reconnect a friend automatically when the host restarts or the connection
drops, without polling aggressively or requiring a dialog.

## Behavior

- New setting: **Reconnect automatically if the connection drops**, default on.
- Only a friend session that was live and drops during this run is retried. A
  fresh app launch does not auto-join.
- Retry delays: 1s, 2s, 5s, 10s, 15s, 30s.
- Every 30s until 10 minutes after the drop.
- Every 60s from 10 to 30 minutes.
- Stop after 30 minutes.
- The manual **Reconnect** button cancels the current timer, resets the
  schedule and attempts immediately. If the attempt fails, the new schedule
  starts at 1s.
- Stop immediately on Disconnect, Leave group, starting or joining another
  session, app close, host revocation, or an explicit denial.
- Friend card shows `Reconnecting…` with the next retry, then a stopped message
  after 30 minutes.

## Endpoint warning

The group connection dialog shows the “endpoint changes, invites are cancelled”
note only while at least one invite is open. With no open invites the note is
hidden. The session-restart and router-forward notes are unchanged.

## Technical notes

- `session.Friend` exposes a done channel and whether the host revoked it.
- `App` owns one reconnect state machine and one goroutine at a time.
- A pure schedule function computes the next delay from the attempt number and
  elapsed time.
- The UI reads a small reconnect status from `App` during its existing
  one-second refresh.

## Not in scope

- Auto-joining on app launch.
- Retrying forever.
- Reconnecting after the device was revoked by the host.
- A per-group reconnect queue; only one session exists at a time.
