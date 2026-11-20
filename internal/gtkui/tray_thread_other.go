//go:build !windows

package gtkui

func lockTrayThread() func() {
	return func() {}
}
