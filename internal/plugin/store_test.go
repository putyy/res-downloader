package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestPluginStoreIndexRejectsDuplicateIDsAndInvalidPackageReferences(t *testing.T) {
	index := validPluginStoreTestIndex()
	index.Extensions = append(index.Extensions, index.Extensions[0])
	if err := ValidatePluginStoreIndex(index); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	for _, repository := range []string{"owner/../repo", "owner/..", "owner/.", "owner/repo?redirect=evil", "https://example.com/repo", "owner/repo%2f..", "owner@evil/repo", "owner/repo#fragment"} {
		index = validPluginStoreTestIndex()
		index.Extensions[0].Repository = repository
		if err := ValidatePluginStoreIndex(index); err == nil {
			t.Fatalf("accepted invalid repository %q", repository)
		}
	}
	for _, tag := range []string{"", "latest", "v1.2.0/../other", "v1.2.0?redirect=evil"} {
		index = validPluginStoreTestIndex()
		index.Extensions[0].Release.Tag = tag
		if err := ValidatePluginStoreIndex(index); err == nil {
			t.Fatalf("accepted invalid tag %q", tag)
		}
	}
	for _, id := range []string{"builtin.demo", "official.demo", "Official.demo"} {
		index = validPluginStoreTestIndex()
		index.Extensions[0].ID = id
		if err := ValidatePluginStoreIndex(index); err == nil {
			t.Fatalf("accepted reserved plugin id %q in a community repository", id)
		}
	}
	index = validPluginStoreTestIndex()
	index.Extensions[0].Source = shared.PluginSourceOfficial
	if err := ValidatePluginStoreIndex(index); err == nil {
		t.Fatal("accepted an official source for a community repository")
	}
}

func validPluginStoreTestIndex() shared.PluginStoreIndex {
	return shared.PluginStoreIndex{
		SchemaVersion: shared.PluginStoreSchemaVersion,
		GeneratedAt:   "2026-08-16T00:00:00Z",
		Topic:         shared.PluginStoreTopic,
		Extensions: []shared.PluginStoreEntry{{
			ID: "com.example.video", Name: "Example", Repository: "owner/repo",
			Source: shared.PluginSourceCommunity, Status: shared.PluginStoreAvailable,
			Release: &shared.PluginStoreRelease{Tag: "v1.2.0"},
		}},
	}
}

func TestLocalArchiveStoreMatch(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	index := validPluginStoreTestIndex()
	if err := writePluginStoreCache(manager.pluginStoreCacheFile(), index); err != nil {
		t.Fatal(err)
	}
	manifest := shared.PluginManifest{ID: index.Extensions[0].ID, Version: index.Extensions[0].Release.Version()}
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
	entry.Release.Accelerated = true
	if err := writePluginStoreCache(manager.pluginStoreCacheFile(), index); err != nil {
		t.Fatal(err)
	}
	archive := pluginTestArchive(t, map[string]string{
		"plugin.json": `{"id":"com.example.video","name":"Package name","version":"1.2.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"]}}`,
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
	manifest, source, err := manager.InstallFromStore(context.Background(), entry.ID, entry.Release.Version(), false)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != entry.ID || manifest.Name != "Package name" || source != "https://github.com/owner/repo/archive/refs/tags/v1.2.0.zip" {
		t.Fatalf("manifest/source = %#v, %q", manifest, source)
	}
}

func TestInstallFromStoreRejectsMismatchedPackageIdentity(t *testing.T) {
	for _, identity := range []struct{ id, version string }{
		{"com.example.other", "1.2.0"},
		{"com.example.video", "1.2.1"},
	} {
		t.Run(identity.id+"-"+identity.version, func(t *testing.T) {
			manager := newInstallTestPluginManager(t)
			index := validPluginStoreTestIndex()
			if err := writePluginStoreCache(manager.pluginStoreCacheFile(), index); err != nil {
				t.Fatal(err)
			}
			archive := pluginTestArchive(t, map[string]string{
				"plugin.json": fmt.Sprintf(`{"id":%q,"name":"Example","version":%q,"apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"]}}`, identity.id, identity.version),
				"main.js":     `function onObservation() { return {decision: "continue"} }`,
			})
			originalDownload := downloadStorePluginArchive
			t.Cleanup(func() { downloadStorePluginArchive = originalDownload })
			downloadStorePluginArchive = func(context.Context, NetworkSettings, string) ([]byte, error) { return archive, nil }
			if _, _, err := manager.InstallFromStore(context.Background(), "com.example.video", "1.2.0", false); err == nil || !strings.Contains(err.Error(), "id or version") {
				t.Fatalf("expected package identity mismatch, got %v", err)
			}
			if _, installed := manager.Status(identity.id); installed {
				t.Fatal("installed a package with a mismatched identity")
			}
		})
	}
}

func TestPluginStoreAcceptsMoreThanOneThousandEntries(t *testing.T) {
	index := validPluginStoreTestIndex()
	for position := 0; position < 1001; position++ {
		index.Extensions = append(index.Extensions, shared.PluginStoreEntry{
			Name: "Unavailable", Repository: fmt.Sprintf("owner/repo-%d", position),
			Source: shared.PluginSourceCommunity, Status: shared.PluginStoreUnavailable,
		})
	}
	raw, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePluginStoreIndex(raw); err != nil {
		t.Fatal(err)
	}
}

func TestPluginStoreRejectsOversizedIndex(t *testing.T) {
	if _, err := decodePluginStoreIndex(make([]byte, shared.PluginStoreMaxIndexSize+1)); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestStoreArchiveURLsDeriveFromRepositoryAndTag(t *testing.T) {
	entry := validPluginStoreTestIndex().Extensions[0]
	entry.Release.Tag = "1.2.0+build.1"
	entry.Release.Accelerated = true
	archive, accelerated := storeArchiveURLs(entry)
	if archive != "https://github.com/owner/repo/archive/refs/tags/1.2.0+build.1.zip" || accelerated != "https://cdn.jsdelivr.net/gh/owner/repo@1.2.0+build.1/dist/plugin.zip" {
		t.Fatalf("unexpected URLs: %q, %q", archive, accelerated)
	}
	if entry.Release.Version() != "1.2.0+build.1" {
		t.Fatalf("version = %q", entry.Release.Version())
	}
	entry.Release.Accelerated = false
	_, accelerated = storeArchiveURLs(entry)
	if accelerated != "" {
		t.Fatalf("unexpected acceleration URL: %q", accelerated)
	}
}
