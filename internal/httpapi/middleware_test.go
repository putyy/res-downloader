package httpapi

import "testing"

func TestTrustedAPIOriginWails3(t *testing.T) {
	for _, test := range []struct {
		origin string
		want   bool
	}{
		{"wails://localhost", true},
		{"http://wails.localhost", true},
		{"wails://localhost.attacker.example", false},
		{"wails://attacker.example", false},
		{"wails://user@localhost", false},
		{"null", false},
	} {
		t.Run(test.origin, func(t *testing.T) {
			if got := trustedAPIOrigin(test.origin); got != test.want {
				t.Errorf("trustedAPIOrigin(%q) = %v, want %v", test.origin, got, test.want)
			}
		})
	}
}
