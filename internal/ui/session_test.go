package ui

import (
	"net"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/desynkro/lan-party/internal/group"
)

// The screen has to be able to start and stop a real session, not just draw a
// button that pretends to.
func TestStartAndEndSession(t *testing.T) {
	a := test.NewApp()
	app := &App{fyneApp: a}
	app.win = a.NewWindow("LAN Party")
	app.win.Resize(fyne.NewSize(460, 620))

	g := group.Create("Friday Night", "Desynkro")
	g.ListenPort = int(freeUDPPort(t))
	app.groups = []group.Group{g}
	app.showGroup(g)

	app.startSession(g)
	if app.live == nil {
		t.Fatal("session did not start")
	}

	app.endSession(g)
	if app.live != nil {
		t.Fatal("session did not stop")
	}
}

func freeUDPPort(t *testing.T) uint16 {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return uint16(conn.LocalAddr().(*net.UDPAddr).Port)
}
