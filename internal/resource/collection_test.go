package resource

import (
	"path/filepath"
	shared "res-downloader/internal/model"
	"testing"
)

func TestResourceViewTreeGroupsChildrenAndKeepsTheirOrder(t *testing.T) {
	parent := shared.ResourceCandidate{ID: "parent", GroupKey: "post", DedupeKey: "parent-sign", Kind: shared.ResourceKindCollection, Source: shared.ResourceSource{PluginID: "example"}}
	second := shared.ResourceCandidate{
		ID: "second", GroupKey: "post:image:2", ParentGroupKey: "post", DedupeKey: "second-sign", Kind: "media.image",
		Tracks:   []shared.ResourceTrack{{ID: "image-2", Role: "image", URL: "https://example.com/2.jpg", Size: 20}},
		Metadata: map[string]interface{}{"collectionIndex": float64(2)}, Source: shared.ResourceSource{PluginID: "example"},
	}
	first := shared.ResourceCandidate{
		ID: "first", GroupKey: "post:image:1", ParentID: "parent", DedupeKey: "first-sign", Kind: "media.image",
		Tracks:   []shared.ResourceTrack{{ID: "image-1", Role: "image", URL: "https://example.com/1.jpg", Size: 10}},
		Metadata: map[string]interface{}{"collectionIndex": float64(1)}, Source: shared.ResourceSource{PluginID: "example"},
	}
	items := resourceViewTree([]shared.ResourceCandidate{second, parent, first})
	if len(items) != 1 || len(items[0].Children) != 2 {
		t.Fatalf("tree = %#v", items)
	}
	if items[0].Children[0].ID != "first" || items[0].Children[1].ID != "second" {
		t.Fatalf("ordered tree = %#v", items[0])
	}
}

func TestDeletingCollectionCascadesButDeletingChildDoesNot(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	resource := &Resource{store: store, resType: map[string]bool{"all": true}}
	parent := shared.ResourceCandidate{ID: "parent", GroupKey: "post", DedupeKey: "parent-sign", Kind: shared.ResourceKindCollection, Source: shared.ResourceSource{PluginID: "example"}}
	first := shared.ResourceCandidate{ID: "first", GroupKey: "post:1", ParentID: "parent", DedupeKey: "first-sign", Kind: "media.image", Source: shared.ResourceSource{PluginID: "example"}}
	second := shared.ResourceCandidate{ID: "second", GroupKey: "post:2", ParentGroupKey: "post", DedupeKey: "second-sign", Kind: "media.image", Source: shared.ResourceSource{PluginID: "example"}}
	foreign := shared.ResourceCandidate{ID: "foreign", GroupKey: "foreign:1", ParentID: "parent", DedupeKey: "foreign-sign", Kind: "media.image", Source: shared.ResourceSource{PluginID: "another-plugin"}}
	for _, candidate := range []shared.ResourceCandidate{parent, first, second, foreign} {
		resource.catalog.Store(candidate.ID, candidate)
		resource.mediaMark.Store(candidate.DedupeKey, true)
		resource.groupIndex.Store(resourceGroupIndexKey(candidate.Source.PluginID, candidate.GroupKey), candidate.ID)
		if err := store.Upsert(candidate); err != nil {
			t.Fatal(err)
		}
	}

	resource.deleteMany([]string{"first"})
	if _, exists := resource.catalog.Load("first"); exists {
		t.Fatal("selected child was not removed")
	}
	if _, exists := resource.catalog.Load("parent"); !exists {
		t.Fatal("deleting a child removed its parent")
	}
	if _, exists := resource.catalog.Load("second"); !exists {
		t.Fatal("deleting a child removed its sibling")
	}

	resource.deleteMany([]string{"parent"})
	if records, err := store.List(); err != nil || len(records) != 1 || records[0].Candidate.ID != "foreign" {
		t.Fatalf("records after cascade = %#v, %v", records, err)
	}
	if _, exists := resource.catalog.Load("second"); exists {
		t.Fatal("collection child survived cascade delete")
	}
	if _, exists := resource.catalog.Load("foreign"); !exists {
		t.Fatal("collection delete crossed the plugin boundary")
	}
}
