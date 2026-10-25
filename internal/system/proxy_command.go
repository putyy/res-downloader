//go:build darwin || linux

package system

import (
	"fmt"
	"strings"
)

// Runners are operation-scoped: snapshots must not retain sudo passwords.
type proxyCommand func([]string) ([]byte, error)

func runProxyCommand(run proxyCommand, args ...string) ([]byte, error) {
	output, err := run(args)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", args[0], args[1], err, strings.TrimSpace(string(output)))
	}
	// networksetup can report invalid arguments in stdout without a failing exit.
	if args[0] == "networksetup" && strings.Contains(strings.ToLower(string(output)), "error:") {
		return nil, fmt.Errorf("networksetup %s: %s", args[1], strings.TrimSpace(string(output)))
	}
	return output, nil
}
