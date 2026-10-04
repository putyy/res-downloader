package resource

import (
	"path/filepath"
	shared "res-downloader/internal/model"
	"testing"
)

func operationCandidate(group string) shared.ResourceCandidate {
	return shared.ResourceCandidate{
		GroupKey: group, Kind: "media.video", Title: group,
		Source:   shared.ResourceSource{PluginID: "com.example.operation"},
		Tracks:   []shared.ResourceTrack{{ID: "video", Role: "video", URL: "https://example.com/" + group + ".mp4"}},
		Metadata: map[string]interface{}{"original": true},
	}
}

func operationResourceStore(t *testing.T) (*Resource, *Store) {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "resources.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	// Explicit publication must work even when passive capture is deselected.
	return &Resource{store: store, resType: map[string]bool{"all": false}}, store
}

func TestOperationPublicationRequiresStoreAndValidatesWholeBatch(t *testing.T) {
	r := &Resource{resType: map[string]bool{"all": false}}
	events := 0
	r.emit = func(string, ...interface{}) { events++ }
	if got, err := r.PublishOperationCandidates([]shared.ResourceCandidate{operationCandidate("valid")}); err == nil || len(got) != 0 {
		t.Fatalf("memory-only publication reported success: %v, %v", got, err)
	}
	if r.RecordCount() != 0 || events != 0 || len(r.resType) != 1 {
		t.Fatal("failed publication leaked catalog, event or type state")
	}
	r, store := operationResourceStore(t)
	invalid := operationCandidate("invalid")
	invalid.Tracks[0].URL = "file:///private/data"
	if _, err := r.PublishOperationCandidates([]shared.ResourceCandidate{operationCandidate("valid"), invalid}); err == nil {
		t.Fatal("accepted invalid candidate")
	}
	if records, err := store.List(); err != nil || len(records) != 0 || r.RecordCount() != 0 {
		t.Fatalf("partial validation committed resources: %v, %v", records, err)
	}
}

func TestOperationPublicationCommitsMergedTreeBeforeEmitting(t *testing.T) {
	r, store := operationResourceStore(t)
	parent := operationCandidate("parent")
	parent.Kind, parent.Tracks = shared.ResourceKindCollection, nil
	child := operationCandidate("child")
	child.ParentGroupKey = "parent"
	update := operationCandidate("child")
	update.Tracks = []shared.ResourceTrack{{ID: "audio", Role: "audio", URL: "https://example.com/audio.m4a"}}
	events := 0
	r.emit = func(name string, _ ...interface{}) {
		if name != "resourcesBatch" {
			t.Errorf("unexpected event %s", name)
		}
		events++
		records, err := store.List()
		if err != nil || len(records) != 2 || r.RecordCount() != 2 {
			t.Errorf("emitted before durable commit: records=%v err=%v", records, err)
		}
	}
	got, err := r.PublishOperationCandidates([]shared.ResourceCandidate{child, parent, update})
	if err != nil || len(got) != 2 {
		t.Fatalf("publication=%v err=%v", got, err)
	}
	if got[0].GroupKey != "child" || got[1].GroupKey != "parent" || got[0].ParentID != got[1].ID || len(got[0].Tracks) != 2 {
		t.Fatalf("incorrect merged tree: %#v", got)
	}
	if child.ID != "" || len(child.Tracks) != 1 || parent.ID != "" {
		t.Fatal("publication mutated caller candidates")
	}
	if events != 1 {
		t.Fatalf("events=%d", events)
	}
	again, err := r.PublishOperationCandidates([]shared.ResourceCandidate{child})
	if err != nil || len(again) != 1 || again[0].ID != got[0].ID || len(again[0].Tracks) != 2 {
		t.Fatalf("stable group publication failed: %v, %v", again, err)
	}
}

