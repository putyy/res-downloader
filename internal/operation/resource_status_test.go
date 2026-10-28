package operation

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	m "res-downloader/internal/model"
	"testing"
	"time"
)

func TestResourceExecutionsKeepActiveWorkOutsideRecentHistory(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	now := time.Now().UnixMilli()
	for i := 0; i < 150; i++ {
		id := fmt.Sprintf("execution-%03d", i)
		s.records[id] = &record{View: m.OperationExecution{
			ExecutionID: id, ResourceID: "resource", State: "succeeded", CreatedAt: now + int64(i),
			Result: json.RawMessage(`{"data":"large result is not a row status"}`),
		}}
	}
	active := s.records["execution-000"]
	active.View.State = "running"
	active.View.ResourceIDs = []string{"published"}
	page, err := s.List("", "", 0, 100)
	if err != nil || len(page["items"].([]m.OperationExecution)) != 100 {
		t.Fatal("history fixture was not paginated", err)
	}
	rows, err := s.ResourceExecutions([]string{"resource", "missing", "resource"})
	if err != nil || len(rows) != 1 || rows[0].ExecutionID != active.View.ExecutionID || rows[0].Result != nil {
		t.Fatalf("active resource status lost: %#v %v", rows, err)
	}
	rows[0].ResourceIDs[0] = "changed"
	if active.View.ResourceIDs[0] != "published" {
		t.Fatal("resource status exposed mutable service state")
	}
	active.View.State = "succeeded"
	if err := s.save([]*record{active}); err != nil {
		t.Fatal(err)
	}
	rows, err = s.ResourceExecutions([]string{"resource"})
	if err != nil || len(rows) != 1 || rows[0].ExecutionID != "execution-149" {
		t.Fatal("completed resource did not select its newest execution", err)
	}
	if err := s.Clean(true, true); err != nil {
		t.Fatal(err)
	}
	if rows, err = s.ResourceExecutions([]string{"resource"}); err != nil || len(rows) != 0 {
		t.Fatal("cleaned history still supplies resource status", err)
	}
}

func TestResourceExecutionsRejectOversizedQueries(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	for _, ids := range [][]string{{""}, make([]string, MaxResourceStatusIDs+1)} {
		if _, err := s.ResourceExecutions(ids); err == nil {
			t.Fatal("invalid resource query accepted")
		}
	}
}

func TestCleanupCacheInvalidatesOnWritesAndRetentionChanges(t *testing.T) {
	s, _ := testService(t, filepath.Join(t.TempDir(), "operations.db"))
	v := startExecution(t, s, "cache")
	succeed(t, s, v.ExecutionID)
	r := s.records[v.ExecutionID]
	r.View.ResultExpiresAt = time.Now().Add(5 * time.Second).UnixMilli()
	if err := s.save([]*record{r}); err != nil {
		t.Fatal(err)
	}
	if s.nextCleanup != 0 {
		t.Fatal("write did not invalidate the retention scan")
	}
	if _, err := s.Get(v.ExecutionID); err != nil {
		t.Fatal(err)
	}
	if s.nextCleanup <= time.Now().UnixMilli() || s.nextCleanup > r.View.ResultExpiresAt {
		t.Fatal("cached scan outlives the next result expiry")
	}
	r.View.UpdatedAt = time.Now().Add(-2 * time.Hour).UnixMilli()
	if err := s.SetSettings(Settings{HistoryDays: 30, ResultHours: 1, Automation: map[string]bool{}}); err != nil {
		t.Fatal(err)
	}
	result, err := s.Get(v.ExecutionID)
	if err != nil || result.ResultStatus != "expired" || len(result.Result) != 0 {
		t.Fatal("retention change reused a stale scan", err)
	}
}
