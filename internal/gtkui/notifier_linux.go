//go:build linux

package gtkui

import "github.com/godbus/dbus/v5"

// statusNotifierAvailable checks for the host that owns tray icons on Linux.
// Without it, hiding the window on close would strand the app.
func statusNotifierAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var owner bool
	err = conn.BusObject().Call(
		"org.freedesktop.DBus.NameHasOwner",
		0,
		"org.kde.StatusNotifierWatcher",
	).Store(&owner)
	return err == nil && owner
}
