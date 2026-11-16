package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

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

	app.showGroup(group.Create("Friday Night", "Desynkro"))
	capture(t, app.win, "group.png")
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
