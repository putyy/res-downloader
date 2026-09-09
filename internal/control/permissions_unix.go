//go:build !windows

package control

import "os"

func protectDirectory(path string) error { return os.Chmod(path, 0700) }
