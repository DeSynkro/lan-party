# Per-group connection implementation plan

Goal: let a host group use its own public address and UDP app port, with the
global Settings values as the default, and shorten invite validity to one hour.

Architecture: the host group stores two optional values. The app resolves them
against the saved settings for the group card, invites and the host listener.
The group card's existing Change button opens a group-only dialog. A live app
port change stops the session, saves, and restarts it on the new port.

Spec: `docs/plans/2026-11-18-per-group-connection.md`
Also: `docs/plans/2026-11-18-invite-lifetime.md`

## Constraints

- Go 1.27.4 from `.tools/go`; project-local caches and XDG directories.
- GTK4 through gotk4 v0.2.2. No new dependencies.
- The window stays 400×640 and not resizable.
- Sentence case copy. No emoji or invented status.
- Empty public address / port `0` means “use the global default”.
- Public addresses keep using `config.ValidateAddress`; app ports are 1–65535.
- The group store version stays `1`; old `groups.json` files keep working.
- `make test` and `make vet` must pass before each commit.
- Commit messages are past tense and explain the change.

## Review focus

1. A group with only one override must keep the other field on the global
   default; editing one value must not silently clear the other.
2. A live session must never keep listening on the old port while new invites
   advertise the new one.
3. The group card and the group dialog must agree on what is the default and
   what is custom.
4. A failed restart after a port change must leave the session stopped and show
   the error, not pretend it is live.
5. Old groups and old invites must keep their stored values.

## Task 1: Store the group connection and resolve it

**Files:**
- Modify: `internal/group/group.go`
- Modify: `internal/app/app.go`
- Test: `internal/app/app_test.go`
- Test: `internal/group/store_test.go`

**Interfaces:**
- Consumes: `config.Settings.PublicAddress`, `config.Settings.AppPort`,
  `config.ValidateAddress`.
- Produces:
  - `group.Group.PublicAddress string` (`json:"publicAddress,omitempty"`)
  - `group.Group.AppPort int` (`json:"appPort,omitempty"`)
  - `app.GroupConnection{PublicAddress string; AppPort int; Custom bool}`
  - `func (a *App) EffectiveConnection(groupID string) (GroupConnection, error)`
  - `func (a *App) GroupEndpoint(groupID string) (string, error)`
  - `func (a *App) GroupAppPort(groupID string) (int, error)`
  - `func (a *App) SetGroupConnection(groupID, publicAddress string, appPort int) error`
  - `func ValidateConnection(publicAddress string, appPort int) error`

- [ ] **Step 1: Write the failing app tests**

Add tests to `internal/app/app_test.go`:

```go
func TestEffectiveConnectionFallsBackToSettings(t *testing.T)
func TestEffectiveConnectionUsesGroupOverrides(t *testing.T)
func TestSetGroupConnectionValidates(t *testing.T)
```

Assertions:

- The first test sets `PublicAddress` and `AppPort` in settings, creates a host
  group, and expects `EffectiveConnection` to return those values with
  `Custom == false`.
- The second sets `SetGroupConnection(g.ID, "group.example.net", 41000)` and
  expects those values with `Custom == true`; then sets `("", 0)` and expects
  the settings values again.
- The third rejects `"https://example.net"`, a port of `70000`, and a friend
  group; it accepts `("", 0)` and a valid address/port.

- [ ] **Step 2: Run the tests to verify they fail**

```sh
export GOMODCACHE=$PWD/.cache/go-mod GOPATH=$PWD/.cache/go \
  GOCACHE=$PWD/.cache/go-build TMPDIR=$PWD/.tmp CGO_ENABLED=1
./.tools/go/bin/go test ./internal/app -run 'TestEffectiveConnection|TestSetGroupConnection' -count=1
```

Expected: compile failure because the fields and methods do not exist.

- [ ] **Step 3: Add the fields and resolver**

Add the two optional fields to `group.Group` after `HostName`.

Add to `internal/app/app.go`:

