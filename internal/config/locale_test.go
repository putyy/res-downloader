package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"res-downloader/internal/logging"
	"testing"
)

func TestInitialLocaleIsPersisted(t *testing.T) {
	for _, language := range []string{"zh", "en", "unsupported"} {
		t.Run(language, func(t *testing.T) {
			directory := t.TempDir()
			logger := logging.New(false, "")
			want := "en"
			if language == "zh" {
				want = "zh"
			}
			created := New(directory, logger, language)
			if created.Locale != want {
				t.Fatalf("initial locale = %q, want %q", created.Locale, want)
			}
			data, err := os.ReadFile(filepath.Join(directory, "config.json"))
			if err != nil {
				t.Fatal(err)
			}
			var saved struct{ Locale string }
			if err := json.Unmarshal(data, &saved); err != nil || saved.Locale != want {
				t.Fatalf("saved locale = %q, want %q; error: %v", saved.Locale, want, err)
			}
			other := "zh"
			if want == "zh" {
				other = "en"
			}
			if reopened := New(directory, logger, other); reopened.Locale != want {
				t.Fatalf("saved language was replaced by the new system default: %q", reopened.Locale)
			}
		})
	}
}

func TestExistingLocaleIsPreserved(t *testing.T) {
	directory := t.TempDir()
	// A language stored by an older version or selected manually remains authoritative.
	if err := os.WriteFile(filepath.Join(directory, "config.json"), []byte(`{"Locale":"en","Theme":"darkTheme"}`), 0600); err != nil {
		t.Fatal(err)
	}
	got := New(directory, logging.New(false, ""), "zh")
	if got.Locale != "en" || got.Theme != "darkTheme" {
		t.Fatalf("existing preferences changed: locale=%q, theme=%q", got.Locale, got.Theme)
	}
}
