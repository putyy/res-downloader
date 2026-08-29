package resource

import (
	"os"
	"path/filepath"
	"res-downloader/internal/config"
	shared "res-downloader/internal/model"
	"testing"
)

func TestResourceStorePersistsCandidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	candidate := shared.ResourceCandidate{
		ID: "resource-1", DedupeKey: "dedupe-1", Kind: "media.video", Title: "Persisted",
		Tracks: []shared.ResourceTrack{{
			ID: "video", Role: "video", URL: "https://cdn.example/video.mp4", Extension: ".mp4",
			Processors: []shared.DownloadStep{{
				Type: "plugin-wasm", Options: map[string]interface{}{"processor": "decrypt", "seed": "secret"},
			}},
		}},
		Actions: []shared.ResourceAction{{
			ID: "decrypt", Data: map[string]interface{}{"options": map[string]interface{}{"seed": "secret"}},
		}},
		Source: shared.ResourceSource{PluginID: "example.plugin"},
	}
	if err := store.Upsert(candidate); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	records, err := reopened.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Candidate.ID != candidate.ID {
		t.Fatalf("records = %#v", records)
	}
	options := records[0].Candidate.Actions[0].Data["options"].(map[string]interface{})
	if options["seed"] != "secret" {
		t.Fatalf("action options = %#v", options)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("database permissions = %o", info.Mode().Perm())
	}
}

func TestResourceStoreOmitsOnlyPluginSelectedHeaders(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	candidate := shared.ResourceCandidate{
		ID: "sensitive", DedupeKey: "sensitive", Kind: "media.video",
		Tracks: []shared.ResourceTrack{{
			ID: "video", Role: "video", URL: "https://cdn.example/video.mp4",
			Headers:              map[string]string{"Cookie": "session=secret", "Authorization": "kept-by-default", "Referer": "https://example.com"},
			NonPersistentHeaders: []string{"cookie"},
		}},
	}
	if err := store.Upsert(candidate); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%#v err=%v", records, err)
	}
	headers := records[0].Candidate.Tracks[0].Headers
	if _, exists := headers["Cookie"]; exists {
		t.Fatal("Cookie was persisted")
	}
	if headers["Authorization"] != "kept-by-default" || headers["Referer"] == "" {
		t.Fatalf("headers=%#v", headers)
	}
	if candidate.Tracks[0].Headers["Cookie"] != "session=secret" {
		t.Fatal("in-memory candidate was mutated")
	}
	if len(records[0].Candidate.Tracks[0].NonPersistentHeaders) != 1 {
		t.Fatal("persistence marker was not retained")
	}
}