```go
type GroupConnection struct {
    PublicAddress string
    AppPort       int
    Custom        bool
}

func ValidateConnection(publicAddress string, appPort int) error

func (a *App) EffectiveConnection(groupID string) (GroupConnection, error)

func (a *App) GroupEndpoint(groupID string) (string, error)

func (a *App) GroupAppPort(groupID string) (int, error)

func (a *App) SetGroupConnection(groupID, publicAddress string, appPort int) error
```

Implementation notes:

- `ValidateConnection` trims the address, calls `config.ValidateAddress`, and
  accepts port `0` or `1..65535`.
- `effectiveConnectionLocked(g group.Group) (public string, appPort int)` uses
  the group override when set, otherwise `a.settings`. Callers hold `a.mu`.
- `EffectiveConnection` locks, finds the group, and returns the effective
  values plus `Custom` when either override is set.
- `GroupEndpoint` resolves `publicEndpoint(public, appPort)`.
- `GroupAppPort` returns the effective port.
- `SetGroupConnection` validates, rejects friend groups, saves with
  `saveLocked`, and rejects an app-port change while that group is live. An
  address-only change while live is allowed.
- The store version stays `1`; the new JSON fields are `omitempty`.

- [ ] **Step 4: Run the tests to verify they pass**

```sh
./.tools/go/bin/go test ./internal/app ./internal/group -count=1
```

Expected: PASS.

- [ ] **Step 5: Extend the store round trip**

Add `PublicAddress: "group.example.net"` and `AppPort: 41000` to the host group
in `TestStoreRoundTrip`, then assert both survive the round trip. Add a case
that loads a saved group without the two fields and gets empty/zero values.

- [ ] **Step 6: Commit**

```sh
git add internal/group/group.go internal/app/app.go internal/app/app_test.go internal/group/store_test.go
git commit -m "Added per-group connection overrides"
```

## Task 2: Use the group connection for invites and sessions

**Files:**
- Modify: `internal/app/app.go`
- Test: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `EffectiveConnection`, `effectiveConnectionLocked`,
  `SetGroupConnection`, `StopSession`, `StartSession`.
- Produces: invites and host listeners that use the group values, and the
  live-session rules from the spec.

- [ ] **Step 1: Write the failing tests**

Add to `internal/app/app_test.go`:

```go
func TestCreateInviteUsesGroupConnection(t *testing.T)
func TestExistingInviteKeepsEndpoint(t *testing.T)
func TestStartSessionUsesGroupPort(t *testing.T)
func TestSetGroupConnectionLiveRules(t *testing.T)
func TestSaveSettingsPortRules(t *testing.T)
```

Assertions:

- `CreateInvite` with `SetGroupConnection(g.ID, "group.example.net", 41000)`
  decodes to `group.example.net:41000`. A public address with an outside port
  keeps that outside port.
- After an invite is created, changing the group connection does not rewrite
  that code: `InviteCode` still decodes to the old endpoint, while a new invite
  decodes to the new one.
- `StartSession` uses the group port: hold a UDP socket on the global app port,
  set the group app port to another free port, and expect the session to start.
  Use free UDP ports in these tests; do not assume 51820 is available.
- While live, `SetGroupConnection` rejects a different effective port, accepts
  an address-only change, and accepts the same effective port. After
  `StopSession`, the port change succeeds.
- `SaveSettings` rejects a global app-port change when the live group inherits
  the global port, and allows it when the live group has its own app port.

- [ ] **Step 2: Run the tests to verify they fail**

```sh
./.tools/go/bin/go test ./internal/app -run 'TestCreateInviteUsesGroup|TestStartSessionUsesGroupPort|TestSetGroupConnectionLiveRules|TestSaveSettingsPortRules' -count=1
```

Expected: failures showing the global settings are still used, or the live
rules are missing.

- [ ] **Step 3: Switch `CreateInvite` and `StartSession` to the resolver**

- In `CreateInvite`, replace the global `public`/`appPort` reads with
  `public, appPort := a.effectiveConnectionLocked(g)` while holding `a.mu`.
- In `StartSession`, replace `appPort := a.settings.AppPort` with the group’s
  effective port.
- In `SaveSettings`, keep the “end the session before changing the app port”
  error only when the live group has `AppPort == 0`. A live group with an
  override does not block the global default change.

