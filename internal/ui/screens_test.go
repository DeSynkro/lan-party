package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/zalando/go-keyring"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
)

type testKeyring map[string]string

func (m testKeyring) slot(service, user string) string { return service + "\x00" + user }

func (m testKeyring) Get(service, user string) (string, error) {
	value, ok := m[m.slot(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (m testKeyring) Set(service, user, secret string) error {
	m[m.slot(service, user)] = secret
	return nil
}

func (m testKeyring) Delete(service, user string) error {
	delete(m, m.slot(service, user))
	return nil
}

func newTestUI(t *testing.T, groups []group.Group) *UI {
	t.Helper()
	paths := config.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "config"),
		DataDir:   filepath.Join(t.TempDir(), "data"),
	}
	if groups != nil {
		if err := group.OpenStore(filepath.Join(paths.DataDir, "groups.json")).Save(groups); err != nil {
			t.Fatal(err)
		}
	}
	state, err := apppkg.NewWithKeyring(paths, testKeyring{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })

	a := test.NewApp()
	a.Settings().SetTheme(NewTheme())
	win := a.NewWindow("LAN Party")
	win.Resize(fyne.NewSize(windowWidth, windowHeight))

	return &UI{fyneApp: a, win: win, app: state}
}

func sampleHostGroup(t *testing.T) group.Group {
	t.Helper()
	g := group.CreateHostGroup("Friday Night", "Desynkro")
	if err := g.AddService(group.Service{
		ID:     "mc",
		Name:   "Minecraft: Java Edition",
		Proto:  "tcp",
		Target: "192.168.1.50:25565",
		Ports:  []int{25565},
		Hint:   "Direct Connect in Minecraft",
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddMember(group.Member{
		Name:        "Ash",
		DeviceName:  "ash-desktop",
		PublicKey:   "public-key",
		Fingerprint: "A1B2-C3D4-E5F6-7890",
		State:       group.MemberApproved,
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddInvite(group.Invite{
		Name:      "Jonah",
		TokenHash: "hash",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(6 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	return g
}

func sampleFriendGroup(t *testing.T) group.Group {
	t.Helper()
	g := group.CreateFriendGroup(
		"Friday Night",
		"Desynkro",
		"home.example.net:51820",
		"pin",
		"member-1",
		"Ash",
		[]group.Service{{
			ID:    "mc",
			Name:  "Minecraft: Java Edition",
			Proto: "tcp",
			Ports: []int{25565},
			Hint:  "Direct Connect in Minecraft",
		}},
	)
	return g
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

func TestHomeScreen(t *testing.T) {
	u := newTestUI(t, nil)
	u.showHome()
	got := screenText(u.win)
	for _, want := range []string{
		"LAN Party",
		"One port, not the network.",
		"Host a group",
		"Join with an invite",
		"How it works",
		"First time? Read the setup guide",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("home missing %q", want)
		}
	}
}

func TestHostScreen(t *testing.T) {
	g := sampleHostGroup(t)
	u := newTestUI(t, []group.Group{g})
	s := u.app.Settings()
	s.PublicAddress = "home.example.net"
	if err := u.app.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	u.showGroup(g.ID)
	got := screenText(u.win)
	for _, want := range []string{
		"Friday Night",
		"Session off",
		"Hosting as Desynkro",
		"Published · 1",
		"Minecraft: Java Edition",
		"127.0.0.1:25565",
		"192.168.1.50:25565",
		"People · 1 member",
		"Ash",
		"Invites",
		"Jonah",
		"Delete group",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("host screen missing %q", want)
		}
	}
}

func TestFriendScreen(t *testing.T) {
	g := sampleFriendGroup(t)
	u := newTestUI(t, []group.Group{g})
	u.showGroup(g.ID)
	got := screenText(u.win)
	for _, want := range []string{
		"Friday Night",
		"Host offline",
		"Minecraft: Java Edition",
		"Port 25565",
		"Leave group",
		"Hosted by Desynkro",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("friend screen missing %q", want)
		}
	}
}

func TestSettingsAndGuide(t *testing.T) {
	u := newTestUI(t, nil)
	u.showSettings(u.showHome)
	got := screenText(u.win)
	for _, want := range []string{"Startup", "Network", "Security", "About", "Approve new devices"} {
		if !strings.Contains(got, want) {
			t.Errorf("settings missing %q", want)
		}
	}

	u.showGuide(u.showHome)
	got = screenText(u.win)
	for _, want := range []string{"Hosting", "Joining", "Problems", "Security", "Forward UDP 51820"} {
		if !strings.Contains(got, want) {
			t.Errorf("guide missing %q", want)
		}
	}
}

func TestPreviewScreens(t *testing.T) {
	dir := os.Getenv("LANPARTY_PREVIEW")
	if dir == "" {
		t.Skip("set LANPARTY_PREVIEW to write preview images")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	host := sampleHostGroup(t)
	friend := sampleFriendGroup(t)
	u := newTestUI(t, []group.Group{host, friend})
	s := u.app.Settings()
	s.PublicAddress = "home.example.net"
	if err := u.app.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	u.showHome()
	capture(t, u.win, filepath.Join(dir, "home.png"))
	u.showGroup(host.ID)
	capture(t, u.win, filepath.Join(dir, "host.png"))
	u.showGroup(friend.ID)
	capture(t, u.win, filepath.Join(dir, "friend.png"))
	u.showSettings(u.showHome)
	capture(t, u.win, filepath.Join(dir, "settings.png"))
	u.showGuide(u.showHome)
	capture(t, u.win, filepath.Join(dir, "guide.png"))
}

func capture(t *testing.T, w fyne.Window, path string) {
	t.Helper()
	img := w.Canvas().Capture()
	if img == nil {
		t.Fatal("screen produced no image")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func findRow(obj fyne.CanvasObject) *rowButton {
	switch o := obj.(type) {
	case *rowButton:
		return o
	case *fyne.Container:
		for _, child := range o.Objects {
			if row := findRow(child); row != nil {
				return row
			}
		}
	case fyne.Widget:
		for _, child := range o.CreateRenderer().Objects() {
			if row := findRow(child); row != nil {
				return row
			}
		}
	}
	return nil
}

func TestHomeRowOpensGroup(t *testing.T) {
	host := sampleHostGroup(t)
	u := newTestUI(t, []group.Group{host})
	u.showHome()

	row := findRow(u.win.Canvas().Content())
	if row == nil {
		t.Fatal("the group row is missing")
	}
	test.Tap(row)
	got := screenText(u.win)
	if !strings.Contains(got, "Session off") {
		t.Fatalf("tapping the group row did not open it:\n%s", got)
	}
}
