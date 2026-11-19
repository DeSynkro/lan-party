# Invite and service tweaks implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start a host session when creating an invite, default new services to loopback with LAN-IP guidance, and let hosts edit published services in place.

**Architecture:** A small GTK invite guard uses a testable state helper and a session-ensure function. Service targets normalize `localhost`, expose their host, and gain a validated in-place update in the group and app layers. The existing publish form opens prefilled in edit mode.

**Tech Stack:** Go 1.27.4, GTK4 through gotk4, existing `internal/group`, `internal/app` and `internal/gtkui` packages.

**Spec:** `docs/plans/2026-11-19-invite-and-service-tweaks.md`

## Global Constraints

- Use the project Go toolchain in `.tools/go`; no new dependencies.
- Keep `groups.json` version `1`; existing files must keep loading.
- UI copy is sentence case. Say “container”, not “Docker”.
- Only `localhost` is accepted as a named target; store it as `127.0.0.1`.
- The user handles pushes. Do not push.
- Commit messages are past tense and explain the change.
- `make fmt`, `make test-race` and `make vet` must pass before handoff.

## Review Focus

1. Invite with the session off starts it first; a start failure creates no invite.
2. Another group's live session blocks the invite and is never ended silently.
3. `localhost` works in add and edit forms; arbitrary hostnames stay rejected.
4. Editing ignores its own old port but still detects conflicts with other services; a failed save restores the old service.
5. Existing remapped services reopen with the correct game and friend ports.

---

### Task 1: Normalize localhost and expose the target host

**Files:**
- Modify: `internal/group/service.go`
- Test: `internal/group/service_test.go`

**Interfaces:**
- Produces: `ParseTarget` / `ParseTargetIP` normalize `localhost` case-insensitively to `127.0.0.1`.
- Produces: `func (s Service) GameHost() (string, error)` returns the target host without its optional port.

- [ ] **Step 1: Add the failing tests**

Extend `TestParseTarget` with `localhost`, `LOCALHOST` and `localhost:25565`, expecting `127.0.0.1`, `127.0.0.1` and `127.0.0.1:25565` with loopback true. Extend `TestParseTargetIP` with `localhost` and `LocalHost`, expecting `127.0.0.1`. Extend `TestComposeServiceTarget` with a `localhost` / `25565` case expecting target `127.0.0.1`.

Add `TestGameHost` covering bare IP, IP with port, `localhost`, `localhost:25565`, and an invalid `example.com`.

- [ ] **Step 2: Run the tests to verify they fail**

```sh
make test
```

Expected: failure on the new cases and `GameHost` undefined.

- [ ] **Step 3: Implement**

In `ParseTarget`, before `net.ParseIP(host)`:

```go
if strings.EqualFold(host, "localhost") {
    host = "127.0.0.1"
}
```

Add:

```go
func (s Service) GameHost() (string, error) {
    host, _, _, err := splitTarget(s.Target)
    return host, err
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```sh
make test
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/group/service.go internal/group/service_test.go
git commit -m "Accepted localhost as a loopback service target" -m "localhost now normalizes to 127.0.0.1, and services expose GameHost for the edit form."
```

---

### Task 2: Update published services in the app

**Files:**
- Modify: `internal/group/group.go`
- Test: `internal/group/group_test.go`
- Modify: `internal/app/app.go`
- Test: `internal/app/app_test.go`

**Interfaces:**
- Produces: `func (g *Group) UpdateService(s Service) error`
- Produces: `func (a *App) UpdateService(groupID string, s group.Service) error`

- [ ] **Step 1: Add the failing tests**

Add `TestUpdateService` to `internal/group/group_test.go`:

- Add TCP `mc` on 25565 and UDP `voice` on 25565.
- Update `mc` to name `Minecraft Modded`, target `localhost`, ports `[25566]`.
- Assert the stored ID is still `mc`, the target is `127.0.0.1`, and the port is 25566.
- Add TCP `web` on 25565, then try updating `mc` to TCP 25565. Assert it fails and `mc` is still on 25566.
- Assert updating ID `missing` fails.

Add `TestUpdateService` to `internal/app/app_test.go`:

- Create a group, add `mc`, update its name and port, and assert `Group` returns the new values.

Add `TestUpdateServiceRestoresOnSaveFailure`:

- Add `mc`, break `a.DataDir()` the same way `TestSetGroupConnectionRestoresOnSaveFailure` does, update the name, assert the error, and assert the in-memory service still has the original name.

- [ ] **Step 2: Run the tests to verify they fail**

```sh
make test
```

Expected: `UpdateService` undefined in both packages.

- [ ] **Step 3: Implement `Group.UpdateService`**

Find the service with `s.ID`, return `service not found` if absent, run `ValidateService`, check `portConflict` against every other service, then replace the slice entry.

- [ ] **Step 4: Implement `App.UpdateService`**

Mirror `AddService`:

- Reject a missing group or non-host role.
- Copy the old services for rollback.
- Call `g.UpdateService`.
- Copy the new services, call `saveLocked`, restore the old slice on failure.
- After unlocking, call `host.UpdateServices(services)` when a host is live.

- [ ] **Step 5: Run the tests to verify they pass**

```sh
make test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```sh
git add internal/group/group.go internal/group/group_test.go internal/app/app.go internal/app/app_test.go
git commit -m "Edited published services in place" -m "Services can be replaced by id. Conflicts are checked against the other services only, live sessions receive the new list, and a failed save restores the old list."
```

