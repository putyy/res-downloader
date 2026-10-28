package plugin

import (
	"encoding/json"
	"sync"
	"testing"

	shared "res-downloader/internal/model"
)

func TestMergeResourceCandidatePreservesPublishedSnapshot(t *testing.T) {
	current := shared.ResourceCandidate{
		Metadata:       map[string]interface{}{"title": "original"},
		Tracks:         []shared.ResourceTrack{{ID: "video", URL: "https://example.com/old.mp4"}},
		Traits:         []string{"original", "spare"},
		RequiredTracks: []string{"video", "spare"},
		Capabilities:   []string{"download", "spare"},
	}
	// Keep a reader's full slice views, including spare capacity that append
	// would otherwise overwrite without changing the original slice length.
	reader := current
	current.Traits = current.Traits[:1]
	current.RequiredTracks = current.RequiredTracks[:1]
	current.Capabilities = current.Capabilities[:1]
	before, err := json.Marshal(reader)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeResourceCandidate(current, shared.ResourceCandidate{
		Metadata: map[string]interface{}{"title": "updated", "quality": "HD"},
		Tracks:   []shared.ResourceTrack{{ID: "video", URL: "https://example.com/new.mp4"}, {ID: "audio", Role: "audio"}},
		Traits:   []string{"updated"}, RequiredTracks: []string{"audio"}, Capabilities: []string{"preview"},
	})
	after, err := json.Marshal(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("published snapshot changed: before=%s after=%s", before, after)
	}
	if merged.Metadata["title"] != "updated" || merged.Metadata["quality"] != "HD" || len(merged.Tracks) != 2 || merged.Tracks[0].URL != "https://example.com/new.mp4" {
		t.Fatalf("merge lost updates: %#v", merged)
	}
}

func TestMergeResourceCandidateAllowsConcurrentSnapshotReaders(t *testing.T) {
	current := shared.ResourceCandidate{
		Metadata: map[string]interface{}{"title": "original"},
		Tracks:   []shared.ResourceTrack{{ID: "video", URL: "https://example.com/old.mp4"}},
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func(writer bool) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if writer {
					MergeResourceCandidate(current, shared.ResourceCandidate{
						Metadata: map[string]interface{}{"title": "updated"},
						Tracks:   []shared.ResourceTrack{{ID: "video", URL: "https://example.com/new.mp4"}},
					})
				} else if _, err := json.Marshal(current); err != nil {
					t.Error(err)
				}
			}
		}(worker != 0)
	}
	wg.Wait()
	if current.Metadata["title"] != "original" || current.Tracks[0].URL != "https://example.com/old.mp4" {
		t.Fatal("concurrent merges mutated their shared input")
	}
}
