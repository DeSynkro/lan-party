//go:build linux

package gtkui

import "path/filepath"

// trayIconPath is the higher-resolution PNG the Linux panel accepts.
func trayIconPath(root string) string {
	return filepath.Join(root, "brand", "png", "tray-icon-color-64.png")
}
