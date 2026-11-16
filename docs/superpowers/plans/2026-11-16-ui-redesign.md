# LAN Party UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Fyne interface as a finished 500×640 desktop utility with a warm graphite/copper look, clear hierarchy, truthful status, and every existing feature reachable.

**Architecture:** Keep the existing `App` screens and Fyne 2.8.2 stack. Introduce a warm palette and a small set of shared components (app bar, section header, status dot, key/value rows, address line, list rows, switch rows). Rewrite each screen against those components, then remove the old splash/footer widgets. Two small data additions — `session.Bound.Hint` and `App.liveGroupID` — let the UI show the invite hint and the live saved group without inventing state.

**Tech Stack:** Go 1.27.4 (project-pinned), Fyne 2.8.2, embedded IBM Plex Sans and JetBrains Mono, Fyne test renderer.

**Spec:** `docs/superpowers/specs/2026-11-16-ui-redesign-design.md`

## Global Constraints

- Window: fixed 500×640; content column max 500; page padding 24; 4px grid.
- Palette exactly: background `#141210`, card `#1b1815`, raised `#24201c`, hairline `#2f2a24`, text `#efe9df`, secondary `#b3aa9e`, muted `#8b8378`, copper `#cf8b57`, live `#6fcf97`, warning `#d9b25c`, danger `#d46a63`, primary fill `#ece5d9`, primary ink `#1a1511`.
- Type: IBM Plex Sans; JetBrains Mono for addresses and version. Sizes 12/13/14/15/16/24. Weights 400–600 only.
- No new Go modules or fonts. No emoji. No gradients, glows, coloured shadows, looping motion, or status pills.
- Copy: sentence case, no exclamation marks, no invented reachability or connection claims.
- Roster never shows box addresses (`100.64.0.x`).
- Every task must leave `make test` green; run `make fmt` before committing.
- Do not stage the user's untracked `docs/mockups/` directory.

## Review Focus

- **Long names and multi-port services:** a 40-character group or service name and a 3-port service must not overflow the 500px window or collapse neighbouring rows; ellipsis shortening and `group.FormatPorts` are exercised.
- **Missing endpoint:** a host with no public address must see a working `Set address` action, and an invite attempt must not dead-end; both empty and set states are rendered.
- **Invite states:** expired invites must never be offered for copy; no-invite and open-invite states must both render clearly.
- **Live transitions:** starting and ending a session, plus the 2-second presence timer, must rebuild or repaint the correct labels without stale text or nil panics; add/remove service must explain why they are disabled while live.
- **Truthfulness and privacy:** no box IPs in the roster, no `n of n can reach`, no green friend status without evidence; screen-text tests assert the absence of these strings.

---

### Task 1: Warm palette, type sizes, icon

**Files:**
- Modify: `internal/ui/theme.go`
- Create: `internal/ui/theme_test.go`
- Modify: `build/icon/main.go`
- Regenerate: `internal/ui/assets/icon.png` (via `make icon`)

**Interfaces:**
- Produces package-level colour values used by every later task:
  `colourBackground`, `colourSurface`, `colourRaised`, `colourHairline`, `colourText`, `colourSecondary`, `colourMuted`, `colourAccent`, `colourLive`, `colourWarning`, `colourDanger`, `colourPrimaryFill`, `colourPrimaryInk` (all `color.Color`).
- `palette` remains `map[fyne.ThemeColorName]color.Color` and is populated from those values.
- `NewTheme() fyne.Theme` keeps its signature.

- [ ] **Step 1: Write the failing contrast test**

Create `internal/ui/theme_test.go` with:

```go
package ui

import (
	"image/color"
	"math"
	"testing"
)

func TestPaletteContrast(t *testing.T) {
	bg := palette[ColourBackground]
	surface := palette[ColourSurface]
	pairs := []struct {
		name string
		fg   color.Color
		bg   color.Color
	}{
		{"text on background", palette[ColourText], bg},
		{"secondary on background", palette[ColourSecondary], bg},
		{"muted on background", palette[ColourMuted], bg},
		{"text on surface", palette[ColourText], surface},
		{"secondary on surface", palette[ColourSecondary], surface},
		{"muted on surface", palette[ColourMuted], surface},
		{"accent on background", palette[ColourAccent], bg},
		{"live on background", palette[ColourSuccess], bg},
		{"warning on background", palette[ColourWarning], bg},
		{"danger on background", palette[ColourDanger], bg},
		{"primary button", hex("#1a1511"), hex("#ece5d9")},
	}
	for _, p := range pairs {
		if got := contrastRatio(p.fg, p.bg); got < 4.5 {
			t.Errorf("%s contrast = %.2f, want >= 4.5", p.name, got)
		}
	}
}

func contrastRatio(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v) / 65535
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `make test`
Expected: FAIL in `TestPaletteContrast`; the current muted `#6f737d` on `#0f1013` is below 4.5.

- [ ] **Step 3: Replace the palette and sizes**

In `internal/ui/theme.go`, add the named colour vars:

