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

func TestManifestAcceptsOperationAction(t *testing.T) {
	manifest := operationTestManifest()
	if err := validateManifest(manifest); err != nil {
		t.Fatal(err)
	}
}

func TestManifestRejectsInvalidOperationAction(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*shared.PluginManifest)
	}{
		{
			name: "missing page bridge",
			mutate: func(manifest *shared.PluginManifest) {
				manifest.Permissions.Capabilities = []string{"inject-page-script"}
			},
		},
		{
			name: "missing page script",
			mutate: func(manifest *shared.PluginManifest) {
				action := manifest.Actions["inspect"]
				action.Operation = ""
				manifest.Actions["inspect"] = action
			},
		},
		{
			name: "unbridged page script",
			mutate: func(manifest *shared.PluginManifest) {
				manifest.PageScripts[0].Bridge = false
			},
		},
		{
			name: "file processor fields",
			mutate: func(manifest *shared.PluginManifest) {
				action := manifest.Actions["inspect"]
				action.Processor = "processor"
				manifest.Actions["inspect"] = action
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := operationTestManifest()
			test.mutate(&manifest)
			if err := validateManifest(manifest); err == nil {
				t.Fatal("expected invalid operation action to be rejected")
			}
		})
	}
}

func operationTestManifest() shared.PluginManifest {
	return shared.PluginManifest{
		ID: "example.operation", Name: "Operation", Version: "1.0.0", APIVersion: shared.PluginAPIVersion,
		Runtime: "javascript", Entry: "main.js",
		Permissions: shared.PluginPermissions{
			Domains: []string{"www.example.com"}, Capabilities: []string{"inject-page-script", "page-bridge"},
		},
		PageScripts: []shared.PluginPageScript{{
			ID: "controller", Entry: "page/controller.js", Bridge: true,
			Match: []shared.PluginPageScriptMatch{{Host: "www.example.com"}},
		}},
		Operations: map[string]shared.OperationDefinition{"inspect": {Name: "Inspect", Category: "custom", PageScript: "controller", Effects: []string{"read"}, InputSchema: map[string]interface{}{"type": "object", "additionalProperties": false}, OutputSchema: map[string]interface{}{"type": "object", "additionalProperties": false}}},
		Actions: map[string]shared.PluginActionDefinition{
			"inspect": {Kind: shared.PluginActionOperation, Operation: "inspect"},
		},
	}
}
