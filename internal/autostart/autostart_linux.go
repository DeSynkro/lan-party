package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// file is the desktop entry used by XDG aware desktops.
const file = "lan-party.desktop"

// Enable asks the desktop to start the app at login.
func Enable(exec string) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	autostart := filepath.Join(dir, "autostart")
	if err := os.MkdirAll(autostart, 0o755); err != nil {
		return err
	}

	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=LAN Party
Comment=One port on one machine, not your whole network
Exec="%s"
X-GNOME-Autostart-enabled=true
`, exec)

	return os.WriteFile(filepath.Join(autostart, file), []byte(entry), 0o644)
}

// Disable removes the entry. A missing entry is not an error.
func Disable() error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(dir, "autostart", file))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
