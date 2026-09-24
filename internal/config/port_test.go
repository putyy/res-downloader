package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSavePortForRestartPreservesOtherSettings(t *testing.T) {
	directory := t.TempDir()
	c := &Config{
		state: &configState{}, storage: NewStorage(directory, "config.json", nil),
		Host: "127.0.0.1", Port: "8899", Locale: "zh", Theme: "darkTheme",
		SaveDirectory: "legacy directory", FilenameTemplate: "legacy invalid template {{",
		WindowWidth: 1280, WindowHeight: 800, AutoProxy: true,
	}
	c.SetApplyHook(func(Config, Config) error {
		t.Fatal("port recovery must not apply unrelated runtime settings")
		return nil
	})
	want := c.Snapshot()
	want.Port = "18899"
	if err := c.SavePortForRestart("18899"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Snapshot(), want) {
		t.Fatal("port recovery changed unrelated settings")
	}
	data, err := os.ReadFile(filepath.Join(directory, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Snapshot(), want) {
		t.Fatal("saved config differs from in-memory config")
	}
}

func TestSavePortForRestartFailurePreservesConfig(t *testing.T) {
	c := &Config{
		state: &configState{}, storage: NewStorage(filepath.Join(t.TempDir(), "missing"), "config.json", nil),
		Host: "127.0.0.1", Port: "8899",
	}
	for _, port := range []string{"18899", "0", "1024", "65535", "invalid"} {
		if err := c.SavePortForRestart(port); err == nil {
			t.Fatalf("port %q: expected a save or validation error", port)
		}
		if c.Snapshot().Port != "8899" {
			t.Fatal("failed save changed the current port")
		}
	}
}
