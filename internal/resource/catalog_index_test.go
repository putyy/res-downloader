package resource

import (
	"path/filepath"
	"testing"

	shared "res-downloader/internal/model"
)

func TestSaveCandidateReplacesOldIndexes(t *testing.T) {
	resource := &Resource{}
	original := shared.ResourceCandidate{ID: "resource", DedupeKey: "old-key", GroupKey: "old-group", Source: shared.ResourceSource{PluginID: "plugin"}}
	if err := resource.SaveCandidate(original); err != nil {
		t.Fatal(err)
	}
	updated := original
	updated.DedupeKey, updated.GroupKey = "new-key", "new-group"
	if err := resource.SaveCandidate(updated); err != nil {
		t.Fatal(err)
	}
	if _, exists := resource.CandidateByGroup("plugin", original.GroupKey); exists || resource.dedupeIndex[original.DedupeKey] != "" {
		t.Fatal("replacing a resource left stale group or deduplication indexes")
	}
	if candidate, exists := resource.CandidateByGroup("plugin", updated.GroupKey); !exists || candidate.DedupeKey != updated.DedupeKey || resource.RecordCount() != 1 {
		t.Fatalf("updated group is not linked to the current candidate: %#v", candidate)
	}
	resource.DeleteMany([]string{updated.ID})
	if _, exists := resource.CandidateByGroup("plugin", updated.GroupKey); exists || resource.dedupeIndex[updated.DedupeKey] != "" {
		t.Fatal("deleting the replacement left its indexes behind")
	}
}

func TestSaveCandidateDoesNotPublishFailedPersistence(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	resource := &Resource{store: store}
	original := shared.ResourceCandidate{ID: "resource", Title: "Original", DedupeKey: "original-key", GroupKey: "original-group"}
	if err := resource.SaveCandidate(original); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	updated := original
	updated.Title, updated.DedupeKey, updated.GroupKey = "Failed update", "failed-key", "failed-group"
	if err := resource.SaveCandidate(updated); err == nil {
		t.Fatal("saving to a closed database succeeded")
	}
	if candidate, exists := resource.CandidateByGroup("", original.GroupKey); !exists || candidate.Title != original.Title {
		t.Fatalf("failed persistence changed the catalog: %#v", candidate)
	}
	if _, exists := resource.CandidateByGroup("", updated.GroupKey); exists || resource.dedupeIndex[updated.DedupeKey] != "" {
		t.Fatal("failed persistence published replacement indexes")
	}
}
