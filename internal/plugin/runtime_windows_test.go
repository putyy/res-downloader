package plugin

import (
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
)

func TestJavaScriptEntryFromRelativeDirectoryWithDifferentCase(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte("function onObservation() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	// Like Windows short names, differently cased paths need canonicalisation
	// before comparing the root with the resolved entry.
	t.Chdir(strings.ToLower(directory))
	_, err := newJavaScriptPlugin(".", shared.PluginManifest{
		ID: "test.relative", Name: "Relative", Version: "1", APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js",
	})
	if err != nil {
		t.Fatalf("load plugin from relative directory: %v", err)
	}
}
