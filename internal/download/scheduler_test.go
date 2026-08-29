package download

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"res-downloader/internal/config"
	shared "res-downloader/internal/model"
)

type schedulerResourceFake struct {
	children []shared.ResourceCandidate
	runs     []string
	events   []shared.DownloadTaskRecord
}

type pausableSchedulerResourceFake struct {
	started chan struct{}
	once    sync.Once
	mu      sync.Mutex
	runs    int
}

func (f *pausableSchedulerResourceFake) ChildrenOf(shared.ResourceCandidate) []shared.ResourceCandidate {
	return nil
}
func (f *pausableSchedulerResourceFake) RunDownloadPlan(ctx context.Context, candidate shared.ResourceCandidate, _ shared.DownloadPlan, directory string, _ shared.DownloadExecution) (string, error) {
	f.mu.Lock()
	f.runs++
	run := f.runs
	f.mu.Unlock()
	if run == 1 {
		f.once.Do(func() { close(f.started) })
		<-ctx.Done()
		return "", ctx.Err()
	}
	return filepath.Join(directory, candidate.ID+".mp4"), nil
}
func (f *pausableSchedulerResourceFake) CancelActive(string) error                       { return nil }
func (f *pausableSchedulerResourceFake) CancelAllActive()                                {}
func (f *pausableSchedulerResourceFake) EmitDownloadTaskEvent(shared.DownloadTaskRecord) {}
func (f *pausableSchedulerResourceFake) EmitDownloadTaskRemoved(string)                  {}

func waitForTaskState(t *testing.T, scheduler *Scheduler, id, state string) shared.DownloadTaskRecord {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		scheduler.mu.RLock()
		task := scheduler.tasks[id]
		scheduler.mu.RUnlock()
		if task.State == state {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	return shared.DownloadTaskRecord{}
}

func (f *schedulerResourceFake) Candidate(string) (shared.ResourceCandidate, bool) {
	return shared.ResourceCandidate{}, false
}
func (f *schedulerResourceFake) SaveCandidate(shared.ResourceCandidate) error { return nil }
func (f *schedulerResourceFake) ChildrenOf(shared.ResourceCandidate) []shared.ResourceCandidate {
	return append([]shared.ResourceCandidate(nil), f.children...)
}
func (f *schedulerResourceFake) RunDownloadPlan(_ context.Context, candidate shared.ResourceCandidate, _ shared.DownloadPlan, directory string, _ shared.DownloadExecution) (string, error) {
	f.runs = append(f.runs, candidate.ID)
	return filepath.Join(directory, candidate.ID+".mp4"), nil
}
func (f *schedulerResourceFake) CancelActive(string) error { return nil }
func (f *schedulerResourceFake) CancelAllActive()          {}
func (f *schedulerResourceFake) EmitDownloadTaskEvent(task shared.DownloadTaskRecord) {
	f.events = append(f.events, task)
}
func (f *schedulerResourceFake) EmitDownloadTaskRemoved(string) {}

type schedulerPlanFake struct{}

func (schedulerPlanFake) CreateDownloadPlan(_ context.Context, candidate shared.ResourceCandidate, _ shared.DownloadOptions) (shared.DownloadPlan, error) {
	return shared.DownloadPlan{Inputs: []shared.DownloadInput{{ID: candidate.ID, URL: "https://example.test/" + candidate.ID}}}, nil
}

type recordingPlanFake struct{}

func (recordingPlanFake) CreateDownloadPlan(_ context.Context, candidate shared.ResourceCandidate, _ shared.DownloadOptions) (shared.DownloadPlan, error) {
	return shared.DownloadPlan{
		Inputs: []shared.DownloadInput{{ID: candidate.ID, Executor: "ffmpeg-hls", URL: "https://example.test/live.m3u8"}},
		Output: shared.DownloadOutput{Input: candidate.ID, Extension: ".mp4"},
	}, nil
}
func (recordingPlanFake) RefreshResource(_ context.Context, candidate shared.ResourceCandidate, _ shared.DownloadOptions) (shared.ResourceCandidate, string, error) {
	return candidate, shared.ResourceRefreshOK, nil
}

type recordingSchedulerResourceFake struct {
	started chan struct{}
	once    sync.Once
}

func (f *recordingSchedulerResourceFake) ChildrenOf(shared.ResourceCandidate) []shared.ResourceCandidate {
	return nil
}
func (f *recordingSchedulerResourceFake) RunDownloadPlan(ctx context.Context, candidate shared.ResourceCandidate, _ shared.DownloadPlan, directory string, _ shared.DownloadExecution) (string, error) {
	f.once.Do(func() { close(f.started) })
	<-ctx.Done()
	return filepath.Join(directory, candidate.ID+".mp4"), nil
}
func (f *recordingSchedulerResourceFake) CancelActive(string) error                       { return nil }
func (f *recordingSchedulerResourceFake) CancelAllActive()                                {}
func (f *recordingSchedulerResourceFake) EmitDownloadTaskEvent(shared.DownloadTaskRecord) {}
func (f *recordingSchedulerResourceFake) EmitDownloadTaskRemoved(string)                  {}
func (schedulerPlanFake) RefreshResource(_ context.Context, candidate shared.ResourceCandidate, _ shared.DownloadOptions) (shared.ResourceCandidate, string, error) {
	return candidate, shared.ResourceRefreshOK, nil
}

func newSchedulerForTest(resources ResourceService, plans PlanService, directory string) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		config: &config.Config{SaveDirectory: directory}, resources: resources, plugins: plans,
		ctx: ctx, cancel: cancel, queue: make(chan string, 4), tasks: make(map[string]shared.DownloadTaskRecord),
		byResource: make(map[string]string), cancelFuncs: make(map[string]context.CancelCauseFunc),
		taskRoot:     filepath.Join(directory, "download-work"),
		lastProgress: make(map[string]time.Time),
	}
}

