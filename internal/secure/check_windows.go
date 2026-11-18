//go:build windows

package secure

import "os"

// checkKeyFile trusts the per-user profile ACLs on Windows. Unix mode bits do
// not describe those ACLs, so checking them would reject every key file.
func checkKeyFile(path string, info os.FileInfo) error {
	return nil
}
