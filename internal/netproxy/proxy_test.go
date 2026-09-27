package netproxy

import (
	"net/http"
	"testing"
)

func TestUpdateRequestsRespectDownloadProxy(t *testing.T) {
	// Ambient proxy variables must not override either an explicit upstream or
	// the user's choice to turn the download proxy off.
	t.Setenv("HTTP_PROXY", "http://environment.invalid:9999")
	t.Setenv("HTTPS_PROXY", "http://environment.invalid:9999")
	t.Setenv("NO_PROXY", "*")
	request, err := http.NewRequest(http.MethodGet, "https://res.putyy.com/version.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, upstream  string
		enabled, direct bool
		want            string
		invalid         bool
	}{
		{name: "disabled", upstream: "http://127.0.0.1:7890"},
		{name: "configured", upstream: "http://127.0.0.1:7890", enabled: true, want: "http://127.0.0.1:7890"},
		{name: "direct retry", upstream: "http://127.0.0.1:7890", enabled: true, direct: true},
		{name: "missing upstream", enabled: true, invalid: true},
		{name: "invalid upstream", upstream: "://invalid", enabled: true, invalid: true},
		{name: "capture loopback", upstream: "http://127.0.0.1:8899", enabled: true, invalid: true},
		{name: "capture localhost", upstream: "http://localhost:8899", enabled: true, invalid: true},
		{name: "capture ipv6", upstream: "http://[::1]:8899", enabled: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxy, err := Resolver("8899", tc.upstream, tc.enabled, tc.direct)(request)
			if (err != nil) != tc.invalid {
				t.Fatalf("proxy error = %v", err)
			}
			got := ""
			if proxy != nil {
				got = proxy.String()
			}
			if got != tc.want {
				t.Fatalf("proxy = %q, want %q", got, tc.want)
			}
		})
	}
}
