//go:build windows

package gtkui

import "path/filepath"

// trayIconPath is the ICO the Windows notification area expects.
func trayIconPath(root string) string {
	return filepath.Join(root, "brand", "ico", "tray-icon-color.ico")
}
