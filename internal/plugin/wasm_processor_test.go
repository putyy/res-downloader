package plugin

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
	"time"
)

func TestWASMPluginFixtureAndProcessor(t *testing.T) {
	directory := filepath.Join("..", "..", "examples", "plugins", "wasm-xor")
	runtime, manifestPath, err := LoadExternalPlugin(directory)
	if err != nil {
		t.Fatal(err)
	}
	manifest := runtime.Manifest()
	if manifest.Processors["xor"].Runtime != "wasm" {
		t.Fatalf("unexpected processor declaration: %#v", manifest.Processors)
	}
	if err := ReplayPluginFixture(directory, filepath.Join(directory, "fixtures", "video.json")); err != nil {
		t.Fatal(err)
	}

	manager := &PluginManager{
		plugins: []managedPlugin{{runtime: runtime, path: manifestPath}},
		statuses: map[string]shared.PluginStatus{
			manifest.ID: {Manifest: manifest, Path: manifestPath, Loaded: true},
		},
		settings: map[string]map[string]interface{}{},
	}
	plan, err := manager.Resolve(context.Background(), shared.ResourceCandidate{
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: "video", URL: "https://cdn.example.com/encrypted.mp4", Extension: ".mp4",
			Processors: []shared.DownloadStep{{
				Type: wasmProcessorType,
				Options: map[string]interface{}{
					wasmProcessorIDOption: "xor",
					wasmProcessorOwnerKey: "another.plugin",
					"key":                 float64(90),
				},
			}},
		}},
		Source: shared.ResourceSource{PluginID: manifest.ID},
	}, shared.DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	processor := plan.Inputs[0].Processors[0]
	if got := processor.Options[wasmProcessorOwnerKey]; got != manifest.ID {
		t.Fatalf("processor owner = %#v, expected %q", got, manifest.ID)
	}
	spec, err := manager.resolveWASMProcessor(processor)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := spec.options[wasmProcessorOwnerKey]; exists {
		t.Fatal("host-only processor owner leaked into guest options")
	}
	if _, exists := spec.options[wasmProcessorIDOption]; exists {
		t.Fatal("host-only processor id leaked into guest options")
	}

	plaintext := make([]byte, pluginWASMChunkSize*2)
	for index := range plaintext {
		plaintext[index] = byte(index % 251)
	}
	encrypted := append([]byte(nil), plaintext...)
	for index := range encrypted {
		encrypted[index] ^= 90
	}
	sourcePath := filepath.Join(t.TempDir(), "encrypted.bin")
	if err := os.WriteFile(sourcePath, encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	outputPath, err := runWASMProcessor(context.Background(), spec, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outputPath) })
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, plaintext) {
		t.Fatal("WASM processor output does not match plaintext")
	}
}

func TestWASMProcessorDeclarationRequiresPermission(t *testing.T) {
	manifest := shared.PluginManifest{
		ID: "example.no-permission", Name: "example", Version: "1.0.0",
		APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js",
		Permissions: shared.PluginPermissions{Domains: []string{"example.com"}},
		Processors: map[string]shared.PluginProcessorDefinition{
			"decrypt": {Runtime: "wasm", Entry: "decrypt.wasm", APIVersion: shared.PluginProcessorAPIVersion},
		},
	}
	err := validateManifest(manifest)
	if err == nil || !strings.Contains(err.Error(), "process-download") {
		t.Fatalf("expected process-download permission error, got %v", err)
	}
}

func TestWASMProcessorEntryCannotEscapePluginDirectory(t *testing.T) {
	parent := t.TempDir()
	directory := filepath.Join(parent, "plugin")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside.wasm")
	if err := os.WriteFile(outside, []byte("\x00asm\x01\x00\x00\x00\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := securePluginFilePath(directory, "../outside.wasm")
	if err == nil || !strings.Contains(err.Error(), "inside the plugin directory") {
		t.Fatalf("expected path containment error, got %v", err)
	}
}

func TestWASMProcessorInstantiationTimeout(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "resource.bin")
	if err := os.WriteFile(sourcePath, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err := runWASMProcessor(context.Background(), wasmProcessorSpec{
		pluginID: "test.timeout", processorID: "spin",
		path: filepath.Join("testdata", "wasm", "infinite_start.wasm"),
	}, sourcePath)
	if err == nil || !strings.Contains(err.Error(), "instantiate") {
		t.Fatalf("expected instantiation timeout, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > pluginWASMCallTimeout+2*time.Second {
		t.Fatalf("WASM instantiation timeout took %s", elapsed)
	}
}