---

### Task 3: Default new services to loopback and add the edit form

**Files:**
- Modify: `internal/gtkui/service_form.go`
- Test: `internal/gtkui/service_form_test.go`
- Modify: `internal/gtkui/dialogs.go`
- Modify: `internal/gtkui/probe.go`
- Modify: `docs/guide.md`, `docs/design.md`, `docs/ui.md`, `CHANGELOG.md`

**Interfaces:**
- Produces: `func serviceFormValues(s group.Service) (ip, gamePorts, friendPort, hint string, err error)`
- Produces: `func serviceTargetHelpFor(ip string) string`
- Produces: `showServiceForm(g group.Group, existing *group.Service)` plus add/edit wrappers.

- [ ] **Step 1: Add the failing tests**

Add `TestServiceFormValues` with these cases:

- Bare target `192.168.1.50`, port 25565 → IP `192.168.1.50`, game `25565`, friend blank.
- Remapped target `192.168.1.50:25566`, published 25565 → IP `192.168.1.50`, game `25566`, friend `25565`.
- Range `2456-2458` → game `2456-2458`, friend blank.
- `localhost` → IP `127.0.0.1`.
- `example.com` → error.

Add `TestServiceTargetHelpFor`:

- `192.168.1.50` → contains `Use 127.0.0.1` and `192.168.1.50`.
- `this PC` → does not contain `this PC: this PC`.

- [ ] **Step 2: Run the tests to verify they fail**

```sh
make test
```

Expected: both helpers undefined.

- [ ] **Step 3: Implement the form helpers**

`serviceFormValues` uses `s.GameHost()` and `s.GamePorts()`. It fills the friend port only when there is one game port and one published port and they differ. It returns `group.FormatPorts` for the game ports.

`serviceTargetHelpFor` returns:

```text
Use 127.0.0.1 for a server or container on this PC.
For another PC, use that PC’s LAN IP (this PC: <ip>).
```

Omit the parenthetical when the IP is empty or `this PC`.

- [ ] **Step 4: Add add/edit form modes**

Rename the body of `showAddServiceDialog` to `showServiceForm(g, existing *group.Service)`. Keep `showAddServiceDialog` calling it with `nil`, and add `showEditServiceDialog` passing the service.

In the form:

- Prefill the IP with `127.0.0.1`.
- When editing, fill name, IP, game ports, friend port, hint and protocol from `serviceFormValues`.
- Put a visible `help` label with `serviceTargetHelpFor(localIPv4())` under the IP entry.
- Use title `Publish a service` / action `Publish`, or `Edit service` / `Save changes`.
- Call `AddService` with a new ID, or `UpdateService` with the existing ID.
- Keep the error label and preview behavior unchanged.

Add an **Edit** button to `showServiceDialog` that closes the details window and opens `showEditServiceDialog`.

- [ ] **Step 5: Add the preview screen**

In `internal/gtkui/probe.go`, add `service-edit:` next to `service-detail:`; open the first service with `showEditServiceDialog`.

- [ ] **Step 6: Run the tests to verify they pass**

```sh
make test
```

Expected: PASS.

- [ ] **Step 7: Update the docs and changelog**

- `docs/guide.md`: new services default to `127.0.0.1`; `localhost` is accepted; another PC needs that PC's LAN IP; services can be edited from their details.
- `docs/design.md`: `localhost` is a `127.0.0.1` alias; updates replace a service in place after validation.
- `docs/ui.md`: mention the loopback default, visible helper and Edit action.
- `CHANGELOG.md`: add loopback-default and service-edit bullets.

- [ ] **Step 8: Commit**

