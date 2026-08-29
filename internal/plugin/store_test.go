package plugin

import (
	"context"
	"encoding/json"
	"errors"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
)

func TestDecodePluginStoreIndex(t *testing.T) {
	index := validPluginStoreTestIndex()
	raw, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodePluginStoreIndex(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Extensions) != 1 || decoded.Extensions[0].ID != "com.example.video" {
		t.Fatalf("unexpected index: %#v", decoded)
	}
}

func TestPluginStoreIndexRejectsDuplicateIDsAndMismatchedArchiveURLs(t *testing.T) {
	index := validPluginStoreTestIndex()
	index.Extensions = append(index.Extensions, index.Extensions[0])
	if err := validatePluginStoreIndex(index); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	index = validPluginStoreTestIndex()
	index.Extensions[0].Release.ArchiveURL = "https://github.com/another/repo/archive/refs/tags/v1.2.0.zip"
	if err := validatePluginStoreIndex(index); err == nil || !strings.Contains(err.Error(), "archive URL") {
		t.Fatalf("expected archive URL error, got %v", err)
	}

	index = validPluginStoreTestIndex()
	index.Extensions[0].Release.AcceleratedURL = "https://cdn.jsdelivr.net/gh/another/repo@v1.2.0/dist/plugin.zip"
	if err := validatePluginStoreIndex(index); err == nil || !strings.Contains(err.Error(), "accelerated archive URL") {
		t.Fatalf("expected accelerated URL error, got %v", err)
	}
}

func validPluginStoreTestIndex() shared.PluginStoreIndex {
	manifest := &shared.PluginManifest{
		ID: "com.example.video", Name: "Example", Version: "1.2.0", APIVersion: shared.PluginAPIVersion,
		Runtime: "javascript", Entry: "main.js",
		Permissions: shared.PluginPermissions{Domains: []string{"example.com"}},
	}
	return shared.PluginStoreIndex{
		SchemaVersion: shared.PluginStoreSchemaVersion,
		GeneratedAt:   "2026-08-16T00:00:00Z",
		Topic:         shared.PluginStoreTopic,
		Extensions: []shared.PluginStoreEntry{{
			ID: "com.example.video", Name: "Example", Repository: "owner/repo",
			RepositoryURL: "https://github.com/owner/repo", Owner: "owner", Source: shared.PluginSourceCommunity, Status: shared.PluginStoreAvailable,
			Manifest: manifest,
			Release: &shared.PluginStoreRelease{
				Version: "1.2.0", Tag: "v1.2.0",
				ArchiveURL: "https://github.com/owner/repo/archive/refs/tags/v1.2.0.zip",
			},
		}},
	}
}

func TestLocalArchiveStoreMatch(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	index := validPluginStoreTestIndex()
	if err := writePluginStoreCache(manager.pluginStoreCacheFile(), index); err != nil {
		t.Fatal(err)
	}
	manifest := *index.Extensions[0].Manifest
	if match := manager.localArchiveStoreMatch(manifest); match != "same-version" {
		t.Fatalf("match = %q", match)
	}
	manifest.Version = "1.2.1"
	if match := manager.localArchiveStoreMatch(manifest); match != "different" {
		t.Fatalf("different match = %q", match)
	}
}

func TestInstallFromStoreFallsBackAfterInvalidAcceleratedPackage(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	index := validPluginStoreTestIndex()
	entry := &index.Extensions[0]
	entry.Release.AcceleratedURL = "https://cdn.jsdelivr.net/gh/owner/repo@v1.2.0/dist/plugin.zip"
	if err := writePluginStoreCache(manager.pluginStoreCacheFile(), index); err != nil {
		t.Fatal(err)
	}
	archive := pluginTestArchive(t, map[string]string{
		"plugin.json": `{"id":"com.example.video","name":"Example","version":"1.2.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"]}}`,
		"main.js":     `function onObservation() { return {decision: "continue"} }`,
	})
	originalDownload := downloadStorePluginArchive
	t.Cleanup(func() { downloadStorePluginArchive = originalDownload })
	downloadStorePluginArchive = func(_ context.Context, _ NetworkSettings, rawURL string) ([]byte, error) {
		if strings.Contains(rawURL, "cdn.jsdelivr.net") {
			return []byte("not a ZIP"), nil
		}
		if strings.Contains(rawURL, "github.com") {
			return archive, nil
		}
		return nil, errors.New("unexpected URL")
	}
	manifest, source, err := manager.InstallFromStore(context.Background(), entry.ID, entry.Release.Version, false)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != entry.ID || source != entry.Release.ArchiveURL {
		t.Fatalf("manifest/source = %#v, %q", manifest, source)
	}
}