- [ ] **Step 4: Run the tests to verify they pass**

```sh
./.tools/go/bin/go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```sh
git add internal/app/app.go internal/app/app_test.go
git commit -m "Used group connections for invites and sessions"
```

## Task 3: Add the group connection dialog and card state

**Files:**
- Modify: `internal/gtkui/group.go`
- Modify: `internal/gtkui/dialogs.go`
- Modify: `internal/gtkui/settings.go`
- Test: manual GTK check; no new automated UI test.

**Interfaces:**
- Consumes: `EffectiveConnection`, `GroupEndpoint`, `GroupAppPort`,
  `SetGroupConnection`, `ValidateConnection`, `StopSession`, `StartSession`.
- Produces: `(*probeUI).showGroupConnectionDialog(g group.Group, showError func(error))`.

- [ ] **Step 1: Update the host session card**

- Replace `u.state.CurrentEndpoint()` with `u.state.GroupEndpoint(g.ID)`.
- Show a `Default` tag when `g.PublicAddress == "" && g.AppPort == 0`, otherwise
  `Custom`. Reuse the existing `tagLabel` helper.
- Replace the app-port line with `u.state.GroupAppPort(g.ID)`.
- Point the Change button at `u.showGroupConnectionDialog(g, showError)`.
- Include `g.PublicAddress`, `g.AppPort`, the effective endpoint and the
  effective app port in `groupSignature` so the live refresh updates the card.

- [ ] **Step 2: Build the dialog**

Add `showGroupConnectionDialog` following the existing dialog pattern. Set a
larger fixed size, about 380×480, so the fields and warning fit.

Copy:

- Title: `Connection for <group name>`
- Top help: `This changes only this group. The defaults live in Settings.`
- Fields: `Public address`, `App port (UDP)`
- Placeholders: the global value, or `home.example.net` / `51820`.
- Default line: `Default: <global endpoint> · UDP <global port>`
- Preview: `Friends connect to: <endpoint>` and
  `Host listens on UDP <port>`.
- Buttons: `Use defaults`, `Cancel`, and the primary action.

The primary action is `Save` normally. When the group is live and the effective
app port changes, it becomes `Stop session and apply`, and a wrapped warning
appears: `Changing the app port restarts the session. Everyone will disconnect
briefly.`

Add the endpoint note:

- Endpoint unchanged: `The advertised address stays the same. Update your
  router forward to the new local port.`
- Endpoint changed: `The advertised endpoint changes. Existing invites will
  stop working; create new invites after the restart.`

- [ ] **Step 3: Wire saving and restart**

- Validate with `ValidateConnection` before stopping anything.
- If the group is not live, or the effective port is unchanged, call
  `SetGroupConnection`, close, and refresh the group page.
- If the group is live and the effective port changes, call `StopSession`, then
  `SetGroupConnection`, then `StartSession`.
- If `StartSession` fails, close the dialog, refresh the group page, and show
  `settings saved, but the session could not restart: <error>`.
- `Use defaults` clears both entries and reruns the preview/button update.

- [ ] **Step 4: Make the default/custom split clear in Settings**

Rename the Settings section from `Network` to `Network defaults` and add:
`Host groups use these defaults unless they set their own connection.`

- [ ] **Step 5: Run formatting, vet and tests**

```sh
make fmt
make vet
make test
```

Expected: all pass, with no GTK compile errors.

- [ ] **Step 6: Manual check**

Run `make run` with a clean project config and check:

1. A host group with no overrides shows `Default` and the Settings endpoint.
2. Change only the address; the card shows `Custom` and the new endpoint, and a
   new invite uses it.
3. Start a session. Change only the address; the button stays `Save`.
4. Change the app port; the warning and `Stop session and apply` appear. Apply;
   the session stops, saves, restarts on the new port, and the card shows it.
5. Try a port already in use; the session stays stopped and the error is shown.
6. Clear both overrides; the card returns to `Default`.

- [ ] **Step 7: Commit**

```sh
git add internal/gtkui/group.go internal/gtkui/dialogs.go internal/gtkui/settings.go
git commit -m "Added the group connection dialog"
```

## Task 4: Shorten invites to one hour

**Files:**
- Modify: `internal/group/group.go`
- Modify: `internal/app/app.go`
- Modify: `internal/invite/invite_test.go`
- Test: `internal/app/app_test.go`
- Test: `internal/group/group_test.go`

**Interfaces:**
- Produces: `group.InviteTTL = time.Hour`; `CreateInvite` and the `AddInvite`
  fallback use it.

- [ ] **Step 1: Write the failing tests**

Add:

```go
func TestCreateInviteExpiryIsOneHour(t *testing.T) // internal/app
func TestAddInviteUsesOneHourDefault(t *testing.T) // internal/group
```

Assertions:

- `CreateInvite` stores `ExpiresAt - CreatedAt` within one minute of
  `group.InviteTTL`.
- `AddInvite` with a zero `ExpiresAt` uses `CreatedAt + InviteTTL`.
- An invite 59 minutes old is open; one 61 minutes old is expired.

- [ ] **Step 2: Run the tests to verify they fail**

```sh
./.tools/go/bin/go test ./internal/app ./internal/group -run 'OneHour|Expiry' -count=1
```

Expected: the app still uses 24 hours.

- [ ] **Step 3: Add the TTL and use it**

Add `const InviteTTL = time.Hour` to `internal/group/group.go`. Replace the two
`24 * time.Hour` uses in `app.CreateInvite` and `group.AddInvite` with it.
Update the `invite` test fixture from 24 hours to one hour. Leave historical
1.0 release notes and the 1.0 changelog alone.

- [ ] **Step 4: Update the copy**

- `internal/gtkui/dialogs.go`: `expires after 24 hours` → `expires after 1 hour`.
- `internal/gtkui/guide.go`: both 24-hour lines → `1 hour`.
- `README.md`: feature and troubleshooting lines → `1 hour`.
- `docs/guide.md`: all three 24-hour references → `1 hour`.
- `docs/security.md`: both 24-hour references → `1 hour`.
- `docs/design.md`: both 24-hour references → `1 hour`.

- [ ] **Step 5: Run tests and commit**

```sh
make test
git add internal/group/group.go internal/app/app.go internal/app/app_test.go \
  internal/group/group_test.go internal/invite/invite_test.go \
  internal/gtkui/dialogs.go internal/gtkui/guide.go README.md \
  docs/guide.md docs/security.md docs/design.md
