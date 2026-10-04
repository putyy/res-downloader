package operation

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
)

type testBackend struct {
	pages    []m.OperationSession
	finalize func(context.Context, m.OperationExecution, m.OperationDefinition, json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error)
	text     string
	tasks    func() []m.DownloadTaskRecord
}

func (b *testBackend) OperationDownloads() []m.DownloadTaskRecord {
	if b.tasks != nil {
		return b.tasks()
	}
	return nil
}

func (b *testBackend) Operations() []m.OperationInfo {
	return []m.OperationInfo{{PluginID: "test.plugin", PluginVersion: "1", OperationID: "read", Definition: testDefinition()}}
}
func (b *testBackend) Sessions() []m.OperationSession                              { return clone(b.pages) }
func (b *testBackend) Send(m.OperationExecution, m.OperationRequest, string) error { return nil }
func (b *testBackend) Finalize(ctx context.Context, e m.OperationExecution, d m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
	if b.finalize != nil {
		return b.finalize(ctx, e, d, data)
	}
	return data, nil, nil, nil, nil
}

type testReader struct{ *strings.Reader }

func (testReader) Close() error { return nil }
func (b *testBackend) OpenArtifact(a m.OperationArtifact) (io.ReadSeekCloser, m.OperationArtifact, error) {
	a.Size = int64(len(b.text))
	a.MIME = "text/plain"
	a.Status = "available"
	return testReader{strings.NewReader(b.text)}, a, nil
}
func testDefinition() m.OperationDefinition {
	return m.OperationDefinition{Name: "Read", Category: "text", PageScript: "page", Effects: []string{"read"}, SafeRetry: true, Cancellable: true, Automation: true, PersistResult: true, InputSchema: map[string]interface{}{"type": "object", "additionalProperties": false, "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string", "x-sensitive": true}}}, OutputSchema: map[string]interface{}{"type": "object", "additionalProperties": false, "properties": map[string]interface{}{"title": map[string]interface{}{"type": "string", "x-persist": true}, "secret": map[string]interface{}{"type": "string", "x-sensitive": true}}}}
}

