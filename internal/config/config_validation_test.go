package config

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

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
