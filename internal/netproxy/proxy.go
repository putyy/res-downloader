// Package netproxy applies the configured proxy to application update requests.
package netproxy

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func Resolver(ownPort, upstream string, enabled, direct bool) func(*http.Request) (*url.URL, error) {
	return func(_ *http.Request) (*url.URL, error) {
		if direct || !enabled {
			return nil, nil
		}
		proxy, err := url.Parse(strings.TrimSpace(upstream))
		if err != nil || proxy.Hostname() == "" || (proxy.Scheme != "http" && proxy.Scheme != "https") ||
			(proxy.Path != "" && proxy.Path != "/") || proxy.RawQuery != "" || proxy.Fragment != "" {
			return nil, errors.New("download proxy requires a valid HTTP or HTTPS upstream address")
		}
		if ownProxy(proxy, ownPort) {
			return nil, errors.New("upstream proxy points to the application's capture port")
		}
		return proxy, nil
	}
}
func ownProxy(proxy *url.URL, port string) bool {
	if proxy.Port() != port {
		return false
	}
	host := strings.ToLower(proxy.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() {
		return true
	}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		local, _, _ := net.ParseCIDR(address.String())
		if local.Equal(ip) {
			return true
		}
	}
	return false
}
