//go:build !windows

package secure

import (
	"fmt"
	"os"
)

// checkKeyFile rejects key files that another user can read.
func checkKeyFile(path string, info os.FileInfo) error {
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("key file %s is readable by other users", path)
	}
	return nil
}
