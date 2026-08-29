package plugin

import (
	shared "res-downloader/internal/model"
	"strings"
	"testing"
)

func TestManifestRejectsReservedPluginIDPrefixes(t *testing.T) {
	for _, id := range []string{"builtin.example", "Builtin.example", "official.example", "OFFICIAL.example"} {
		manifest := reservedPrefixTestManifest(id)
		if err := validateManifest(manifest); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("expected %q to be rejected as reserved, got %v", id, err)
		}
	}
}

func TestTrustedBundledManifestAllowsOfficialPrefixOnly(t *testing.T) {
	manifest := reservedPrefixTestManifest("official.example")
	if err := validateManifestForSource(manifest, true); err != nil {
		t.Fatalf("trusted bundled manifest was rejected: %v", err)
	}
	manifest.ID = "Builtin.example"
	if err := validateManifestForSource(manifest, true); err == nil {
		t.Fatal("builtin prefix must remain reserved for native plugins")
	}
}

func reservedPrefixTestManifest(id string) shared.PluginManifest {
	return shared.PluginManifest{
		ID: id, Name: "Reserved Prefix", Version: "1.0.0", APIVersion: shared.PluginAPIVersion,
		Runtime:     "declarative",
		Permissions: shared.PluginPermissions{Domains: []string{"example.com"}},
	}
}

func TestManifestRejectsUnknownFileActionProcessor(t *testing.T) {
	manifest := shared.PluginManifest{
		ID: "example.action", Name: "Action", Version: "1", APIVersion: shared.PluginAPIVersion,
		Runtime: "javascript", Entry: "main.js",
		Permissions: shared.PluginPermissions{Domains: []string{"example.com"}, Capabilities: []string{"process-download"}},
		Actions: map[string]shared.PluginActionDefinition{
			"decrypt": {Kind: shared.PluginActionProcessFile, Processor: "missing", OutputExtension: ".mp4"},
		},
	}
	if err := validateManifest(manifest); err == nil {
		t.Fatal("expected unknown action processor to be rejected")
	}
}
