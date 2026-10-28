//go:build !windows

package server

import (
	"errors"
	"syscall"
)

func isPortBindError(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
