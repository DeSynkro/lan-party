package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/desynkro/lan-party/internal/group"
)

// Both screens have to build and draw. This catches layout panics and nil
// widget mistakes without needing a display.
//
// Set LANPARTY_PREVIEW to a directory to also write what they look like.
func TestScreensRender(t *testing.T) {
	a := test.NewApp()
	a.Settings().SetTheme(NewTheme())

	app := &App{fyneApp: a}
	app.win = a.NewWindow("LAN Party")
	app.win.Resize(fyne.NewSize(420, 620))

	app.showHome()
	capture(t, app.win, "home.png")

	g := group.Create("Friday Night", "Desynkro")
	peerKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	g.Members = append(g.Members, group.Member{Name: "Ash", PublicKey: peerKey.PublicKey().String(), BoxAddress: "100.64.0.2"})
	g.Invites = []group.Code{{Value: "lp1_test", PeerName: "Jonah", Expires: time.Now().Add(3 * time.Hour)}}
	g.Services = []group.Service{{
		ID:     "svc1",
		Name:   "Minecraft: Java Edition",
		Proto:  "tcp",
		Ports:  []int{25565},
		Target: "192.168.1.50:25565",
	}}
	app.showGroup(g)
	capture(t, app.win, "group.png")

	g.ListenPort = int(freeUDPPort(t))
	app.startSession(g)
	capture(t, app.win, "group-live.png")
	app.endSession(g)
}

func capture(t *testing.T, w fyne.Window, name string) {
	t.Helper()

	img := w.Canvas().Capture()
	if img == nil {
		t.Fatal("screen produced no image")
	}

	dir := os.Getenv("LANPARTY_PREVIEW")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
