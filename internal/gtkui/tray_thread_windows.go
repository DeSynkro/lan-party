//go:build windows

package gtkui

import "runtime"

// lockTrayThread pins the tray message loop to one OS thread. Win32 windows
// are thread-affine, so click messages stop if Go moves the goroutine.
func lockTrayThread() func() {
	runtime.LockOSThread()
	return runtime.UnlockOSThread
}
