// Package autostart manages the XDG desktop entry that opens the app at login.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const fileName = "lan-party.desktop"

// Set writes or removes the desktop entry under configHome/autostart.
func Set(configHome string, enabled bool) error {
	path := filepath.Join(configHome, "autostart", fileName)
	if !enabled {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	// An AppImage runs from a temporary mount. The APPIMAGE variable points at
	// the file the user actually downloaded.
	if appImage := os.Getenv("APPIMAGE"); appImage != "" {
		if absolute, err := filepath.Abs(appImage); err == nil {
			executable = absolute
		} else {
			executable = appImage
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}

	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=LAN Party
Comment=A LAN cable across the internet
Exec="%s"
Terminal=false
X-GNOME-Autostart-enabled=true
`, executable)

	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return fmt.Errorf("write autostart entry: %w", err)
	}
	return nil
}
