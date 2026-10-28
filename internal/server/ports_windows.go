package server

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isPortBindError(err error) bool {
	return errors.Is(err, windows.WSAEADDRINUSE) || errors.Is(err, windows.WSAEACCES)
}
