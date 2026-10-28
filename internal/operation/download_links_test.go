package operation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
)

func downloadLinkFixture(t *testing.T) (*Service, m.OperationExecution, m.DownloadTaskRecord) {
	t.Helper()
	s, backend := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	backend.finalize = func(_ context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		return data, []string{"resource"}, nil, nil, nil
	}
	execution := startExecution(t, s, "link-key")
	succeed(t, s, execution.ExecutionID)
	execution, err := s.Get(execution.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	task := m.DownloadTaskRecord{ID: "task", ResourceID: "resource", PluginID: "test.plugin", Resource: m.ResourceCandidate{ID: "resource", Source: m.ResourceSource{PluginID: "test.plugin"}, Tracks: []m.ResourceTrack{{ID: "text", MIME: "text/plain"}}}}
	return s, execution, task
}

func TestManualDownloadAssociationIsDurableAndIdempotent(t *testing.T) {
	s, before, task := downloadLinkFixture(t)
	for range 2 {
		if err := s.LinkDownload(task); err != nil {
			t.Fatal(err)
		}
	}
	after, err := s.Get(before.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.DownloadTaskIDs) != 1 || after.DownloadTaskIDs[0] != task.ID || len(after.ArtifactIDs) != 1 {
		t.Fatalf("missing or duplicate links: %#v", after)
	}
	if after.UpdatedAt != before.UpdatedAt || after.ResultExpiresAt != before.ResultExpiresAt {
		t.Fatal("linking extended execution retention")
	}
	a := s.artifacts[after.ArtifactIDs[0]]
	if a.ResourceID != task.ResourceID || a.DownloadTaskID != task.ID || a.PluginID != task.PluginID || a.ExecutionID != after.ExecutionID || a.ExpiresAt != before.ResultExpiresAt || a.MIME != "text/plain" {
		t.Fatalf("incorrect artifact ownership: %#v", a)
	}
	if err := s.db.View(func(tx *bolt.Tx) error {
		var persisted record
		if err := json.Unmarshal(tx.Bucket([]byte("executions")).Get([]byte(after.ExecutionID)), &persisted); err != nil {
			return err
		}
		if len(persisted.View.DownloadTaskIDs) != 1 || len(persisted.View.ArtifactIDs) != 1 || tx.Bucket([]byte("artifacts")).Get([]byte(a.ArtifactID)) == nil {
			t.Error("links were not durably committed together")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestManualDownloadDoesNotReviveCleanedOrExpiredArtifacts(t *testing.T) {
	for _, mode := range []string{"cleaned", "expired", "history", "never_persisted"} {
		t.Run(mode, func(t *testing.T) {
			s, before, task := downloadLinkFixture(t)
			switch mode {
			case "cleaned":
				if err := s.Clean(false, true); err != nil {
					t.Fatal(err)
				}
			case "expired":
				s.records[before.ExecutionID].View.ResultExpiresAt = time.Now().Add(-time.Second).UnixMilli()
				s.nextCleanup = 0 // Simulate reaching the previously scheduled expiry.
			case "history":
				if err := s.Clean(true, true); err != nil {
					t.Fatal(err)
				}
			case "never_persisted":
				s.records[before.ExecutionID].View.ResultStatus = "never_persisted"
				s.records[before.ExecutionID].View.Result = nil
			}
			if err := s.LinkDownload(task); err != nil {
				t.Fatal(err)
			}
			if len(s.artifacts) != 0 {
				t.Fatal("manual download revived a discarded artifact")
			}
			if mode == "history" {
				if len(s.records) != 0 {
					t.Fatal("manual download revived deleted history")
				}
			} else if len(s.records[before.ExecutionID].View.DownloadTaskIDs) != 1 {
				t.Fatal("retained summary did not record real download task")
			}
		})
	}
}

func TestManualDownloadAssociationRejectsForeignOrFabricatedOwnership(t *testing.T) {
	s, before, task := downloadLinkFixture(t)
	task.Resource.ID = "different"
	if err := s.LinkDownload(task); err == nil {
		t.Fatal("accepted mismatched task resource")
	}
	task.Resource.ID = task.ResourceID
	task.PluginID, task.Resource.Source.PluginID = "foreign.plugin", "foreign.plugin"
	if err := s.LinkDownload(task); err != nil {
		t.Fatal(err)
	}
	if len(s.records[before.ExecutionID].View.DownloadTaskIDs) != 0 || len(s.artifacts) != 0 {
		t.Fatal("linked a foreign plugin's task")
	}
}

func TestManualDownloadAssociationWriteFailureLeavesMemoryUntouched(t *testing.T) {
	s, before, task := downloadLinkFixture(t)
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkDownload(task); err == nil {
		t.Fatal("association write failure was hidden")
	}
	if got := s.records[before.ExecutionID].View; len(got.DownloadTaskIDs) != 0 || len(got.ArtifactIDs) != 0 || len(s.artifacts) != 0 {
		t.Fatal("failed transaction leaked in-memory associations")
	}
}

func TestManualDownloadAssociationHonorsMemoryOnlyResultPolicy(t *testing.T) {
	s, before, task := downloadLinkFixture(t)
	s.records[before.ExecutionID].Definition.PersistResult = false
	if err := s.LinkDownload(task); err != nil {
		t.Fatal(err)
	}
	if len(s.records[before.ExecutionID].View.ArtifactIDs) != 1 {
		t.Fatal("live memory-only result lost artifact association")
	}
	if err := s.db.View(func(tx *bolt.Tx) error {
		var stored record
		if err := json.Unmarshal(tx.Bucket([]byte("executions")).Get([]byte(before.ExecutionID)), &stored); err != nil {
			return err
		}
		if len(stored.View.ArtifactIDs) != 0 || tx.Bucket([]byte("artifacts")).Stats().KeyN != 0 {
			t.Error("memory-only artifact association was persisted")
		}
		if len(stored.View.DownloadTaskIDs) != 1 {
			t.Error("durable execution summary lost download task ID")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizerReconcilesManualDownloadCreatedBeforeExecutionFinishes(t *testing.T) {
	s, backend := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	task := m.DownloadTaskRecord{ID: "early", ResourceID: "resource", PluginID: "test.plugin", Resource: m.ResourceCandidate{ID: "resource", Source: m.ResourceSource{PluginID: "test.plugin"}}}
	backend.finalize = func(_ context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		// The resource event can trigger create_download before Finalize returns.
		if err := s.LinkDownload(task); err != nil {
			t.Error(err)
		}
		return data, []string{"resource"}, nil, nil, nil
	}
	backend.tasks = func() []m.DownloadTaskRecord {
		if !s.mu.TryLock() {
			t.Error("download snapshot invoked under operation lock")
			return nil
		}
		s.mu.Unlock()
		return []m.DownloadTaskRecord{task}
	}
	v := startExecution(t, s, "early-download")
	succeed(t, s, v.ExecutionID)
	linked, err := s.Get(v.ExecutionID)
	if err != nil || len(linked.DownloadTaskIDs) != 1 || linked.DownloadTaskIDs[0] != task.ID || len(linked.ArtifactIDs) != 1 {
		t.Fatalf("finalization race lost association: %#v %v", linked, err)
	}
	// An overlapping HTTP completion after the snapshot is harmless.
	if err := s.LinkDownload(task); err != nil {
		t.Fatal(err)
	}
	if len(s.records[v.ExecutionID].View.ArtifactIDs) != 1 {
		t.Fatal("snapshot and HTTP completion duplicated the artifact")
	}
}

func TestFinalizerReconciliationCannotReviveConcurrentCleanup(t *testing.T) {
	for _, history := range []bool{false, true} {
		t.Run(map[bool]string{false: "results", true: "history"}[history], func(t *testing.T) {
			s, backend := testService(t, filepath.Join(t.TempDir(), "operations.db"))
			backend.finalize = func(_ context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
				return data, []string{"resource"}, nil, nil, nil
			}
			backend.tasks = func() []m.DownloadTaskRecord {
				if err := s.Clean(history, true); err != nil {
					t.Error(err)
				}
				return []m.DownloadTaskRecord{{ID: "task", ResourceID: "resource", PluginID: "test.plugin", Resource: m.ResourceCandidate{ID: "resource", Source: m.ResourceSource{PluginID: "test.plugin"}}}}
			}
			v := startExecution(t, s, "cleanup-race")
			succeed(t, s, v.ExecutionID)
			if len(s.artifacts) != 0 {
				t.Fatal("reconciliation revived cleaned artifacts")
			}
			if history && len(s.records) != 0 {
				t.Fatal("reconciliation revived deleted history")
			}
			if !history && s.records[v.ExecutionID].View.ResultStatus != "cleaned" {
				t.Fatal("reconciliation reset cleanup status")
			}
		})
	}
}

func TestFinalizerReconciliationCompletesDuringCloseAndKeepsLifetime(t *testing.T) {
	s, backend := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	backend.finalize = func(_ context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		return data, []string{"resource"}, nil, nil, nil
	}
	snapshotStarted, release := make(chan struct{}), make(chan struct{})
	backend.tasks = func() []m.DownloadTaskRecord {
		s.mu.Lock()
		s.closing = true // Close has stopped new callers but must wait for this finalizer.
		s.mu.Unlock()
		close(snapshotStarted)
		<-release
		return []m.DownloadTaskRecord{{ID: "task", ResourceID: "resource", PluginID: "test.plugin", Resource: m.ResourceCandidate{ID: "resource", Source: m.ResourceSource{PluginID: "test.plugin"}}}}
	}
	v := startExecution(t, s, "closing-race")
	reported := make(chan error, 1)
	go func() {
		reported <- s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "succeeded", Data: json.RawMessage(`{"title":"public"}`)})
	}()
	select {
	case <-snapshotStarted:
	case <-time.After(time.Second):
		t.Fatal("snapshot did not start")
	}
	waited := make(chan struct{})
	go func() { s.finalizers.Wait(); close(waited) }()
	select {
	case <-waited:
		close(release)
		t.Fatal("finalizer lifetime ended before reconciliation")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-reported; err != nil {
		t.Fatal(err)
	}
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("finalizer lifetime was not released")
	}
	if len(s.records[v.ExecutionID].View.DownloadTaskIDs) != 1 || len(s.artifacts) != 1 {
		t.Fatal("closing discarded an in-flight finalizer's associations")
	}
}

func TestFinalizerReconciliationSurfacesStorageFailure(t *testing.T) {
	s, backend := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	backend.finalize = func(_ context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		return data, []string{"resource"}, nil, nil, nil
	}
	backend.tasks = func() []m.DownloadTaskRecord {
		if err := s.db.Close(); err != nil {
			t.Error(err)
		}
		return []m.DownloadTaskRecord{{ID: "task", ResourceID: "resource", PluginID: "test.plugin", Resource: m.ResourceCandidate{ID: "resource", Source: m.ResourceSource{PluginID: "test.plugin"}}}}
	}
	v := startExecution(t, s, "write-race")
	err := s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "succeeded", Data: json.RawMessage(`{"title":"public"}`)})
	if err == nil || s.fault == nil {
		t.Fatal("deferred association storage failure was hidden")
	}
	if _, err := s.Get(v.ExecutionID); err == nil {
		t.Fatal("storage failure was not visible to queries")
	}
	if len(s.records[v.ExecutionID].View.DownloadTaskIDs) != 0 || len(s.artifacts) != 0 {
		t.Fatal("failed reconciliation leaked partial associations")
	}
}