git commit -m "Shortened invites to one hour"
```

## Task 5: Document per-group connections and verify

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`
- Modify: `features-to-be-added.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/plans/2026-11-18-per-group-connection.md` only if a decision
  changed during implementation.

- [ ] **Step 1: Add the guide section**

Under Hosting, add a short “Use a different address or port for one group”
section:

- Change on the host group card.
- Blank fields use the Settings defaults.
- The address can include an outside port.
- Changing the app port restarts the session; everyone disconnects briefly.
- Update the router forward.
- If the advertised endpoint changes, create new invites.
- The public address can change while live; new invites use it.

- [ ] **Step 2: Add the README feature**

Add to the Features list:

`**Per-group connection:** a host group can use its own public address and UDP
port; blank fields use the Settings defaults.`

- [ ] **Step 3: Update the backlog and changelog**

Remove the “Per-group public address and app port” section from
`features-to-be-added.md`, or move it to a short “Done in 1.1” line. Do the same
for the friend-side bind only when that feature is actually implemented.

Add a `## 1.1.0 — Unreleased` entry to `CHANGELOG.md` covering per-group
connections and one-hour invites. The release date is set when 1.1 ships.

- [ ] **Step 4: Final verification**

```sh
make fmt
make vet
make test-race
make build
./.tools/go/bin/go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -mode=binary ./bin/lan-party
make appimage
```

Build the AppImage and run the manual checks from Task 3 once more.

- [ ] **Step 5: Commit**

```sh
git add README.md docs/guide.md features-to-be-added.md CHANGELOG.md
git commit -m "Documented per-group connections and one-hour invites"
```
