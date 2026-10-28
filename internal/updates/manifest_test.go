package updates

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStableUpdateComparison(t *testing.T) {
	for _, test := range []struct {
		next, current string
		want          bool
	}{
		{"4.0.0", "3.9.9", true}, {"4.0.0", "4.0.0", false}, {"4.0.0", "4.0.0-beta.2", true},
		{"4.0.0", "4.1.0-beta.1", false}, {"4.0.1-rc.1", "4.0.0", false}, {"4.0.0", "invalid", false},
		{"4.10.0", "4.9.0", true}, {"4.0.0", "v4.0.0", false},
	} {
		if got := Newer(test.next, test.current); got != test.want {
			t.Errorf("Newer(%q,%q)=%v", test.next, test.current, got)
		}
	}
}
func TestDecodeRejectsUntrustedAssets(t *testing.T) {
	good := Manifest{SchemaVersion: 1, Version: "4.0.0", Tag: "v4.0.0", Assets: []Asset{{Name: "res-downloader_4.0.0_win_amd64.exe", URL: "https://github.com/putyy/res-downloader/releases/download/v4.0.0/res-downloader_4.0.0_win_amd64.exe", Size: 42}}}
	raw, _ := json.Marshal(good)
	for _, test := range []struct {
		name  string
		raw   []byte
		valid bool
	}{
		{"official", raw, true},
		{"missing schema", []byte(`{"version":"4.0.0"}`), false},
		{"unsupported schema", []byte(strings.Replace(string(raw), `"schemaVersion":1`, `"schemaVersion":2`, 1)), false},
		{"missing tag", []byte(`{"schemaVersion":1,"version":"4.0.0","assets":[{}]}`), false},
		{"missing assets", []byte(`{"schemaVersion":1,"version":"4.0.0","tag":"v4.0.0"}`), false},
		{"empty assets", []byte(`{"schemaVersion":1,"version":"4.0.0","tag":"v4.0.0","assets":[]}`), false},
		{"foreign repo", []byte(strings.ReplaceAll(string(raw), "/putyy/", "/someone/")), false},
		{"http", []byte(strings.ReplaceAll(string(raw), "https://", "http://")), false},
		{"prerelease", []byte(`{"version":"4.0.0-beta.1"}`), false},
		{"bad digest", []byte(strings.Replace(string(raw), `"size":42`, `"size":42,"sha256":"invalid"`, 1)), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode(test.raw)
			if (err == nil) != test.valid {
				t.Fatalf("Decode error = %v", err)
			}
			if !test.valid && !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("invalid manifest was not classified: %v", err)
			}
		})
	}
}