func TestCollectionTaskRunsFromSnapshotAfterCaptureCatalogIsCleared(t *testing.T) {
	child := shared.ResourceCandidate{
		ID: "child", Title: "Child", State: shared.ResourceStateReady,
		Capabilities: []string{shared.ResourceCapabilityDownload},
	}
	resources := &schedulerResourceFake{children: []shared.ResourceCandidate{child}}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()

	task, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "parent", Title: "Collection", Kind: shared.ResourceKindCollection})
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Items) != 1 || task.Items[0].Resource.ID != child.ID {
		t.Fatalf("collection snapshot = %#v", task.Items)
	}
	resources.children = nil // Simulates clearing the capture catalog.
	scheduler.execute(task.ID)

	result := scheduler.tasks[task.ID]
	if result.State != shared.DownloadTaskCompleted {
		t.Fatalf("state = %q, error = %q", result.State, result.Error)
	}
	if len(resources.runs) != 1 || resources.runs[0] != child.ID {
		t.Fatalf("downloaded resources = %#v", resources.runs)
	}
}

func TestSchedulerPausesAndResumesHTTPTaskWithoutRetry(t *testing.T) {
	resources := &pausableSchedulerResourceFake{started: make(chan struct{})}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	scheduler.workers = 1
	scheduler.Start()
	defer scheduler.Close()

	task, err := scheduler.Enqueue(shared.ResourceCandidate{
		ID: "video", Title: "Video", State: shared.ResourceStateReady,
		Capabilities: []string{shared.ResourceCapabilityDownload},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-resources.started:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start")
	}
	paused, err := scheduler.Pause(task.ID)
	if err != nil || paused.State != shared.DownloadTaskPausing {
		t.Fatalf("pause: task=%#v err=%v", paused, err)
	}
	paused = waitForTaskState(t, scheduler, task.ID, shared.DownloadTaskPaused)
	if paused.ID == "" {
		t.Fatal("task did not reach paused state")
	}
	if _, err := scheduler.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForTaskState(t, scheduler, task.ID, shared.DownloadTaskCompleted)
	if completed.ID == "" {
		t.Fatal("task did not complete after resume")
	}
	if completed.Attempts != 1 || completed.Resumes != 1 {
		t.Fatalf("attempts=%d resumes=%d", completed.Attempts, completed.Resumes)
	}
	if !completed.Resumable || completed.Error != "" {
		t.Fatalf("unexpected completed task: %#v", completed)
	}
}

func TestStoppingRecordingCompletesWithPreservedOutput(t *testing.T) {
	resources := &recordingSchedulerResourceFake{started: make(chan struct{})}
	scheduler := newSchedulerForTest(resources, recordingPlanFake{}, t.TempDir())
	scheduler.workers = 1
	scheduler.Start()
	defer scheduler.Close()
	task, err := scheduler.Enqueue(shared.ResourceCandidate{
		ID: "live", Title: "Live", State: shared.ResourceStateReady,
		Capabilities: []string{shared.ResourceCapabilityDownload},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !task.Recording || task.Resumable {
		t.Fatalf("recording flags=%#v", task)
	}
	select {
	case <-resources.started:
	case <-time.After(3 * time.Second):
		t.Fatal("recording did not start")
	}
	if err := scheduler.StopRecording(task.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForTaskState(t, scheduler, task.ID, shared.DownloadTaskCompleted)
	if completed.ID == "" || completed.OutputPath == "" {
		t.Fatalf("recording was not completed with output: %#v", completed)
	}
}