```sh
git add internal/gtkui/service_form.go internal/gtkui/service_form_test.go internal/gtkui/dialogs.go internal/gtkui/probe.go docs/guide.md docs/design.md docs/ui.md CHANGELOG.md
git commit -m "Defaulted services to loopback and allowed edits" -m "New services start at 127.0.0.1, accept localhost, and show the machine's LAN IP in the helper. The publish form opens prefilled for an existing service and saves it in place."
```

---

### Task 4: Start the session when creating an invite

**Files:**
- Create: `internal/gtkui/invite_form.go`
- Test: `internal/gtkui/invite_form_test.go`
- Modify: `internal/gtkui/dialogs.go`
- Modify: `docs/guide.md`, `docs/ui.md`, `CHANGELOG.md`

**Interfaces:**
- Produces: `inviteDialogState{Warning, Action string; Blocked bool}`
- Produces: `func inviteDialogStateFor(liveGroupID, groupID, liveGroupName string) inviteDialogState`
- Produces: `func ensureInviteSession(state *apppkg.App, groupID string) error`

- [ ] **Step 1: Add the failing dialog-state tests**

Add `TestInviteDialogStateFor` with:

- This group live → no warning, action `Create invite`, not blocked.
- No session → warning `Session is off. It will start when you create this invite.`, action `Start session and invite`, not blocked.
- Another group named `Friday Night` live → warning `The session for Friday Night is live. End it before inviting from this group.`, blocked.
- Another group with no name → warning uses `another group`.

- [ ] **Step 2: Run the test to verify it fails**

```sh
make test
```

Expected: helper undefined.

- [ ] **Step 3: Implement the helper and session guard**

Add `inviteDialogStateFor` with the three states above.

Add `ensureInviteSession`:

- Return nil when the live group is this group.
- Return `the session for <name> is live; end it before inviting from this group` when another group is live; fall back to `another group`.
- Otherwise call `state.StartSession(groupID)`.

- [ ] **Step 4: Wire the guard into the invite dialog**

In `showInviteDialog`:

- Read `LiveGroupID`, look up the live group's name when it differs, and call `inviteDialogStateFor`.
- Show the warning above the name field when non-empty.
- Use the state's action label; disable the primary button when blocked.
- On click, validate the name, call `ensureInviteSession`, and return on error without creating a code.
- On success, create the invite, close the dialog, call `u.showGroup(g.ID)`, and show the code dialog.

- [ ] **Step 5: Run the tests to verify they pass**

```sh
make test
```

Expected: PASS.

- [ ] **Step 6: Add the session-guard test**

Add `TestEnsureInviteSessionStartsThisGroupOnly` with a temp app, a free UDP port for `AppPort`, and two host groups:

- `ensureInviteSession(state, first.ID)` starts the first session.
- `ensureInviteSession(state, second.ID)` returns an error.
- `LiveGroupID` is still `first.ID`.

Use `net.ListenUDP` on `net.IPv4zero:0` to reserve the free port, then close it. Model the app setup on `live_test.go`.

- [ ] **Step 7: Run the tests to verify they pass**

```sh
make test
```

Expected: PASS.

- [ ] **Step 8: Update the docs and changelog**

- `docs/guide.md`: clicking **Invite** with the session off starts it before creating the code; another live group must be ended first.
- `docs/ui.md`: describe the warning and `Start session and invite` action.
- `CHANGELOG.md`: add the invite-guard bullet.

- [ ] **Step 9: Commit**

```sh
git add internal/gtkui/invite_form.go internal/gtkui/invite_form_test.go internal/gtkui/dialogs.go docs/guide.md docs/ui.md CHANGELOG.md
git commit -m "Started the session when creating an invite" -m "The invite dialog explains when the session is off, starts it before creating the code, and creates nothing when the start fails. A session for another group blocks the action instead of switching silently."
```

---

### Task 5: Full verification

- [ ] **Step 1: Format**

```sh
make fmt
```

- [ ] **Step 2: Run the race suite**

```sh
make test-race
```

Expected: PASS for every package.

- [ ] **Step 3: Run vet**

```sh
make vet
```

Expected: exit 0.

- [ ] **Step 4: Build**

```sh
make build
```

Expected: `bin/lan-party` is produced.

- [ ] **Step 5: Compile the Windows backend tests**

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 ./.tools/go/bin/go test -c -o /tmp/opencode/windows-tests/app.test.exe ./internal/app
```

Expected: exit 0.

- [ ] **Step 6: Check the working tree**

```sh
git status --short
git log --oneline -6
```

Expected: clean tree and the four feature commits on top of the existing history. Do not push.