func TestOperationPublicationDiskFailureDoesNotMutateMergedCatalog(t *testing.T) {
	r, store := operationResourceStore(t)
	initial, err := r.PublishOperationCandidates([]shared.ResourceCandidate{operationCandidate("existing")})
	if err != nil {
		t.Fatal(err)
	}
	events := 0
	r.emit = func(string, ...interface{}) { events++ }
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	update := operationCandidate("existing")
	update.Title = "changed"
	update.Metadata["new"] = true
	update.Tracks[0].URL = "https://example.com/changed.mp4"
	fresh := operationCandidate("fresh")
	if got, err := r.PublishOperationCandidates([]shared.ResourceCandidate{update, fresh}); err == nil || len(got) != 0 {
		t.Fatalf("failed transaction reported success: %v %v", got, err)
	}
	stored, ok := r.Candidate(initial[0].ID)
	if !ok || stored.Title != "existing" || stored.Metadata["new"] != nil || stored.Tracks[0].URL != initial[0].Tracks[0].URL {
		t.Fatalf("failed merge modified stored snapshot: %#v", stored)
	}
	if _, exists := r.CandidateByGroup(fresh.Source.PluginID, fresh.GroupKey); exists {
		t.Fatal("failed publication leaked group index")
	}
	if err := validateCandidate(&fresh); err != nil {
		t.Fatal(err)
	}
	if r.mediaIsMarked(fresh.DedupeKey) || r.RecordCount() != 1 || events != 0 {
		t.Fatal("failed publication leaked mark, catalog or event")
	}
}

func TestOperationPublicationRejectsParentCyclesAndForeignParents(t *testing.T) {
	r, store := operationResourceStore(t)
	a, b := operationCandidate("a"), operationCandidate("b")
	a.ParentGroupKey, b.ParentGroupKey = "b", "a"
	if _, err := r.PublishOperationCandidates([]shared.ResourceCandidate{a, b}); err == nil {
		t.Fatal("accepted parent cycle")
	}
	foreign := operationCandidate("foreign")
	foreign.Source.PluginID = "com.example.foreign"
	parents, err := r.PublishOperationCandidates([]shared.ResourceCandidate{foreign})
	if err != nil {
		t.Fatal(err)
	}
	child := operationCandidate("child")
	child.ParentID = parents[0].ID
	if _, err := r.PublishOperationCandidates([]shared.ResourceCandidate{child}); err == nil {
		t.Fatal("accepted cross-plugin parent")
	}
	if records, err := store.List(); err != nil || len(records) != 1 {
		t.Fatalf("rejected parents leaked records: %v %v", records, err)
	}
}

func TestOperationPublicationReturnsExistingDedupeID(t *testing.T) {
	r, _ := operationResourceStore(t)
	candidate := operationCandidate("same")
	candidate.GroupKey = ""
	first, err := r.PublishOperationCandidates([]shared.ResourceCandidate{candidate})
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.PublishOperationCandidates([]shared.ResourceCandidate{candidate, candidate})
	if err != nil || len(second) != 1 || second[0].ID != first[0].ID || r.RecordCount() != 1 {
		t.Fatalf("dedupe did not return durable ID: %v %v", second, err)
	}
}

func TestPassivePublicationKeepsMemoryFallback(t *testing.T) {
	r := &Resource{resType: map[string]bool{"all": true}}
	candidate := operationCandidate("memory")
	if err := validateCandidate(&candidate); err != nil {
		t.Fatal(err)
	}
	r.PublishCandidates([]shared.ResourceCandidate{candidate})
	if _, ok := r.CandidateByGroup(candidate.Source.PluginID, candidate.GroupKey); !ok {
		t.Fatal("passive memory-only publication regressed")
	}
	r, store := operationResourceStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	r.PublishCandidates([]shared.ResourceCandidate{candidate})
	if _, ok := r.CandidateByGroup(candidate.Source.PluginID, candidate.GroupKey); !ok {
		t.Fatal("passive publication lost disk-failure fallback")
	}
}
