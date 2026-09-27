package metadata

import (
	"context"
	"errors"
	"net/url"
	"sync/atomic"
	"testing"

	"res-downloader/internal/system"
)

func TestSingleSourceFetch(t *testing.T) {
	domestic, overseas := Domestic, Overseas
	preference.Lock()
	previous := preference.source
	preference.source = ""
	preference.Unlock()
	Domestic, Overseas = "https://example.com/update-test/", "https://example.com/update-test"
	t.Cleanup(func() {
		Domestic, Overseas = domestic, overseas
		preference.Lock()
		preference.source = previous
		preference.Unlock()
	})
	for _, outcome := range []string{"success", "download failure", "invalid metadata"} {
		t.Run(outcome, func(t *testing.T) {
			var reads atomic.Int32
			failure := errors.New(outcome)
			read := func(ctx context.Context, address string, maximum int64) ([]byte, error) {
				reads.Add(1)
				if address != "https://example.com/update-test/version.json" {
					t.Errorf("unexpected source: %s", address)
				}
				if outcome == "download failure" {
					return nil, failure
				}
				return []byte(`{"version":"4.0.1"}`), nil
			}
			validate := func(raw []byte) error {
				if outcome == "invalid metadata" {
					return failure
				}
				return nil
			}
			raw, source, err := fetch(context.Background(), "/version.json", validate, read)
			if reads.Load() != 1 {
				t.Fatalf("fetched the same source %d times", reads.Load())
			}
			if outcome == "success" {
				if err != nil || source != "https://example.com/update-test" || len(raw) == 0 {
					t.Fatalf("source=%q error=%v", source, err)
				}
			} else if !errors.Is(err, failure) {
				t.Fatalf("did not retain fetch/validation failure: %v", err)
			}
		})
	}
}

func TestInstallationURL(t *testing.T) {
	domestic, overseas := Domestic, Overseas
	preference.Lock()
	previous := preference.source
	preference.source = ""
	preference.Unlock()
	t.Cleanup(func() {
		Domestic, Overseas = domestic, overseas
		preference.Lock()
		preference.source = previous
		preference.Unlock()
	})
	fallback := "https://putyy.github.io/res-downloader"
	if system.DefaultLocale() == "zh" {
		fallback = "https://res.putyy.com"
	}
	for _, test := range []struct{ source, website string }{
		{"https://res.putyy.com", "https://res.putyy.com"},
		{"https://res.putyy.com/update-test/", "https://res.putyy.com"},
		{"https://RES.PUTYY.COM/update-test", "https://res.putyy.com"},
		{"https://putyy.github.io/res-downloader", "https://putyy.github.io/res-downloader"},
		{"https://putyy.github.io/res-downloader/update-test", "https://putyy.github.io/res-downloader"},
		{"https://example.com/update-test", fallback},
		{"https://res.putyy.com.example.com/update-test", fallback},
	} {
		t.Run(test.source, func(t *testing.T) {
			Domestic, Overseas = test.source, test.source
			for _, source := range []string{test.source, ""} {
				for _, locale := range []string{"zh", "en"} {
					path := "/guide/installation.html"
					if locale == "en" {
						path = "/en" + path
					}
					if got := InstallationURL(source, locale); got != test.website+path {
						t.Errorf("InstallationURL(%q, %q) = %q, want %q", source, locale, got, test.website+path)
					}
					prefix, anchor := test.website+"/", "文件命名模板"
					if locale == "en" {
						prefix, anchor = prefix+"en/", "filename-template"
					}
					for page, want := range map[string]string{
						"home":              prefix,
						"filename-template": prefix + "guide/settings.html#" + anchor,
						"unknown":           "",
					} {
						if got := DocumentationURL(source, locale, page); got != want {
							t.Errorf("DocumentationURL(%q, %q, %q) = %q, want %q", source, locale, page, got, want)
						}
					}
				}
			}
		})
	}
}

func TestWithinSource(t *testing.T) {
	for _, test := range []struct {
		target string
		want   bool
	}{
		{"https://example.com/update-test/package.exe", true},
		{"https://example.com/update-test/assets/package.exe", true},
		{"https://EXAMPLE.com:443/update-test/package.exe", true},
		{"https://example.com/update-test/package.exe?download=1", true},
		{"https://example.com/update-testing/package.exe", false},
		{"https://example.com/elsewhere/package.exe", false},
		{"https://example.com/update-test/../package.exe", false},
		{"https://example.com/update-test/%2e%2e/package.exe", false},
		{"https://example.com/update-test/%252e%252e/package.exe", false},
		{"https://example.com/update-test/sub%2fpackage.exe", false},
		{"https://example.com/update-test/sub%5cpackage.exe", false},
		{"https://example.com/update-test//package.exe", false},
		{"https://example.com/update-test/%00package.exe", false},
		{"https://example.com/update-test/package.exe#fragment", false},
		{"https://example.com:8443/update-test/package.exe", false},
		{"https://example.com.evil.test/update-test/package.exe", false},
		{"https://user@example.com/update-test/package.exe", false},
		{"http://example.com/update-test/package.exe", false},
	} {
		t.Run(test.target, func(t *testing.T) {
			target, err := url.Parse(test.target)
			if err != nil {
				t.Fatal(err)
			}
			if got := WithinSource(target, "https://example.com/update-test/"); got != test.want {
				t.Errorf("WithinSource = %v", got)
			}
		})
	}
	target, _ := url.Parse("https://example.com/package.exe")
	if !WithinSource(target, "https://example.com/") {
		t.Error("root source should accept a child file")
	}
	for _, source := range []string{"http://example.com", "https://user@example.com", "https://example.com?path=/update-test", "https://example.com/../update-test"} {
		if WithinSource(target, source) {
			t.Errorf("accepted invalid source %q", source)
		}
	}
}
