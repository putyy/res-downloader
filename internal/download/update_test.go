package download

import (
	"context"
	shared "res-downloader/internal/model"
	"testing"
)

func TestUpdateReservationPreservesActiveWork(t *testing.T) {
	for _, state := range []string{shared.DownloadTaskPending, shared.DownloadTaskResolving, shared.DownloadTaskDownloading, shared.DownloadTaskProcessing, shared.DownloadTaskPausing} {
		s := &Scheduler{ctx: context.Background(), tasks: map[string]shared.DownloadTaskRecord{"one": {State: state}}}
		if s.ReserveUpdate() {
			t.Errorf("reserved update while task is %s", state)
		}
	}
	s := &Scheduler{ctx: context.Background(), tasks: map[string]shared.DownloadTaskRecord{"one": {ID: "one", ResourceID: "resource", Resumable: true, State: shared.DownloadTaskPaused}}}
	if !s.ReserveUpdate() {
		t.Fatal("paused tasks should allow updates")
	}
	if s.ReserveUpdate() {
		t.Fatal("must not reserve the same restart twice")
	}
	if _, err := s.Resume("one"); err == nil {
		t.Fatal("resumed work during update")
	}
	if _, err := s.Retry("one"); err == nil {
		t.Fatal("retried work during update")
	}
	s.ReleaseUpdate()
	if !s.ReserveUpdate() {
		t.Fatal("failed preparation must release reservation")
	}
}
