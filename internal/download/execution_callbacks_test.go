package download

import (
	"context"
	"testing"

	shared "res-downloader/internal/model"
)

func TestExecutionCallbacksIgnoreOldExecutionAfterResume(t *testing.T) {
	resources := &schedulerResourceFake{}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	task, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "resource"})
	if err != nil {
		t.Fatal(err)
	}
	scheduler.update(&task, shared.DownloadTaskDownloading, "download", "")
	oldContext, cancel := context.WithCancel(scheduler.ctx)
	oldCallbacks := scheduler.executionCallbacks(oldContext, task)
	oldCallbacks.Progress(25, 100)
	cancel()
	// A resumed execution reuses the task ID, but not the old context.
	scheduler.update(&task, shared.DownloadTaskPaused, "", "")
	scheduler.update(&task, shared.DownloadTaskDownloading, "download", "")
	currentCallbacks := scheduler.executionCallbacks(scheduler.ctx, task)
	currentCallbacks.Progress(50, 100)
	events := len(resources.events)
	oldCallbacks.Progress(90, 100)
	oldCallbacks.Processing()
	current := scheduler.tasks[task.ID]
	if current.Downloaded != 50 || current.Total != 100 || current.State != shared.DownloadTaskDownloading || len(resources.events) != events {
		t.Fatalf("old execution changed resumed task: %#v", current)
	}
	currentCallbacks.Processing()
	if scheduler.taskState(task.ID) != shared.DownloadTaskProcessing {
		t.Fatal("current execution could not enter processing")
	}
}

func TestExecutionCallbacksDoNotFollowNewResourceOwner(t *testing.T) {
	resources := &schedulerResourceFake{}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	resource := shared.ResourceCandidate{ID: "resource"}
	old, err := scheduler.Enqueue(resource)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.update(&old, shared.DownloadTaskDownloading, "download", "")
	callbacks := scheduler.executionCallbacks(scheduler.ctx, old)
	scheduler.update(&old, shared.DownloadTaskCompleted, "", "")
	current, err := scheduler.Enqueue(resource)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.update(&current, shared.DownloadTaskDownloading, "download", "")
	events := len(resources.events)
	callbacks.Progress(90, 100)
	callbacks.Processing()
	if scheduler.tasks[current.ID].Downloaded != 0 || scheduler.taskState(current.ID) != shared.DownloadTaskDownloading || len(resources.events) != events {
		t.Fatal("old task callback changed the new resource owner")
	}
}

func TestCollectionCallbacksKeepChildCountProgress(t *testing.T) {
	child := shared.ResourceCandidate{ID: "child", State: shared.ResourceStateReady, Capabilities: []string{shared.ResourceCapabilityDownload}}
	resources := &progressSchedulerResources{schedulerResourceFake: schedulerResourceFake{children: []shared.ResourceCandidate{child}}}
	scheduler := newSchedulerForTest(resources, schedulerPlanFake{}, t.TempDir())
	defer scheduler.cancel()
	independent, err := scheduler.Enqueue(child)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.update(&independent, shared.DownloadTaskDownloading, "download", "")
	collection, err := scheduler.Enqueue(shared.ResourceCandidate{ID: "collection", Kind: shared.ResourceKindCollection})
	if err != nil {
		t.Fatal(err)
	}
	scheduler.execute(collection.ID)
	current := scheduler.tasks[collection.ID]
	if current.State != shared.DownloadTaskCompleted || current.Downloaded != 1 || current.Total != 1 {
		t.Fatalf("collection lost child-count progress: %#v", current)
	}
	if scheduler.tasks[independent.ID].Downloaded != 0 || scheduler.taskState(independent.ID) != shared.DownloadTaskDownloading {
		t.Fatal("collection execution updated the child's independent task")
	}
	for _, event := range resources.events {
		if event.ID == collection.ID && event.Total > 1 {
			t.Fatalf("child bytes escaped into collection progress: %#v", event)
		}
	}
}

type progressSchedulerResources struct{ schedulerResourceFake }

func (r *progressSchedulerResources) RunDownloadPlan(ctx context.Context, candidate shared.ResourceCandidate, plan shared.DownloadPlan, directory string, execution shared.DownloadExecution, callbacks ExecutionCallbacks) (string, error) {
	if callbacks.Progress != nil {
		callbacks.Progress(1024, 2048)
	}
	if callbacks.Processing != nil {
		callbacks.Processing()
	}
	return r.schedulerResourceFake.RunDownloadPlan(ctx, candidate, plan, directory, execution, callbacks)
}
