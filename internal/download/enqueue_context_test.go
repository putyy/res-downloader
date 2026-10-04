package download

import (
	"context"
	"errors"
	"path/filepath"
	shared "res-downloader/internal/model"
	"testing"
	"time"
)

type contextEnqueuePlans struct {
	schedulerPlanFake
	plan func(context.Context, shared.ResourceCandidate) (shared.DownloadPlan, error)
}

func (p contextEnqueuePlans) CreateDownloadPlan(ctx context.Context, candidate shared.ResourceCandidate, _ shared.DownloadOptions) (shared.DownloadPlan, error) {
	return p.plan(ctx, candidate)
}

type enqueueEventResources struct {
	schedulerResourceFake
	onEvent func(shared.DownloadTaskRecord)
}

func (r *enqueueEventResources) EmitDownloadTaskEvent(task shared.DownloadTaskRecord) {
	if r.onEvent != nil {
		r.onEvent(task)
	}
}

func TestEnqueueContextCancelsPlanningWithoutPublishingTask(t *testing.T) {
	for _, collection := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "collection"}[collection], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			resource := shared.ResourceCandidate{ID: "root"}
			resources := &schedulerResourceFake{}
			if collection {
				resource.Kind = shared.ResourceKindCollection
				resources.children = []shared.ResourceCandidate{{ID: "child", Capabilities: []string{shared.ResourceCapabilityDownload}}}
			}
			calls := 0
			plans := contextEnqueuePlans{plan: func(actual context.Context, _ shared.ResourceCandidate) (shared.DownloadPlan, error) {
				calls++
				if _, ok := actual.Deadline(); !ok {
					t.Error("operation deadline did not reach plan service")
				}
				cancel()
				<-actual.Done()
				return shared.DownloadPlan{}, actual.Err()
			}}
			scheduler := newSchedulerForTest(resources, plans, t.TempDir())
			defer scheduler.cancel()
			task, err := scheduler.EnqueueContext(ctx, resource)
			if !errors.Is(err, context.Canceled) || task.ID != "" || calls != 1 || len(scheduler.List()) != 0 || len(scheduler.queue) != 0 {
				t.Fatalf("cancelled plan published work: task=%v err=%v calls=%d", task, err, calls)
			}
		})
	}
}

func TestEnqueueContextRejectsFullQueueBeforePersistence(t *testing.T) {
	scheduler := newSchedulerForTest(&schedulerResourceFake{}, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	store, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	scheduler.store = store
	for i := 0; i < cap(scheduler.queue); i++ {
		scheduler.queue <- "occupied"
	}
	task, err := scheduler.EnqueueContext(context.Background(), shared.ResourceCandidate{ID: "resource"})
	if err == nil || task.ID != "" || len(scheduler.List()) != 0 {
		t.Fatalf("queue rejection published task: %v %v", task, err)
	}
	if records, err := store.List(); err != nil || len(records) != 0 {
		t.Fatalf("queue rejection left persisted pending task: %v %v", records, err)
	}
}

func TestEnqueueContextRetainsCommittedIDWhenCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resources := &enqueueEventResources{onEvent: func(shared.DownloadTaskRecord) { cancel() }}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	store, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	scheduler.store = store
	task, err := scheduler.EnqueueContext(ctx, shared.ResourceCandidate{ID: "resource"})
	if task.ID == "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("lost committed ID on cancellation: %v %v", task, err)
	}
	if queued := <-scheduler.queue; queued != task.ID {
		t.Fatalf("committed task is not queued: %s", queued)
	}
	if records, err := store.List(); err != nil || len(records) != 1 || records[0].ID != task.ID {
		t.Fatalf("committed task is not recoverable: %v %v", records, err)
	}
	if len(scheduler.enqueueReady) != 0 || scheduler.taskState(task.ID) != shared.DownloadTaskPending {
		t.Fatal("caller cancellation leaked publication gate or cancelled independent task")
	}
}

func TestEnqueueContextReservationCannotExecuteAfterFailedPersistence(t *testing.T) {
	resources := &schedulerResourceFake{}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	store, err := Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	scheduler.store = store
	task, err := scheduler.EnqueueContext(context.Background(), shared.ResourceCandidate{ID: "resource"})
	if err == nil || task.ID != "" || len(scheduler.List()) != 0 {
		t.Fatalf("persistence failure published task: %v %v", task, err)
	}
	if len(scheduler.queue) != 1 {
		t.Fatal("expected an inert reserved queue ID")
	}
	scheduler.execute(<-scheduler.queue)
	if len(resources.runs) != 0 || len(resources.events) != 0 {
		t.Fatal("failed persistence produced an execution or event")
	}
}

func TestEnqueueContextWorkerWaitsForInitialEvent(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	resources := &enqueueEventResources{}
	resources.onEvent = func(task shared.DownloadTaskRecord) {
		if task.State == shared.DownloadTaskPending {
			close(started)
			<-release
		}
	}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	result := make(chan error, 1)
	go func() {
		_, err := scheduler.EnqueueContext(context.Background(), shared.ResourceCandidate{ID: "resource"})
		result <- err
	}()
	awaitSchedulerSignal(t, started)
	id := <-scheduler.queue
	scheduler.mu.RLock()
	gate := scheduler.enqueueReady[id]
	scheduler.mu.RUnlock()
	if gate == nil {
		close(release)
		t.Fatal("missing pending event gate")
	}
	done := make(chan struct{})
	go func() { scheduler.execute(id); close(done) }()
	// Cancel the scheduler while the event is blocked: workers must still exit.
	scheduler.cancel()
	awaitSchedulerSignal(t, done)
	close(release)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("scheduler stop after commit = %v", err)
	}
	if len(resources.runs) != 0 {
		t.Fatal("worker ran before pending event publication completed")
	}
}