```go
var (
	colourBackground  = hex("#141210")
	colourSurface     = hex("#1b1815")
	colourRaised      = hex("#24201c")
	colourHairline    = hex("#2f2a24")
	colourText        = hex("#efe9df")
	colourSecondary   = hex("#b3aa9e")
	colourMuted       = hex("#8b8378")
	colourAccent      = hex("#cf8b57")
	colourLive        = hex("#6fcf97")
	colourWarning     = hex("#d9b25c")
	colourDanger      = hex("#d46a63")
	colourPrimaryFill = hex("#ece5d9")
	colourPrimaryInk  = hex("#1a1511")
)
```

Populate `palette` from them and map every stock Fyne colour name relevant to dialogs and entries: `Background`, `Button`, `DisabledButton`, `Disabled`, `Error`, `Focus`, `Foreground`, `ForegroundOnError`, `ForegroundOnPrimary`, `ForegroundOnSuccess`, `ForegroundOnWarning`, `HeaderBackground`, `Hover`, `Hyperlink`, `InnerWindowBorder`, `InnerWindowBorderInactive`, `InputBackground`, `InputBorder`, `MenuBackground`, `OverlayBackground`, `PlaceHolder`, `Pressed`, `Primary`, `ScrollBar`, `ScrollBarBackground`, `Selection`, `Separator`, `Shadow`, `Success`, `Warning`.

Use: `Primary = colourAccent`, `ForegroundOnPrimary = colourPrimaryInk`, `Hyperlink = colourAccent`, `Focus = colourAccent`, `Selection = #4a3425`, `Success = colourLive`, `Warning = colourWarning`, `Error = colourDanger`. Keep `theme.ColorNameShadow` black at 160 alpha.

Theme sizes: text 13, caption 12, subheading 15, heading 20, input radius 8, selection radius 6, scrollbar 6/3, separator 1.

- [ ] **Step 4: Recolour the keyhole icon**

In `build/icon/main.go`, keep the rounded border and keyhole shape; change `background` to `#1b1815` and `accent` to `#cf8b57`.

Run: `make icon`
Expected: `internal/ui/assets/icon.png` rewritten; `file` still reports a 512×512 PNG.

- [ ] **Step 5: Verify and commit**

Run: `make fmt && make test`
Expected: all tests PASS, including `TestPaletteContrast`.

```bash
git add internal/ui/theme.go internal/ui/theme_test.go build/icon/main.go internal/ui/assets/icon.png
git commit -m "style: warm graphite palette and keyhole icon"
```

---

