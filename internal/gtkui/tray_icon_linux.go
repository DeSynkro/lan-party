//go:build linux

package gtkui

import "path/filepath"

// trayIconPath is the PNG the Linux panel accepts.
func trayIconPath(root string) string {
	return filepath.Join(root, "brand", "png", "tray-icon-color-24.png")
}
