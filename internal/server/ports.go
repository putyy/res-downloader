package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

// IsPortUnavailable only classifies failed TCP listeners, never HTTP errors or
// connection failures. Windows also reports exclusive port use as access denied.
func IsPortUnavailable(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && op.Op == "listen" &&
		(op.Net == "tcp" || op.Net == "tcp4" || op.Net == "tcp6") && isPortBindError(op.Err)
}

// ReserveAlternativePort keeps the selected port bound until the caller closes
// the listener. Port zero asks the OS for an available ephemeral port.
func ReserveAlternativePort(host, failedPort string) (net.Listener, string, error) {
	return reserveAlternativePort(host, failedPort, net.Listen)
}

func reserveAlternativePort(host, failedPort string, listen func(string, string) (net.Listener, error)) (net.Listener, string, error) {
	var failures []error
	for _, port := range []string{"18899", "7788", "0", "0", "0"} {
		if port == failedPort {
			continue
		}
		listener, err := listen("tcp", net.JoinHostPort(host, port))
		if err != nil {
			failures = append(failures, err)
			if !IsPortUnavailable(err) {
				break
			}
			continue
		}
		address, ok := listener.Addr().(*net.TCPAddr)
		if ok && address.Port > 1024 && address.Port < 65535 && strconv.Itoa(address.Port) != failedPort {
			return listener, strconv.Itoa(address.Port), nil
		}
		_ = listener.Close()
		failures = append(failures, fmt.Errorf("allocated port is outside the allowed range or matches the failed port"))
	}
	return nil, "", fmt.Errorf("no alternative listen port is available: %w", errors.Join(failures...))
}
