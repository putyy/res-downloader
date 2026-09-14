package download

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	shared "res-downloader/internal/model"
)

type concurrentSchedulerResources struct {
	schedulerResourceFake
	started chan struct{}
	runs    atomic.Int32
}

func (f *concurrentSchedulerResources) EmitDownloadTaskEvent(shared.DownloadTaskRecord) {}

func (f *concurrentSchedulerResources) RunDownloadPlan(ctx context.Context, _ shared.ResourceCandidate, _ shared.DownloadPlan, _ string, _ shared.DownloadExecution) (string, error) {
	f.runs.Add(1)
	f.started <- struct{}{}
	<-ctx.Done()
	return "", ctx.Err()
}

type blockingSchedulerPlans struct {
	schedulerPlanFake
	started chan struct{}
	release chan struct{}
}

func (f *blockingSchedulerPlans) CreateDownloadPlan(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.DownloadPlan, error) {
	if resource.ID == "slow" {
		f.started <- struct{}{}
		select {
		case <-f.release:
		case <-ctx.Done():
			return shared.DownloadPlan{}, ctx.Err()
		}
	}
	return f.schedulerPlanFake.CreateDownloadPlan(ctx, resource, options)
}

func awaitSchedulerSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("scheduler operation did not finish")
	}
}

func TestSchedulerClaimsDuplicateQueueEntriesOnce(t *testing.T) {
	const workers = 16
	resources := &concurrentSchedulerResources{started: make(chan struct{}, workers)}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	task, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "video"})
	if err != nil {
		t.Fatal(err)
	}
	// Pausing and resuming a pending task leaves an old queue entry behind.
	if _, err := scheduler.Pause(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.queue) != 2 {
		t.Fatalf("queue entries = %d, want 2", len(scheduler.queue))
	}

	var wait sync.WaitGroup
	ready := make(chan struct{}, workers)
	// Make competing workers contend for the same pending snapshot.
	scheduler.mu.Lock()
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ready <- struct{}{}
			scheduler.execute(task.ID)
		}()
	}
	for range workers {
		<-ready
	}
	scheduler.mu.Unlock()
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	awaitSchedulerSignal(t, resources.started)
	if err := scheduler.CancelTask(task.ID); err != nil {
		t.Fatal(err)
	}
	awaitSchedulerSignal(t, done)
	if runs := resources.runs.Load(); runs != 1 {
		t.Fatalf("download executions = %d, want 1", runs)
	}
	if state := scheduler.taskState(task.ID); state != shared.DownloadTaskCancelled {
		t.Fatalf("state = %q", state)
	}
	if len(scheduler.cancelFuncs) != 0 {
		t.Fatal("completed execution retained its cancel function")
	}
}

func TestSchedulerDoesNotClaimPausedOrCancelledPendingTask(t *testing.T) {
	for _, action := range []string{"pause", "cancel"} {
		t.Run(action, func(t *testing.T) {
			resources := &schedulerResourceFake{}
			scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
			defer scheduler.cancel()
			task, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "video"})
			if err != nil {
				t.Fatal(err)
			}
			wantState := shared.DownloadTaskCancelled
			if action == "pause" {
				_, err = scheduler.Pause(task.ID)
				wantState = shared.DownloadTaskPaused
			} else {
				err = scheduler.CancelTask(task.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			scheduler.execute(<-scheduler.queue)
			if len(resources.runs) != 0 || scheduler.taskState(task.ID) != wantState {
				t.Fatalf("task was claimed after %s", action)
			}
		})
	}
}

func TestSchedulerResumePreservesAnotherTasksResourceOwnership(t *testing.T) {
	scheduler := newSchedulerForTest(&schedulerResourceFake{}, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	resource := shared.ResourceCandidate{ID: "video"}
	old, err := scheduler.Enqueue(resource)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.update(&old, shared.DownloadTaskInterrupted, "", "interrupted")
	current, err := scheduler.Enqueue(resource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.Resume(old.ID); err == nil {
		t.Fatal("resumed an interrupted task while its resource was already owned")
	}
	if err := scheduler.CancelTask(old.ID); err == nil {
		t.Fatal("cancelled an interrupted task through another task's ownership")
	}
	// A late terminal update for the old task must not remove the new owner.
	scheduler.update(&old, shared.DownloadTaskFailed, "", "late failure")
	scheduler.Progress(resource.ID, 25, 100)
	if scheduler.byResource[resource.ID] != current.ID || scheduler.tasks[current.ID].Downloaded != 25 {
		t.Fatal("old task displaced the current owner or its progress")
	}
	if scheduler.taskState(current.ID) != shared.DownloadTaskPending {
		t.Fatal("cancelling the old task changed the current task")
	}
	if err := scheduler.CancelTask(current.ID); err != nil {
		t.Fatal(err)
	}
	// Once the resource is free, an interrupted task can resume normally.
	scheduler.update(&old, shared.DownloadTaskInterrupted, "", "interrupted")
	if _, err := scheduler.Resume(old.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSchedulerSlowPlanDoesNotBlockControlsAndConcurrentEnqueueDeduplicates(t *testing.T) {
	plans := &blockingSchedulerPlans{started: make(chan struct{}, 2), release: make(chan struct{})}
	resources := &concurrentSchedulerResources{}
	scheduler := newSchedulerForTest(resources, plans, t.TempDir())
	defer scheduler.cancel()
	existing, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "existing"})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		task shared.DownloadTaskRecord
		err  error
	}
	results := make(chan result, 2)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(plans.release) }) }
	defer release()
	for range 2 {
		go func() {
			task, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "slow"})
			results <- result{task, err}
		}()
	}
	for range 2 {
		awaitSchedulerSignal(t, plans.started)
	}
	controls := make(chan error, 1)
	go func() {
		if len(scheduler.List()) != 1 {
			controls <- errors.New("unresolved task was published too early")
			return
		}
		if _, err := scheduler.Pause(existing.ID); err != nil {
			controls <- err
			return
		}
		if err := scheduler.CancelTask(existing.ID); err != nil {
			controls <- err
			return
		}
		_, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "unrelated"})
		controls <- err
	}()
	select {
	case err := <-controls:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("slow plugin blocked task controls or unrelated enqueue")
	}
	release()
	var id string
	for range 2 {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if id != "" && result.task.ID != id {
				t.Fatal("concurrent enqueue created duplicate tasks")
			}
			id = result.task.ID
		case <-time.After(3 * time.Second):
			t.Fatal("enqueue did not finish after plugin resolved")
		}
	}
	if tasks := scheduler.List(); len(tasks) != 3 || len(scheduler.queue) != 3 {
		t.Fatalf("tasks = %d, queue entries = %d; want 3 each", len(tasks), len(scheduler.queue))
	}
}