func TestResourceStoreDeleteAndClear(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, id := range []string{"a", "b"} {
		if err := store.Upsert(shared.ResourceCandidate{ID: id, DedupeKey: id, Kind: "file.binary"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Delete([]string{"a"}); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil || len(records) != 1 || records[0].Candidate.ID != "b" {
		t.Fatalf("after delete: records=%#v err=%v", records, err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	records, err = store.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("after clear: records=%#v err=%v", records, err)
	}
}

func TestResourceRestoreRebuildsRuntimeIndexes(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	candidate := shared.ResourceCandidate{
		ID: "restored", DedupeKey: "restored-sign", GroupKey: "group-1", Kind: "media.video",
		Tracks:    []shared.ResourceTrack{{ID: "video", Role: "video", URL: "https://cdn.example/video.mp4"}},
		Source:    shared.ResourceSource{PluginID: "example.plugin"},
		Lifecycle: shared.ResourceLifecycle{SchemaVersion: shared.ResourceSchemaVersion, DiscoveredAt: 1000, UpdatedAt: 1100, Availability: shared.ResourceAvailabilityAvailable},
	}
	if err := store.Upsert(candidate); err != nil {
		t.Fatal(err)
	}
	resource := &Resource{store: store, resType: map[string]bool{"all": true}}
	if err := resource.restore(); err != nil {
		t.Fatal(err)
	}
	if _, exists := resource.catalog.Load(candidate.ID); !exists {
		t.Fatal("restored resource is missing from catalog")
	}
	if !resource.mediaIsMarked(candidate.DedupeKey) {
		t.Fatal("restored dedupe key is missing")
	}
	groupID, exists := resource.groupIndex.Load(resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey))
	if !exists || groupID != candidate.ID {
		t.Fatalf("restored group index = %#v, %v", groupID, exists)
	}
	restoredValue, _ := resource.catalog.Load(candidate.ID)
	restored := restoredValue.(shared.ResourceCandidate)
	if restored.Lifecycle.DiscoveredAt != candidate.Lifecycle.DiscoveredAt || restored.Lifecycle.UpdatedAt != candidate.Lifecycle.UpdatedAt {
		t.Fatalf("restored lifecycle=%#v candidate=%#v", restored.Lifecycle, candidate.Lifecycle)
	}
}

func TestResourceRestoreKeepsConfiguredListOrder(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	older := shared.ResourceCandidate{
		ID: "older", DedupeKey: "older-key", Kind: "media.audio",
		Lifecycle: shared.ResourceLifecycle{SchemaVersion: shared.ResourceSchemaVersion, DiscoveredAt: 1000, UpdatedAt: 3100, Availability: shared.ResourceAvailabilityAvailable},
	}
	newer := shared.ResourceCandidate{
		ID: "newer", DedupeKey: "newer-key", Kind: "media.audio",
		Lifecycle: shared.ResourceLifecycle{SchemaVersion: shared.ResourceSchemaVersion, DiscoveredAt: 2000, UpdatedAt: 2100, Availability: shared.ResourceAvailabilityAvailable},
	}
	if err := store.UpsertMany([]shared.ResourceCandidate{older, newer}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		insertTail bool
		want       []string
	}{
		{name: "insert at front", insertTail: false, want: []string{"newer", "older"}},
		{name: "insert at tail", insertTail: true, want: []string{"older", "newer"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			resource := &Resource{
				store: store, config: &config.Config{InsertTail: test.insertTail},
				resType: map[string]bool{"all": true},
			}
			if err := resource.restore(); err != nil {
				t.Fatal(err)
			}
			items := resource.list()
			if len(items) != len(test.want) {
				t.Fatalf("items=%#v", items)
			}
			for index, want := range test.want {
				if items[index].ID != want {
					t.Fatalf("order=%q,%q want=%q,%q", items[0].ID, items[1].ID, test.want[0], test.want[1])
				}
			}
			olderValue, _ := resource.catalog.Load(older.ID)
			newerValue, _ := resource.catalog.Load(newer.ID)
			if olderValue.(shared.ResourceCandidate).Lifecycle.UpdatedAt != older.Lifecycle.UpdatedAt ||
				newerValue.(shared.ResourceCandidate).Lifecycle.UpdatedAt != newer.Lifecycle.UpdatedAt {
				t.Fatal("restore overwrote persisted lifecycle timestamps")
			}
		})
	}
}

func TestImportedResourceBecomesPersistentCandidate(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	resource := &Resource{store: store, resType: map[string]bool{"all": true}}
	items, err := resource.importResources([]shared.ResourceView{{ResourceCandidate: shared.ResourceCandidate{
		ID: "imported", DedupeKey: "imported-sign", Kind: "media.video", Title: "Imported",
		Tracks:       []shared.ResourceTrack{{ID: "primary", Role: "video", URL: "https://cdn.example/imported.mp4", Extension: ".mp4", MIME: "video/mp4"}},
		Capabilities: []string{shared.ResourceCapabilityDownload},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "imported" {
		t.Fatalf("imported items = %#v", items)
	}
	records, err := store.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%#v err=%v", records, err)
	}
	if len(records[0].Candidate.Tracks) != 1 || records[0].Candidate.Tracks[0].URL != "https://cdn.example/imported.mp4" {
		t.Fatalf("candidate = %#v", records[0].Candidate)
	}
}

func TestResourceCatalogSupportsMemoryOnlyFallback(t *testing.T) {
	resource := &Resource{resType: map[string]bool{"all": true}}
	candidate := shared.ResourceCandidate{
		ID: "memory-only", DedupeKey: "memory-only-sign", Kind: "media.image",
		Tracks: []shared.ResourceTrack{{ID: "image", Role: "image", URL: "https://cdn.example/image.jpg"}},
	}
	resource.catalog.Store(candidate.ID, candidate)
	resource.mediaMark.Store(candidate.DedupeKey, true)

	items := resource.list()
	if len(items) != 1 || items[0].ID != candidate.ID {
		t.Fatalf("memory-only list = %#v", items)
	}
	resource.deleteMany([]string{candidate.ID})
	if len(resource.list()) != 0 {
		t.Fatal("memory-only delete did not remove the resource")
	}
	resource.clear()
	resource.Close()
}

func TestDeletedResourceCanBePublishedAgain(t *testing.T) {
	resource := &Resource{resType: map[string]bool{"all": true}}
	candidate := shared.ResourceCandidate{
		DedupeKey: "repeatable-sign", GroupKey: "repeatable-group", Kind: "stream.hls",
		PrimaryType: shared.ResourceTypeVideo,
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: "video", Executor: "hls", URL: "https://cdn.example/index.m3u8",
		}},
		Source: shared.ResourceSource{PluginID: "builtin.generic-detector"},
	}
	resource.PublishCandidate(candidate)
	first := resource.list()
	if len(first) != 1 {
		t.Fatalf("initial resources = %#v", first)
	}
	resource.deleteMany([]string{first[0].ID})
	if resource.mediaIsMarked(candidate.DedupeKey) {
		t.Fatal("deleted resource retained its dedupe marker")
	}
	resource.PublishCandidate(candidate)
	second := resource.list()
	if len(second) != 1 {
		t.Fatalf("recaptured resources = %#v", second)
	}
	if second[0].ID == first[0].ID {
		t.Fatalf("recaptured resource reused deleted ID %q", second[0].ID)
	}
}
