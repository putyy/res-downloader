//go:build !windows

package system

// PrepareCommandConsole is a no-op on platforms without Windows GUI consoles.
func PrepareCommandConsole() {}
