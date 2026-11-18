//go:build windows

package gtkui

// statusNotifierAvailable reports whether a tray host is available. Windows
// always provides the notification area, so the tray can be used.
func statusNotifierAvailable() bool {
	return true
}