// Construct a deterministic scheduler without background goroutines; tests drive ticks.
func testService(t *testing.T, path string) (*Service, *testBackend) {
	t.Helper()
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"executions", "settings", "artifacts", "idempotency"} {
			if _, e := tx.CreateBucketIfNotExists([]byte(name)); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	backend := &testBackend{pages: []m.OperationSession{{PluginID: "test.plugin", ScriptID: "page", PageSessionID: "page1", Connected: true, Ready: true, Login: "unknown", Revision: "revision1", Operations: []string{"read"}}}}
	s := &Service{db: db, backend: backend, records: map[string]*record{}, keys: map[string]keyRecord{}, artifacts: map[string]m.OperationArtifact{}, settings: Settings{7, 2, map[string]bool{}}}
	if err = s.load(); err != nil {
		t.Fatal(err)
	}
	return s, backend
}
func testRequest(key string) m.OperationRequest {
	return m.OperationRequest{PluginID: "test.plugin", OperationID: "read", Input: map[string]interface{}{"query": "private search"}, IdempotencyKey: key}
}
func startExecution(t *testing.T, s *Service, key string) m.OperationExecution {
	t.Helper()
	v, err := s.Submit(testRequest(key), "desktop")
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.tickLocked()
	s.mu.Unlock()
	if err = s.Claim(v.ExecutionID, "page1", "revision1"); err != nil {
		t.Fatal(err)
	}
	return v
}
func succeed(t *testing.T, s *Service, id string) {
	t.Helper()
	if err := s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: id, State: "succeeded", Data: json.RawMessage(`{"title":"public","secret":"never write this"}`)}); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryCleanupPreservesIdempotencyAndNoSecrets(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	v := startExecution(t, s, "once")
	succeed(t, s, v.ExecutionID)
	if err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("executions")).ForEach(func(_, value []byte) error {
			if strings.Contains(string(value), "private search") || strings.Contains(string(value), "never write this") {
				t.Fatal("sensitive value persisted")
			}
			return nil
		})
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Clean(true, true); err != nil {
		t.Fatal(err)
	}
	again, err := s.Submit(testRequest("once"), "desktop")
	if err != nil || again.ExecutionID != v.ExecutionID || again.ResultStatus != "cleaned" {
		t.Fatalf("duplicate recreated execution: %#v %v", again, err)
	}
	changed := testRequest("once")
	changed.Input["query"] = "different"
	if _, err = s.Submit(changed, "desktop"); err == nil {
		t.Fatal("conflicting key accepted")
	}
	if _, err = s.Get(v.ExecutionID); err != nil {
		t.Fatal("tombstone must remain queryable", err)
	}
}
func TestRestartInterruptsAndRestoresProjectedResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.db")
	s, _ := testService(t, path)
	done := startExecution(t, s, "done")
	succeed(t, s, done.ExecutionID)
	active := startExecution(t, s, "active")
	if err := s.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, _ := testService(t, path)
	v, err := reopened.Get(active.ExecutionID)
	if err != nil || v.State != "interrupted" {
		t.Fatalf("restarted execution %#v %v", v, err)
	}
	v, err = reopened.Get(done.ExecutionID)
	if err != nil || !v.ResultProjection || strings.Contains(string(v.Result), "never write this") {
		t.Fatalf("bad projection %#v %v", v, err)
	}
	if len(v.Result) == 0 {
		t.Fatal("allowlisted result lost")
	}
}
func TestBatchIsAtomicOrderedAndKeyBindsWholeList(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	bad := testRequest("")
	bad.OperationID = "missing"
	if _, err := s.SubmitBatch([]m.OperationRequest{testRequest(""), bad}, "batch", "desktop"); err == nil || len(s.records) != 0 {
		t.Fatal("invalid batch partially admitted")
	}
	batch, err := s.SubmitBatch([]m.OperationRequest{testRequest(""), testRequest("")}, "batch", "desktop")
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range batch.Items {
		if v.Index != i {
			t.Fatal("lost input order")
		}
	}
	if _, err = s.SubmitBatch([]m.OperationRequest{testRequest("")}, "batch", "desktop"); err == nil {
		t.Fatal("changed batch length accepted")
	}
	if err = s.CancelBatch(batch.BatchID); err != nil {
		t.Fatal(err)
	}
	if err = s.Clean(true, false); err != nil {
		t.Fatal(err)
	}
	again, err := s.SubmitBatch([]m.OperationRequest{testRequest(""), testRequest("")}, "batch", "desktop")
	if err != nil || again.BatchID != batch.BatchID || len(again.Items) != 2 {
		t.Fatalf("batch tombstones lost %#v %v", again, err)
	}
}
func TestPageOwnershipDeadlinesAndLateReports(t *testing.T) {
	s, b := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	b.pages = append(b.pages, m.OperationSession{PluginID: "test.plugin", ScriptID: "page", PageSessionID: "page2", Connected: true, Ready: true, Revision: "revision2", Operations: []string{"read"}})
	if _, err := s.Submit(testRequest(""), "desktop"); err == nil {
		t.Fatal("ambiguous page auto-selected")
	}
	b.pages = b.pages[:1]
	v := startExecution(t, s, "")
	if err := s.Report(context.Background(), "foreign", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "failed"}); err == nil {
		t.Fatal("foreign page reported")
	}
	s.records[v.ExecutionID].View.Deadline = time.Now().Add(-time.Second).UnixMilli()
	_ = s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "succeeded", Data: json.RawMessage(`{}`)})
	if s.records[v.ExecutionID].View.State != "timed_out" {
		t.Fatal("deadline not enforced at report boundary")
	}
}
func TestFinalizingDoesNotBlockCancelAndKeepsTerminal(t *testing.T) {
	s, b := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	entered := make(chan struct{})
	b.finalize = func(ctx context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		close(entered)
		<-ctx.Done()
		return data, []string{"published-before-cancel"}, nil, nil, ctx.Err()
	}
	v := startExecution(t, s, "")
	finished := make(chan error, 1)
	go func() {
		finished <- s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: v.ExecutionID, State: "succeeded", Data: json.RawMessage(`{}`)})
	}()
	<-entered
	cancelled := make(chan error, 1)
	go func() { cancelled <- s.Cancel(v.ExecutionID) }()
	select {
	case err := <-cancelled:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel blocked on finalizer")
	}
	<-finished
	result, err := s.Get(v.ExecutionID)
	if err != nil || result.State != "cancelled" || len(result.ResourceIDs) != 1 {
		t.Fatalf("late effects lost or state overwritten %#v %v", result, err)
	}
}
func TestResultCleanupRevokesArtifactsAndUTF8Offsets(t *testing.T) {
	s, b := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	b.text = "ab你好cd"
	b.finalize = func(_ context.Context, _ m.OperationExecution, _ m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		return data, nil, nil, []m.OperationArtifact{{ArtifactID: "artifact1", MIME: "text/plain"}}, nil
	}
	v := startExecution(t, s, "")
	succeed(t, s, v.ExecutionID)
	part, err := s.ReadText("artifact1", 0, 4)
	if err != nil || part["text"] != "ab" || part["nextOffset"] != int64(2) {
		t.Fatalf("UTF-8 split %#v %v", part, err)
	}
	if _, err = s.ReadText("artifact1", 3, 4); err == nil {
		t.Fatal("mid-rune offset accepted")
	}
	if err = s.Clean(false, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadText("artifact1", 0, 4); err == nil {
		t.Fatal("cleaned artifact still readable")
	}
}
func TestOldHistoryCannotOverwriteReusedKey(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	first := startExecution(t, s, "key")
	succeed(t, s, first.ExecutionID)
	old := s.records[first.ExecutionID]
	old.KeyExpires = time.Now().Add(-time.Second).UnixMilli()
	prior := s.keys[old.Key]
	prior.Expires = old.KeyExpires
	s.keys[old.Key] = prior
	s.nextCleanup = 0 // The test advanced stored deadlines outside the normal save path.
	next := startExecution(t, s, "key")
	succeed(t, s, next.ExecutionID)
	if err := s.Clean(true, true); err != nil {
		t.Fatal(err)
	}
	again, err := s.Submit(testRequest("key"), "desktop")
	if err != nil || again.ExecutionID != next.ExecutionID {
		t.Fatalf("old cleanup replaced current key %#v %v", again, err)
	}
}

func TestTerminalExecutorKeepsLeaseUntilStopAcknowledgement(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	first := startExecution(t, s, "")
	queued, err := s.Submit(testRequest(""), "desktop")
	if err != nil {
		t.Fatal(err)
	}
	s.records[first.ExecutionID].View.Deadline = time.Now().Add(-time.Second).UnixMilli()
	s.mu.Lock()
	s.tickLocked()
	s.mu.Unlock()
	if s.records[first.ExecutionID].View.State != "timed_out" || s.records[queued.ExecutionID].View.State != "queued" {
		t.Fatal("timeout released a still-running page executor")
	}
	if _, err := s.Submit(testRequest(""), "desktop"); err == nil {
		t.Fatal("unknown page executor allowed another submission")
	}
	if err := s.Report(context.Background(), "page1", "revision1", m.OperationReport{ExecutionID: first.ExecutionID, State: "cancelled"}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.tickLocked()
	s.mu.Unlock()
	if s.records[first.ExecutionID].View.State != "timed_out" || !s.records[first.ExecutionID].View.CancelConfirmed || s.records[queued.ExecutionID].View.State != "running" {
		t.Fatal("stop acknowledgement changed terminal state or failed to release the page")
	}
}
