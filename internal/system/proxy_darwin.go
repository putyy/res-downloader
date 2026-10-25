//go:build darwin

package system

import (
	"errors"
	"strings"
)

type networkProxy struct {
	enabled, authenticated bool
	server, port           string
}
type darwinProxyStore struct {
	run     proxyCommand
	capture bool
}

func (s darwinProxyStore) read(key string) (any, error) {
	kind, service, _ := strings.Cut(key, "\x00")
	output, err := runProxyCommand(s.run, "networksetup", "-get"+kind+"proxy", service)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(output), "\n") {
		name, value, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(name)] = strings.TrimSpace(value)
		}
	}
	for _, name := range []string{"Enabled", "Server", "Port", "Authenticated Proxy Enabled"} {
		if _, ok := fields[name]; !ok {
			return nil, errors.New("networksetup returned incomplete proxy settings")
		}
	}
	if fields["Enabled"] != "Yes" && fields["Enabled"] != "No" || fields["Authenticated Proxy Enabled"] != "0" && fields["Authenticated Proxy Enabled"] != "1" {
		return nil, errors.New("networksetup returned invalid proxy flags")
	}
	value := networkProxy{fields["Enabled"] == "Yes", fields["Authenticated Proxy Enabled"] == "1", fields["Server"], fields["Port"]}
	if s.capture && value.authenticated {
		return nil, errors.New("authenticated system proxy cannot be restored automatically; connect to capture manually")
	}
	return value, nil
}

func (s darwinProxyStore) write(key string, current, target any) (any, error) {
	value, desired := current.(networkProxy), target.(networkProxy)
	kind, service, _ := strings.Cut(key, "\x00")
	if value.server != desired.server || value.port != desired.port {
		port := desired.port
		if desired.server == "" && port == "0" {
			port = ""
		}
		if _, err := runProxyCommand(s.run, "networksetup", "-set"+kind+"proxy", service, desired.server, port, "off"); err != nil {
			return value, err
		}
		value = networkProxy{enabled: true, server: desired.server, port: desired.port}
	}
	if value.enabled != desired.enabled {
		state := "off"
		if desired.enabled {
			state = "on"
		}
		if _, err := runProxyCommand(s.run, "networksetup", "-set"+kind+"proxystate", service, state); err != nil {
			return value, err
		}
		value.enabled = desired.enabled
	}
	return value, nil
}
