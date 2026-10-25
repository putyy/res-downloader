//go:build linux

package system

import (
	"errors"
	"strings"
)

type linuxProxy [6]string
type linuxProxyStore struct{ run proxyCommand }

var gnomeProxyKeys = [6][2]string{
	{"org.gnome.system.proxy.http", "use-authentication"},
	{"org.gnome.system.proxy.http", "host"},
	{"org.gnome.system.proxy.http", "port"},
	{"org.gnome.system.proxy.https", "host"},
	{"org.gnome.system.proxy.https", "port"},
	{"org.gnome.system.proxy", "mode"},
}

func (s linuxProxyStore) read(string) (any, error) {
	var value linuxProxy
	for i, key := range gnomeProxyKeys {
		output, err := runProxyCommand(s.run, "gsettings", "get", key[0], key[1])
		if err != nil {
			return nil, err
		}
		value[i] = strings.TrimSpace(string(output))
		if value[i] == "" {
			return nil, errors.New("gsettings returned an empty proxy setting")
		}
	}
	return value, nil
}

func (s linuxProxyStore) write(_ string, current, target any) (any, error) {
	value, desired := current.(linuxProxy), target.(linuxProxy)
	order := []int{0, 1, 2, 3, 4, 5}
	// Never enable existing credentials while HTTP still points to capture.
	if desired[0] == "true" {
		order = []int{5, 4, 3, 2, 1, 0}
	}
	for _, i := range order {
		if value[i] == desired[i] {
			continue
		}
		key := gnomeProxyKeys[i]
		if _, err := runProxyCommand(s.run, "gsettings", "set", key[0], key[1], desired[i]); err != nil {
			return value, err
		}
		value[i] = desired[i]
	}
	return value, nil
}
