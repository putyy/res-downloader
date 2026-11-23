package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"res-downloader/internal/logging"
	"testing"
)

func TestUnknownThemeFallsBackToLight(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "config.json"), []byte(`{"Theme":"unknown","Locale":"zh"}`), 0600); err != nil {
		t.Fatal(err)
	}
	current := New(directory, logging.New(false, ""), "en")
	if current.Theme != "light" || current.Locale != "zh" {
		t.Fatalf("loaded theme = %q, locale = %q", current.Theme, current.Locale)
	}
	next := current.Snapshot()
	next.Theme = "unknown"
	if err := current.Apply(next); err != nil {
		t.Fatal(err)
	}
	if current.Snapshot().Theme != "light" {
		t.Fatal("applying an unknown theme should select light")
	}
}

func TestThemeOverridesCanBeResetIndependently(t *testing.T) {
	directory := t.TempDir()
	logger := logging.New(false, "")
	current := New(directory, logger, "en")
	next := current.Snapshot()
	next.Theme = "dark"
	next.ThemeColors = ThemeColors{"dark": {"text": "#dddddd"}, "light": {"text": "#222222"}}
	if err := current.Apply(next); err != nil {
		t.Fatal(err)
	}

	pending := current.Snapshot()
	delete(pending.ThemeColors["dark"], "text")
	if current.Snapshot().ThemeColors["dark"]["text"] != "#dddddd" {
		t.Fatal("editing the snapshot changed the saved palette")
	}
	delete(pending.ThemeColors, "dark")
	if err := current.Apply(pending); err != nil {
		t.Fatal(err)
	}
	reopened := New(directory, logger, "en")
	want := ThemeColors{"light": {"text": "#222222"}}
	if reopened.Theme != "dark" || !reflect.DeepEqual(reopened.ThemeColors, want) {
		t.Fatalf("reopened theme = %q, overrides = %v, want dark with %v", reopened.Theme, reopened.ThemeColors, want)
	}
}

func TestApplyRejectsInvalidSettingsWithoutReplacingConfig(t *testing.T) {
	base := Config{
		Host: "127.0.0.1", Port: "8899", TaskNumber: 2, DownNumber: 1,
		FilenameTemplate: "{{title}}.{{ext}}", FilenameConflict: "rename",
		SaveDirectory: t.TempDir(),
	}
	missingDirectory := filepath.Join(t.TempDir(), "missing")
	for _, test := range []struct {
		name  string
		field string
		edit  func(*Config)
	}{
		{"template", "FilenameTemplate", func(value *Config) { value.FilenameTemplate = "../{{title}}" }},
		{"directory", "SaveDirectory", func(value *Config) { value.SaveDirectory = missingDirectory }},
		{"host", "Host", func(value *Config) { value.Host = "999.999.999.999" }},
		{"bracketed IPv6 host", "Host", func(value *Config) { value.Host = "[::1]" }},
		{"port", "Port", func(value *Config) { value.Port = "8899abc" }},
		{"proxy URL", "UpstreamProxy", func(value *Config) { value.UpstreamProxy = "http://127.0.0.1:0" }},
		{"proxy required", "UpstreamProxy", func(value *Config) { value.DownloadProxy = true }},
		{"FFmpeg relative path", "FFmpegPath", func(value *Config) { value.FFmpegPath = "ffmpeg" }},
		{"FFprobe missing file", "FFprobePath", func(value *Config) { value.FFprobePath = missingDirectory }},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := base
			next := base
			test.edit(&next)
			err := current.Apply(next)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("validation error = %v, want field %q", err, test.field)
			}
			if got := current.Snapshot(); !reflect.DeepEqual(got, base.Snapshot()) {
				t.Fatalf("invalid settings changed config: %#v", got)
			}
		})
	}
}