### Task 2: Truthful state plumbing

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/session/session_test.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/session_test.go`

**Interfaces:**
- `session.Bound` gains `Hint string`.
- `session.Friend.Bound()` keeps returning `[]Bound`; each entry carries the invite service hint.
- `App` gains `liveGroupID string`; set by `startSession`, cleared by `endSession` and `closeSessions`.

- [ ] **Step 1: Write the failing `Bound.Hint` test**

Append to `internal/session/session_test.go`:

```go
func TestBoundCarriesInviteHint(t *testing.T) {
	echoAddr := startTCPEcho(t)
	_, inv := startSessionPair(t, group.Service{
		ID: "svc-tcp", Name: "Minecraft", Proto: "tcp",
		Ports: []int{25565}, Target: echoAddr,
	})
	inv.Services[0].Hint = "Direct Connect in Minecraft"

	friend, err := StartFriend(FriendConfig{Invite: inv})
	if err != nil {
		t.Fatalf("start friend: %v", err)
	}
	defer friend.Close()

	if got := friend.Bound()[0].Hint; got != "Direct Connect in Minecraft" {
		t.Fatalf("hint = %q, want %q", got, "Direct Connect in Minecraft")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `make test`
Expected: compile FAIL because `session.Bound` has no `Hint`.

- [ ] **Step 3: Add `Hint` and populate it**

In `internal/session/session.go`, add `Hint string` to `Bound`. In both `f.bound = append(...)` calls in `StartFriend`, set `Hint: svc.Hint`.

- [ ] **Step 4: Write the failing live-group test**

In `internal/ui/session_test.go`, extend `TestStartAndEndSession` after `app.startSession(g)`:

```go
if app.liveGroupID != g.ID {
	t.Fatalf("liveGroupID = %q, want %q", app.liveGroupID, g.ID)
}
```

and after `app.endSession(g)`:

```go
if app.liveGroupID != "" {
	t.Fatalf("liveGroupID = %q after end, want empty", app.liveGroupID)
}
```

- [ ] **Step 5: Implement `liveGroupID`**

In `internal/ui/app.go`, add `liveGroupID string` to `App`, set `a.liveGroupID = g.ID` immediately after `a.live = live` in `startSession`, and clear it in `endSession` and `closeSessions`.

- [ ] **Step 6: Verify and commit**

Run: `make fmt && make test`
Expected: PASS.

```bash
git add internal/session/session.go internal/session/session_test.go internal/ui/app.go internal/ui/session_test.go
git commit -m "feat: carry service hints and live group id"
```

---

### Task 3: Shared components, formatting helpers, window size

**Files:**
- Create: `internal/ui/format.go`
- Create: `internal/ui/format_test.go`
- Create: `internal/ui/components.go`
- Create: `internal/ui/components_test.go`
- Modify: `internal/ui/widgets.go`
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/home.go`, `internal/ui/group.go`, `internal/ui/friend.go` (column max only)
- Modify: `internal/ui/screens_test.go`, `internal/ui/session_test.go`, `internal/ui/join_test.go` (window size 500×640 and test helpers)

**Interfaces:**
- `const appWidth float32 = 500`
- `func addressParts(address string) (host, port string)`
- `func loopbackPreview(s group.Service) string`
- `func openInvites(codes []group.Code, now time.Time) []group.Code`
- `func connectedLabel(g group.Group, presence map[string]time.Time, now time.Time) string`
- `type boundGroup struct { Name, Proto, Hint string; Addresses []string }`
- `func groupBounds(bounds []session.Bound) []boundGroup`
- `func shorten(s string, max int) string`
- `func appBar(leading fyne.CanvasObject, title string, trailing fyne.CanvasObject) fyne.CanvasObject`
- `func page(bar, body fyne.CanvasObject) fyne.CanvasObject` (scrolls)
- `func pageCentred(bar, body fyne.CanvasObject) fyne.CanvasObject` (fills, vertical spacers)
- `func sectionHeader(title string, count int, action fyne.CanvasObject) fyne.CanvasObject`
- `func statusDot(c color.Color) *canvas.Circle`
- `func statusLine(text string, size float32, textColour, dotColour color.Color) fyne.CanvasObject`
- `func keyValueRow(label string, value, action fyne.CanvasObject) fyne.CanvasObject`
- `func addressLine(address string, size float32) fyne.CanvasObject`
- `func monoText(text string, size float32, c color.Color) *canvas.Text`
- `func divider() fyne.CanvasObject`
- `func pairRow(left, right fyne.CanvasObject, gap float32) fyne.CanvasObject`
- `func appMark() fyne.CanvasObject`
- `func (a *App) copyButton(label, value string, kind buttonKind, small bool) *button`
- `type switchRow`, `func newSwitchRow(title, description string, checked bool, onChanged func(bool)) *switchRow`, `func (s *switchRow) SetChecked(bool)`
- `type listRow`, `func newNavRow(title, meta string, live bool, onTap func()) *listRow`, `func newActionRow(title, meta string, disabled bool, onTap func()) *listRow`

- [ ] **Step 1: Write failing helper tests**

Create `internal/ui/format_test.go`:

```go
package ui

import (
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

func TestAddressParts(t *testing.T) {
	host, port := addressParts("127.0.0.1:25565")
	if host != "127.0.0.1:" || port != "25565" {
		t.Fatalf("got %q %q", host, port)
	}
	if host, port := addressParts("no-port"); host != "no-port" || port != "" {
		t.Fatalf("got %q %q for bare string", host, port)
	}
}

func TestLoopbackPreview(t *testing.T) {
	if got := loopbackPreview(group.Service{Ports: []int{25565}}); got != "127.0.0.1:25565" {
		t.Fatalf("got %q", got)
	}
	if got := loopbackPreview(group.Service{Ports: []int{2456, 2457, 2458}}); got != "127.0.0.1:2456-2458" {
		t.Fatalf("got %q", got)
	}
}

func TestOpenInvites(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	codes := []group.Code{
		{PeerName: "Old", Expires: now.Add(-time.Hour)},
		{PeerName: "Soon", Expires: now.Add(time.Hour)},
		{PeerName: "Later", Expires: now.Add(3 * time.Hour)},
	}
	got := openInvites(codes, now)
	if len(got) != 2 || got[0].PeerName != "Later" || got[1].PeerName != "Soon" {
		t.Fatalf("got %#v", got)
	}
}

func TestConnectedLabel(t *testing.T) {
	now := time.Now()
	g := group.Group{Members: []group.Member{
		{Name: "Host", IsHost: true},
		{Name: "Ash", PublicKey: "ash"},
		{Name: "Pip", PublicKey: "pip"},
	}}
	presence := map[string]time.Time{"ash": now.Add(-30 * time.Second)}
	if got := connectedLabel(g, presence, now); got != "1 of 2 connected" {
		t.Fatalf("got %q", got)
	}
	if got := connectedLabel(group.Group{Members: []group.Member{{Name: "Host", IsHost: true}}}, nil, now); got != "No one invited yet" {
		t.Fatalf("got %q", got)
	}
}

func TestGroupBounds(t *testing.T) {
	bounds := []session.Bound{
		{Service: "Valheim", Proto: "udp", Address: "127.0.0.1:2456", Hint: "Join Game"},
		{Service: "Valheim", Proto: "udp", Address: "127.0.0.1:2457"},
		{Service: "Minecraft", Proto: "tcp", Address: "127.0.0.1:25565"},
	}
	got := groupBounds(bounds)
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2", len(got))
	}
	if got[0].Name != "Valheim" || len(got[0].Addresses) != 2 || got[0].Hint != "Join Game" || got[0].Proto != "UDP" {
		t.Fatalf("Valheim = %#v", got[0])
	}
	if got[1].Name != "Minecraft" || got[1].Proto != "TCP" {
		t.Fatalf("Minecraft = %#v", got[1])
	}
}

func TestGroupBoundsMergesProtocols(t *testing.T) {
	got := groupBounds([]session.Bound{
		{Service: "Echo", Proto: "tcp", Address: "127.0.0.1:25565"},
		{Service: "Echo", Proto: "udp", Address: "127.0.0.1:25565"},
	})
	if len(got) != 1 || got[0].Proto != "TCP + UDP" || len(got[0].Addresses) != 1 {
		t.Fatalf("got %#v", got)
	}
}

func TestShorten(t *testing.T) {
	if got := shorten("abcdefghij", 5); got != "abcd…" {
		t.Fatalf("got %q", got)
	}
	if got := shorten("abc", 5); got != "abc" {
		t.Fatalf("got %q", got)
	}
}
```

Create `internal/ui/components_test.go`:

```go
package ui

import "testing"

func TestSwitchRowToggles(t *testing.T) {
	got := false
	row := newSwitchRow("Start minimised", "Open in the tray", false, func(on bool) { got = on })
	row.Tapped(nil)
	if !row.checked || !got {
		t.Fatalf("row did not toggle on")
	}
	row.SetChecked(false)
	if row.checked || !got {
		t.Fatalf("SetChecked changed callback or failed")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `make test`
Expected: compile FAIL with undefined helpers/types.

- [ ] **Step 3: Implement `format.go`**

Add the pure helpers with these exact semantics:

- `openInvites` walks the slice backwards, keeps only `Expires.After(now)`, preserving newest-first order.
- `connectedLabel` counts non-host members with a handshake within 2 minutes; returns `"No one invited yet"` when there are no non-host members, otherwise `"%d of %d connected"`.
- `groupBounds` keyed by service name, preserving first-appearance order; unique uppercase protocols joined by ` + ` ordered TCP then UDP then BOTH; unique addresses preserving order; first non-empty hint.
- `shorten` uses rune count; if too long, returns the first `max-1` runes plus `…`.

- [ ] **Step 4: Implement `components.go`**

Use the palette values from Task 1.

- `appBar`: HBox with leading (if non-nil), 8px gap, `canvas.Text` title at 15 semibold using `shorten(title, 36)`, spacer, trailing; wrap in 12/24 padding; add `divider()` below.
- `page`: `container.NewBorder(bar, nil, nil, nil, container.NewVScroll(pad(24, 24, body)))` wrapped in `column{max: appWidth}`.
- `pageCentred`: same but with `container.NewVBox(layout.NewSpacer(), pad(24, 24, body), layout.NewSpacer())` inside the border area, no scroll.
- `sectionHeader`: 12px regular title in `colourSecondary`, count in `colourMuted` after a 6px gap, spacer, action.
- `statusDot`: 6×6 `canvas.Circle`.
- `statusLine`: HBox(dot, 8px gap, text).
- `keyValueRow`: HBox(fixed 104px label box, value, spacer, action). Add a `minWidth` layout type; label uses `metaText` (12 muted).
- `addressLine`: `addressParts`, host at `colourText`, port at `colourAccent`, both mono at `size`.
- `monoText`, `divider`, `appMark` (20×20 image from `appIcon`, fill contain).
- `pairRow`: custom layout that splits width minus gap equally and fills height.
- `copyButton`: returns a `button`; on tap writes to clipboard, sets its label to `Copied`, and after 1.5s restores the original label via `time.AfterFunc` + `fyne.Do`.
- `switchRow`: 52px min height, title 13 semibold, description 12 muted, 34×20 rounded track and 14px knob on the right; toggles on tap; `SetChecked` updates visual state without firing the callback.
- `listRow`: 56px min height when meta is present, 44 otherwise; optional 16px leading glyph, title 14 semibold truncated with `shorten(title, 34)`, meta 12 muted, optional green dot plus `live` at 12, optional chevron `›`, hover raised, disabled muted. `newNavRow` sets live + chevron; `newActionRow` sets leading `+`, chevron, and disabled.

- [ ] **Step 5: Update `widgets.go`**

- Replace the size constants with: title 24, app bar 15, name 14, body 13, meta 12, section 12, address 14, hero address 16.
- Point `titleText`, `nameText`, `bodyText`, `metaText`, `sectionLabel`, `addressText` at the named colours.
- Change `card` to surface/card `#1b1815`, hairline, radius 12, padding 16.
- Update `button` colours: primary fill/ink; secondary raised/hairline; quiet transparent/secondary; danger raised with danger text and `#4a302d` border; disabled surface/muted. Add `small` (28px, 12px text) and `large` (40px) sizes; default 36px. Add `setLabel(string)`.
- Add `smallButton(label, kind, onTap)` helper.
- Keep old widgets (`tile`, `tag`, `pill`, `rowButton`, `rightLayout`, footer helpers) compiling until the screens are rewritten; remove them in Task 8.
- Give `tapArea` a pointer cursor.

- [ ] **Step 6: Set the window size and test helpers**

In `internal/ui/app.go`: `app.win.Resize(fyne.NewSize(500, 640))`. In `home.go`, `group.go`, `friend.go`, replace `column{max: 460}` with `column{max: appWidth}`.

In `internal/ui/screens_test.go`, add:

```go
func newTestApp(t *testing.T) (*App, fyne.Window) {
	t.Helper()
	a := test.NewApp()
	a.Settings().SetTheme(NewTheme())
	app := &App{fyneApp: a}
	win := a.NewWindow("LAN Party")
	win.Resize(fyne.NewSize(500, 640))
	app.win = win
	return app, win
}

func screenText(w fyne.Window) string {
	var parts []string
	collectText(w.Canvas().Content(), &parts)
	return strings.Join(parts, "\n")
}

func collectText(obj fyne.CanvasObject, out *[]string) {
	switch o := obj.(type) {
	case *canvas.Text:
		*out = append(*out, o.Text)
	case *widget.Label:
		*out = append(*out, o.Text)
	case *fyne.Container:
		for _, child := range o.Objects {
			collectText(child, out)
		}
	case fyne.Widget:
		for _, child := range o.CreateRenderer().Objects() {
			collectText(child, out)
		}
	}
}
```

Use `newTestApp` and 500×640 in `screens_test.go`, `session_test.go`, and `join_test.go`.

- [ ] **Step 7: Verify and commit**

Run: `make fmt && make test`
Expected: PASS.

```bash
git add internal/ui/app.go internal/ui/components.go internal/ui/components_test.go internal/ui/format.go internal/ui/format_test.go internal/ui/widgets.go internal/ui/home.go internal/ui/group.go internal/ui/friend.go internal/ui/screens_test.go internal/ui/session_test.go internal/ui/join_test.go
git commit -m "feat: add ui components and formatting helpers"
```

---

### Task 4: Home screen

**Files:**
- Modify: `internal/ui/home.go`
- Modify: `internal/ui/screens_test.go`
- Modify: `internal/ui/session_test.go` (live-group home assertion)

**Interfaces:**
- Consumes `appBar`, `page`, `pageCentred`, `sectionHeader`, `listRow`, `pairRow`, `smallButton`, `card`, `statusDot`, `divider`.
- `showHome()` keeps its signature. `newGroup`, `joinGroup`, `openJoined`, `remember` keep theirs.
- `liveGroupID` is read to decide the `live` flag on a saved group row.

- [ ] **Step 1: Write failing home tests**

Add to `internal/ui/screens_test.go`:

```go
func TestHomeScreen(t *testing.T) {
	app, win := newTestApp(t)
	app.showHome()
	got := screenText(win)
	for _, want := range []string{
		"LAN Party", "One port, not the network.",
		"Host a group", "Join with an invite", "How it works",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("home missing %q", want)
		}
	}
}

func TestHomeShowsSavedGroups(t *testing.T) {
	app, win := newTestApp(t)
	g := group.Create("Friday Night", "Desynkro")
	app.groups = []group.Group{g}
	app.showHome()
	got := screenText(win)
	for _, want := range []string{"Your groups", "Friday Night", "host · 1 member"} {
		if !strings.Contains(got, want) {
			t.Errorf("home missing %q", want)
		}
	}
}
```

Add to `TestStartAndEndSession` after the `liveGroupID` assertion:

```go
app.showHome()
if !strings.Contains(screenText(app.win), "live") {
	t.Fatal("home does not show the live group")
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `make test`
Expected: FAIL on missing home strings.

- [ ] **Step 3: Implement the new home**

In `home.go`:

- `showHome` chooses `pageCentred` when both `a.groups` and `a.joined` are empty, otherwise `page`.
- App bar: `appBar(appMark(), "LAN Party", smallButton("Settings", buttonQuiet, a.showSettings))`.
- Headline: `One port, not the network.` at 24 semibold.
- Explanation as two 13px secondary lines:
  `Host a game server on your LAN.` and
  `Friends open it at 127.0.0.1; nothing else is reachable.`
- Actions: `pairRow` with a 40px primary `Host a group` and secondary `Join with an invite`, 12px gap.
- Saved groups: `Your groups` and `Groups you joined` section headers with counts; `newNavRow` rows. Host meta `host · N member(s)`; joined meta `joined · hosted by <host>`; live flag from `a.live != nil && a.liveGroupID == g.ID`.
- `pathPanel`: card with `How it works`, a horizontal row of two address nodes (`192.168.1.50:25565`, `127.0.0.1:25565`) joined by a 1px hairline and the word `encrypted`, labels `your server` / `their game`, and the line `Only the ports you publish cross the tunnel.`.
- No footer. Settings is only in the app bar.

- [ ] **Step 4: Update render captures**

At the end of `TestScreensRender`, before creating the group, keep a `home-empty.png` capture. After adding the group, call `app.showHome()` and capture `home-groups.png` before `showGroup`.

- [ ] **Step 5: Verify and commit**

Run: `make fmt && make test`
Expected: PASS.

```bash
git add internal/ui/home.go internal/ui/screens_test.go internal/ui/session_test.go
git commit -m "feat: rebuild home screen"
```

---

### Task 5: Group screen

**Files:**
- Modify: `internal/ui/group.go`
- Modify: `internal/ui/presence.go`
- Create: `internal/ui/group_test.go` (screen-text tests)
- Modify: `internal/ui/screens_test.go` (window size already done; capture states)
- Modify: `internal/ui/app.go` (only if a session-side helper is needed)

**Interfaces:**
- Consumes `appBar`, `page`, `sectionHeader`, `statusLine`, `statusDot`, `keyValueRow`, `addressLine`, `monoText`, `card`, `divider`, `copyButton`, `listRow`, `group.FormatPorts`, `connectedLabel`, `openInvites`, `loopbackPreview`.
- `showGroup(g group.Group)` keeps its signature.
- `memberRow` changes to return `dot *canvas.Circle` instead of `*canvas.Text`.
- `refreshPresence` updates `memberView.dot.FillColor`.
- `setEndpoint(g group.Group)` becomes reachable from `sessionPanel`.

- [ ] **Step 1: Write failing group-screen tests**

Create `internal/ui/group_test.go`:

```go
package ui

import (
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/group"
)

func TestGroupScreenStates(t *testing.T) {
	app, win := newTestApp(t)
	g := group.Create("Friends", "DeSynkro")
	peer, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	g.Members = append(g.Members, group.Member{Name: "Ash", PublicKey: peer.PublicKey().String(), BoxAddress: "100.64.0.2"})
	g.Services = []group.Service{{
		ID: "svc", Name: "Minecraft: Java Edition", Proto: "tcp",
		Ports: []int{25565}, Target: "192.168.1.50:25565",
	}}
	g.Invites = []group.Code{
		{PeerName: "Old", Expires: time.Now().Add(-time.Hour)},
		{PeerName: "Ash", Expires: time.Now().Add(time.Hour)},
	}
	app.groups = []group.Group{g}
	app.showGroup(g)

	got := screenText(win)
	for _, want := range []string{
		"Session off", "Set address", "Publish a service",
		"Friends open", "127.0.0.1:", "25565", "On your LAN",
		"People", "Invite for Ash", "Copy",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("group missing %q", want)
		}
	}
	for _, forbidden := range []string{"100.64.0.2", "can reach", "Invite for Old"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("group still shows %q", forbidden)
		}
	}

	g.Endpoint = "lanparty.example.com:51820"
	app.showGroup(g)
	got = screenText(win)
	if !strings.Contains(got, g.Endpoint) || !strings.Contains(got, "Change") {
		t.Fatalf("set address state not rendered")
	}
}

func TestGroupScreenLive(t *testing.T) {
	app, win := newTestApp(t)
	g := group.Create("Friday Night", "Desynkro")
	g.ListenPort = int(freeUDPPort(t))
	app.groups = []group.Group{g}
	app.showGroup(g)
	app.startSession(g)
	t.Cleanup(func() { app.endSession(g) })
	app.refreshPresence()

	got := screenText(win)
	for _, want := range []string{"Session live", "End session", "No one invited yet"} {
		if !strings.Contains(got, want) {
			t.Errorf("live group missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `make test`
Expected: FAIL on missing labels and the forbidden strings.

- [ ] **Step 3: Implement the group screen**

In `group.go`:

- `showGroup`: app bar `appBar(smallButton("←", buttonQuiet, a.showHome), g.Name, smallButton("Settings", buttonQuiet, a.showSettings))`; meta `Hosting as <host> · <memberCount>`; body session panel, 24px, published section, 24px, people section; `page`.
- `sessionPanel`: status LED + `Session live`/`Session off`; right action `End session` (danger, small) or `Start session` (primary). Add `connectedLabel` row when live. Address row via `keyValueRow`: empty value `Not set — friends can't join` in warning with a `Set address` quiet small button; set value mono with `Change`. UDP row `51820` mono. All inside a card.
- `publishedSection`: section header count; service cards; add row. Service card content: name 14 semibold truncated at 42; protocol uppercase 12 mono; `Friends open` + `addressLine(loopbackPreview(s), 14)`; `On your LAN` + target 12 mono secondary. Wrap in `tapArea` to open the detail dialog. Add row `newActionRow("Publish a service", "A preset or your own LAN address", a.live == nil, ...)`; when live use meta `End the session to publish a service` and disabled.
- `peopleSection`: section header count + `smallButton("Invite", buttonQuiet, ...)`; card with roster rows then open invites. Roster: host row dot muted, name, `you`, trailing `host`; peers dot from presence, name, trailing status (`live` green / `waiting` muted / `idle` muted). Register peer views in `a.memberViews`. No box IPs. Invites: `openInvites(g.Invites, time.Now())`; each row name 13 + `expires in <timeLeft>` 12 muted + small Copy button using `copyButton`; when none, `No open invites` meta.
- `serviceDialog`: show protocol/ports/target/address; Remove button; if live, disabled with meta `End the session to remove a service`. On Remove, hide the dialog first then call `confirmRemove`, which asks and calls `removeService`.
- `addService` copy: dialog title `Publish a service`; primary `Publish`; helper line `A preset fills the name and ports; a custom service takes any LAN address.` Keep validation.
- `setEndpoint`: wire from the address action; add helper line `Friends connect to this host and port. A hostname survives a changing IP.` Keep validation and `showGroup` refresh.
- `newInvite`: if endpoint empty, say `Set a public address in the session panel first, then invite them.`; keep the rest.
- `showCode`: use `a.copyButton("Copy", code, buttonPrimary, false)` and mono entry.
- Remove `statusPill`, `sessionBanner`, `inviteCard`, `playingLabel`; replace with the new helpers.
- Update `presence.go`: `memberView.dot` becomes `*canvas.Circle`; refresh sets `FillColor` and status text/colour. Keep the 2-second ticker.

- [ ] **Step 4: Capture group states**

In `TestScreensRender`, capture `group-idle.png` and `group-live.png` with the new window size. Keep the live session cleanup.

- [ ] **Step 5: Verify and commit**

Run: `make fmt && make test`
Expected: PASS.

```bash
git add internal/ui/group.go internal/ui/presence.go internal/ui/group_test.go internal/ui/screens_test.go internal/ui/app.go
git commit -m "feat: rebuild group screen"
```

---

### Task 6: Friend screen

**Files:**
- Modify: `internal/ui/friend.go`
- Modify: `internal/ui/join_test.go`
- Modify: `internal/ui/home.go` (remove the now-unused footer)
- Modify: `internal/ui/widgets.go` (only if a helper is dead)

**Interfaces:**
- Consumes `boundGroup`/`groupBounds`, `appBar`, `page`, `sectionHeader`, `statusLine`, `addressLine`, `monoText`, `card`, `divider`, `copyButton`.
- `showFriend(inv invite.Invite)` keeps its signature.
- `boundCard(a *App, b boundGroup) fyne.CanvasObject` replaces the old `boundCard(a,b session.Bound)`. `groupBounds` merges ports and protocols.

- [ ] **Step 1: Write failing friend tests**

In `internal/ui/join_test.go`, add a hint to the invite service in `liveInvite`:

```go
Services: []invite.Service{{
	Name: "Minecraft", Proto: "tcp", Ports: []int{25565},
	Hint: "Direct Connect in Minecraft",
}},
```

In `TestJoinAndForget`, after `app.openJoined(inv)` and the bound-count check, add:

```go
got := screenText(app.win)
for _, want := range []string{
	"Session open", "Minecraft", "Direct Connect in Minecraft", "Copy",
} {
	if !strings.Contains(got, want) {
		t.Errorf("friend screen missing %q", want)
	}
}
if !strings.Contains(got, "127.0.0.1:") {
	t.Fatal("friend screen missing the loopback address")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `make test`
Expected: FAIL on missing friend labels.

- [ ] **Step 3: Implement the friend screen**

In `friend.go`:

- App bar: `appBar(nil, inv.GroupName, smallButton("Disconnect", buttonQuiet, a.confirmDisconnect))`.
- Status line: muted dot + `Session open` 13 + `hosted by <host>` 12 muted.
- Section `Ready to play` with count of `groupBounds(a.friend.Bound())`.
- `boundCard`: name 14 semibold, protocol 12 mono muted; address 16 mono with copper port and a small Copy button; hint text 12 secondary (fallback `Use this address in the game's direct connect field.`); extra addresses `127.0.0.1:2457 · 127.0.0.1:2458` at 12 mono muted.
- Empty: card with `No service open` and `The host has not published anything to this group.`.
- `Forget this group` after a divider, quiet danger small button; confirmation `Forget this group? This disconnects and removes the saved invite.` then `a.forget(inv)`.
- `confirmDisconnect`: confirmation `Disconnect from this group? The local ports will close.` then close the friend session and `showHome`.
- Remove the footer from the friend screen; delete the `footer` helper from `home.go` once no screen uses it. Keep `versionText()` for Settings.

- [ ] **Step 4: Capture the friend screen**

`TestJoinAndForget` already calls `capture(t, app.win, "friend.png")`; keep it and confirm it uses the new 500×640 window.

- [ ] **Step 5: Verify and commit**

Run: `make fmt && make test`
Expected: PASS.

```bash
git add internal/ui/friend.go internal/ui/join_test.go internal/ui/home.go
git commit -m "feat: rebuild friend screen"
```

---

### Task 7: Settings

**Files:**
- Modify: `internal/ui/settings.go`
- Modify: `internal/ui/home.go` (move `versionText` here if still present)
- Create: `internal/ui/settings_test.go`

**Interfaces:**
- `func (a *App) settingsView() fyne.CanvasObject` returns the dialog body and is directly testable.
- `func (a *App) showSettings()` wraps `settingsView` in `dialog.NewCustom("Settings", "Close", ...)` and sizes it 420×340.
- `func (a *App) setAutostart(on bool) error` returns an error instead of showing dialogs itself; callers show the error and revert the switch.
- `switchRow` from Task 3 is used for both preferences.

- [ ] **Step 1: Write the failing settings test**

Create `internal/ui/settings_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestSettingsView(t *testing.T) {
	a := test.NewApp()
	app := &App{fyneApp: a}
	got := screenTextIn(app.settingsView())
	for _, want := range []string{
		"Startup", "Start minimised", "Start on login",
		"Closing the window keeps a live session running in the tray.",
		"LAN Party · dev build",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("settings missing %q", want)
		}
	}
}
```

Add this helper to `screens_test.go`:

```go
func screenTextIn(obj fyne.CanvasObject) string {
	var parts []string
	collectText(obj, &parts)
	return strings.Join(parts, "\n")
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `make test`
Expected: compile FAIL because `settingsView` does not exist.

- [ ] **Step 3: Implement the settings sheet**

In `settings.go`:

- `settingsView` returns a VBox: section header `Startup`; switch row `Start minimised` / `Open in the tray without a window.`; switch row `Start on login` / `Open LAN Party when you sign in.`; divider; note text; divider; `LAN Party · <versionText>` at 12 muted.
- Start minimised callback writes `prefStartMinimised`.
- Start on login callback calls `setAutostart(on)`; on error show a dialog and `boot.SetChecked(!on)`; otherwise write `prefStartOnBoot`.
- `showSettings` opens the themed dialog and resizes 420×340.
- Move `versionText()` from `home.go` to `settings.go` if it is still there.

- [ ] **Step 4: Verify and commit**

Run: `make fmt && make test`
Expected: PASS.

```bash
git add internal/ui/settings.go internal/ui/settings_test.go internal/ui/home.go internal/ui/screens_test.go
git commit -m "feat: rebuild settings sheet"
```

---

### Task 8: Remove dead widgets, final visual pass, build

**Files:**
- Modify: `internal/ui/widgets.go`
- Modify: `internal/ui/screens_test.go`
- Maybe modify: `docs/design.md` (only if a sentence is now stale)
- Generated previews under `/tmp/lanparty-preview`

**Interfaces:**
- No new interfaces. Remove only helpers with no references: `tile`, `tag`, `pill`, `rowButton`, `rightLayout`, `disabledButton`, `newCompactButton`, and any footer helpers left after Tasks 4–7.

- [ ] **Step 1: Find dead helpers**

Run: `grep -rn "tile(\|tag(\|pill(\|newRowButton\|rowButton\|rightLayout\|disabledButton\|newCompactButton\|footer(" internal/ui`
Expected: only declarations remain (or nothing). Remove those declarations from `widgets.go`. Do not remove `newTestApp`, `screenText`, `screenTextIn`, or `column`.

- [ ] **Step 2: Expand screen captures**

In `TestScreensRender`, capture:
- `home-empty.png` before adding a group.
- `home-groups.png` after adding the group and calling `showHome`.
- `group-idle.png` after the first `showGroup`.
- `group-live.png` after `startSession`.
`TestJoinAndForget` captures `friend.png`; add a small `settings.png` capture by creating a window with `app.settingsView()` inside a page or by rendering the settings body directly to a window.

- [ ] **Step 3: Render and inspect**

Run:
```bash
rm -rf /tmp/lanparty-preview && mkdir -p /tmp/lanparty-preview
env LANPARTY_PREVIEW=/tmp/lanparty-preview make test
```
Expected: tests PASS and six PNGs appear. Inspect each with the image reader. Check against the unslop checklist:

- 4px spacing rhythm, equal row heights, aligned labels.
- One neutral ramp, three text roles, copper only where it means a port/link.
- No emoji, gradients, glows, pills, uppercase tracked labels, or dead bands.
- The empty home and the group screen each have one clear bright element.
- Long names do not overflow; addresses and ports are mono and aligned.
- No `can reach`, no `100.64.0.x` roster tags, friend says `Session open`.

Fix any visual failures and re-render; keep the number of render passes small (three max).

- [ ] **Step 4: Run the full verification**

Run: `make fmt && make vet && make test && make build`
Expected: all pass; binary at `bin/lan-party`.

- [ ] **Step 5: Commit**

```bash
git add internal/ui internal/session
git commit -m "style: finish ui redesign"
```

---

## Self-review notes

- Spec coverage: palette/type (Task 1), truthful plumbing (Task 2), components and 500×640 shell (Task 3), home (Task 4), group including address action/people/invites (Task 5), friend including hints and confirmations (Task 6), settings (Task 7), cleanup/visual verification (Task 8).
- Type consistency: `boundGroup` is defined in Task 3 and consumed in Task 6; `liveGroupID` Task 2 and consumed in Task 4; `session.Bound.Hint` Task 2 and consumed by `groupBounds` Task 3 and friend Task 6.
- Review Focus tests: long/multi-port (Task 3 `TestGroupBounds` + Task 8 screenshot), missing endpoint (Task 5), invite states (Task 3 `TestOpenInvites` + Task 5), live transitions (Task 2/5), truthfulness/privacy (Task 5 forbidden strings + Task 6 labels).
