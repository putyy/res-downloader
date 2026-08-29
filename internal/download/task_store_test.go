package download

import (
	"path/filepath"
	"testing"

	shared "res-downloader/internal/model"
)

func TestDownloadTaskStorePersistsPlanWithoutSelectedHeaders(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	task := shared.DownloadTaskRecord{
		ID: "task", ResourceID: "resource", State: shared.DownloadTaskPending,
		Resource: shared.ResourceCandidate{ID: "resource", Tracks: []shared.ResourceTrack{{ID: "video", Headers: map[string]string{"Cookie": "secret", "Referer": "kept"}, NonPersistentHeaders: []string{"cookie"}}}},
		Plan:     shared.DownloadPlan{Inputs: []shared.DownloadInput{{ID: "video", Headers: map[string]string{"Cookie": "secret", "Referer": "kept"}}}},
	}
	if err := store.Upsert(task); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.List()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%#v err=%v", tasks, err)
	}
	if _, exists := tasks[0].Plan.Inputs[0].Headers["Cookie"]; exists {
		t.Fatal("task plan persisted Cookie")
	}
	if tasks[0].Plan.Inputs[0].Headers["Referer"] != "kept" {
		t.Fatal("task plan removed an unmarked header")
	}
	if task.Plan.Inputs[0].Headers["Cookie"] != "secret" {
		t.Fatal("in-memory task was mutated")
	}
}

func TestDownloadTaskStoreScrubsCollectionItemHeaders(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	task := shared.DownloadTaskRecord{
		ID: "collection", ResourceID: "parent", State: shared.DownloadTaskPending,
		Items: []shared.DownloadTaskItem{{
			Resource: shared.ResourceCandidate{ID: "child", Tracks: []shared.ResourceTrack{{
				ID: "video", Headers: map[string]string{"Authorization": "secret", "Referer": "kept"},
				NonPersistentHeaders: []string{"authorization"},
			}}},
			Plan: shared.DownloadPlan{Inputs: []shared.DownloadInput{{
				ID: "video", Headers: map[string]string{"Authorization": "secret", "Referer": "kept"},
			}}},
		}},
	}
	if err := store.Upsert(task); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.List()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%#v err=%v", tasks, err)
	}
	item := tasks[0].Items[0]
	if _, exists := item.Resource.Tracks[0].Headers["Authorization"]; exists {
		t.Fatal("collection resource persisted Authorization")
	}
	if _, exists := item.Plan.Inputs[0].Headers["Authorization"]; exists {
		t.Fatal("collection plan persisted Authorization")
	}
	if item.Plan.Inputs[0].Headers["Referer"] != "kept" {
		t.Fatal("collection plan removed an unmarked header")
	}
}
