package download

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	"sort"
	"strings"
	"sync"
	"time"

	shared "res-downloader/internal/model"
	"res-downloader/internal/naming"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type Scheduler struct {
	config       *config.Config
	resources    ResourceService
	plugins      PlanService
	logger       *logging.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	store        *Store
	queue        chan string
	mu           sync.RWMutex
	tasks        map[string]shared.DownloadTaskRecord
	byResource   map[string]string
	cancelFuncs  map[string]context.CancelCauseFunc
	wg           sync.WaitGroup
	closeOnce    sync.Once
	startOnce    sync.Once
	workers      int
	workerMu     sync.Mutex
	workerStops  []chan struct{}
	started      bool
	lastProgress map[string]time.Time
}

const taskWorkspaceDirectory = ".res-downloader-work"

type ResourceService interface {
	ChildrenOf(shared.ResourceCandidate) []shared.ResourceCandidate
	RunDownloadPlan(context.Context, shared.ResourceCandidate, shared.DownloadPlan, string, shared.DownloadExecution) (string, error)
	CancelActive(string) error
	CancelAllActive()
	EmitDownloadTaskEvent(shared.DownloadTaskRecord)
	EmitDownloadTaskRemoved(string)
}

type PlanService interface {
	CreateDownloadPlan(context.Context, shared.ResourceCandidate, shared.DownloadOptions) (shared.DownloadPlan, error)
	RefreshResource(context.Context, shared.ResourceCandidate, shared.DownloadOptions) (shared.ResourceCandidate, string, error)
}

func NewScheduler(userDir string, config *config.Config, resources ResourceService, plugins PlanService, logger *logging.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	scheduler := &Scheduler{
		config: config, resources: resources, plugins: plugins, logger: logger,
		ctx: ctx, cancel: cancel, queue: make(chan string, 1024),
		tasks: make(map[string]shared.DownloadTaskRecord), byResource: make(map[string]string), cancelFuncs: make(map[string]context.CancelCauseFunc),
		lastProgress: make(map[string]time.Time),
	}
	store, err := Open(filepath.Join(userDir, "tasks.db"))
	if err != nil {
		logger.Esg(err, "open download task database; using memory-only queue")
	} else {
		scheduler.store = store
		scheduler.restore()
	}
	scheduler.workers = config.Snapshot().DownNumber
	if scheduler.workers < 1 {
		scheduler.workers = 1
	}
	return scheduler
}

func (s *Scheduler) Start() {
	s.startOnce.Do(func() {
		s.workerMu.Lock()
		s.started = true
		s.resizeWorkersLocked(s.workers)
		s.workerMu.Unlock()
		s.requeuePending()
	})
}

func (s *Scheduler) SetWorkerCount(count int) {
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}
	s.workerMu.Lock()
	s.workers = count
	if s.started {
		s.resizeWorkersLocked(count)
	}
	s.workerMu.Unlock()
}

func (s *Scheduler) resizeWorkersLocked(count int) {
	for len(s.workerStops) < count {
		stop := make(chan struct{})
		s.workerStops = append(s.workerStops, stop)
		s.wg.Add(1)
		go s.worker(stop)
	}
	for len(s.workerStops) > count {
		last := len(s.workerStops) - 1
		close(s.workerStops[last])
		s.workerStops = s.workerStops[:last]
	}
}

func (s *Scheduler) restore() {
	records, err := s.store.List()
	if err != nil {
		s.logger.Esg(err, "restore download tasks")
		return
	}
	now := time.Now().UnixMilli()
	for _, task := range records {
		if task.State == shared.DownloadTaskPending && taskNeedsRecaptureForHeaders(task) {
			task.State = shared.DownloadTaskInterrupted
			task.Error = "non-persistent request headers require recapture"
			task.UpdatedAt = now
			_ = s.store.Upsert(task)
		}
		switch task.State {
		case shared.DownloadTaskPausing:
			task.State = shared.DownloadTaskPaused
			task.Error = ""
			task.UpdatedAt = now
			_ = s.store.Upsert(task)
		case shared.DownloadTaskResolving, shared.DownloadTaskDownloading, shared.DownloadTaskProcessing:
			task.State = shared.DownloadTaskInterrupted
			task.Error = "application exited before the task completed"
			task.UpdatedAt = now
			_ = s.store.Upsert(task)
		}
		s.tasks[task.ID] = task
		if taskOwnsResource(task.State) {
			s.byResource[task.ResourceID] = task.ID
		}
	}
}

func taskNeedsRecaptureForHeaders(task shared.DownloadTaskRecord) bool {
	if shared.ResourceNeedsRecaptureForHeaders(task.Resource) {
		return true
	}
	for _, item := range task.Items {
		if shared.ResourceNeedsRecaptureForHeaders(item.Resource) {
			return true
		}
	}
	return false
}

func (s *Scheduler) requeuePending() {
	s.mu.RLock()
	ids := make([]string, 0)
	for id, task := range s.tasks {
		if task.State == shared.DownloadTaskPending {
			ids = append(ids, id)
		}
	}
	s.mu.RUnlock()
	for _, id := range ids {
		s.queue <- id
	}
}

func activeDownloadTaskState(state string) bool {
	switch state {
	case shared.DownloadTaskPending, shared.DownloadTaskResolving, shared.DownloadTaskDownloading, shared.DownloadTaskProcessing, shared.DownloadTaskPausing:
		return true
	default:
		return false
	}
}

func taskOwnsResource(state string) bool {
	return activeDownloadTaskState(state) || state == shared.DownloadTaskPaused
}

var errTaskPaused = errors.New("download task paused")

func planIsResumable(plan shared.DownloadPlan) bool {
	if len(plan.Inputs) == 0 {
		return false
	}
	for _, input := range plan.Inputs {
		executor := input.Executor
		if executor == "" {
			executor = "http-file"
		}
		if executor != "http-file" && executor != "capture-file" {
			return false
		}
	}
	return true
}

func planIsRecording(plan shared.DownloadPlan) bool {
	for _, input := range plan.Inputs {
		if input.Executor == "ffmpeg-hls" {
			return true
		}
	}
	return false
}

func collectionIsResumable(items []shared.DownloadTaskItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !planIsResumable(item.Plan) {
			return false
		}
	}
	return true
}

func isCancelledDownload(err error) bool {
	return err != nil && (errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "cancelled") || strings.Contains(err.Error(), "canceled"))
}

func (s *Scheduler) Enqueue(resource shared.ResourceCandidate) (shared.DownloadTaskRecord, error) {
	if resource.ID == "" {
		return shared.DownloadTaskRecord{}, errors.New("resource id is required")
	}
	s.mu.Lock()
	if taskID := s.byResource[resource.ID]; taskID != "" {
		task := s.tasks[taskID]
		s.mu.Unlock()
		return task, nil
	}
	configuredDirectory := s.config.Snapshot().SaveDirectory
	if configuredDirectory == "" {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, errors.New("save directory is empty")
	}
	saveDirectory, err := filepath.Abs(configuredDirectory)
	if err != nil {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, fmt.Errorf("resolve save directory: %w", err)
	}
	id, _ := gonanoid.New()
	if id == "" {
		id = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	now := time.Now().UnixMilli()
	task := shared.DownloadTaskRecord{
		ID: id, ResourceID: resource.ID, ParentID: resource.ParentID, Resource: resource,
		PluginID: resource.Source.PluginID, PluginVersion: resource.Source.PluginVersion,
		PluginDigest: resource.Source.PluginDigest, State: shared.DownloadTaskPending,
		CreatedAt: now, UpdatedAt: now, SaveDirectory: saveDirectory,
		TempPath: filepath.Join(saveDirectory, taskWorkspaceDirectory, id),
	}
	children := s.resources.ChildrenOf(resource)
	if resource.Kind == shared.ResourceKindCollection || len(children) > 0 {
		for _, child := range children {
			item := shared.DownloadTaskItem{Resource: child, State: shared.DownloadTaskPending}
			if child.State != shared.ResourceStatePartial && contains(child.Capabilities, shared.ResourceCapabilityDownload) &&
				child.Lifecycle.Availability != shared.ResourceAvailabilityNeedsRefresh {
				if plan, planErr := s.plugins.CreateDownloadPlan(s.ctx, child, shared.DownloadOptions{}); planErr == nil {
					item.Plan = plan
				}
			}
			task.Items = append(task.Items, item)
		}
		task.Resumable = collectionIsResumable(task.Items)
	} else if resource.Lifecycle.Availability != shared.ResourceAvailabilityNeedsRefresh {
		plan, err := s.plugins.CreateDownloadPlan(s.ctx, resource, shared.DownloadOptions{})
		if err != nil {
			s.mu.Unlock()
			return shared.DownloadTaskRecord{}, err
		}
		task.Plan = plan
		task.Resumable = planIsResumable(plan)
		task.Recording = planIsRecording(plan)
	}
	s.tasks[id], s.byResource[resource.ID] = task, id
	s.mu.Unlock()
	if err := s.persist(task); err != nil {
		s.mu.Lock()
		delete(s.tasks, id)
		delete(s.byResource, resource.ID)
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, err
	}
	s.resources.EmitDownloadTaskEvent(task)
	select {
	case s.queue <- id:
		return task, nil
	case <-s.ctx.Done():
		return shared.DownloadTaskRecord{}, errors.New("download scheduler is stopped")
	}
}

func (s *Scheduler) worker(stop <-chan struct{}) {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-stop:
			return
		case id := <-s.queue:
			s.execute(id)
		}
	}
}

func (s *Scheduler) execute(id string) {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists || task.State != shared.DownloadTaskPending {
		return
	}
	taskCtx, cancel := context.WithCancelCause(s.ctx)
	s.mu.Lock()
	s.cancelFuncs[id] = cancel
	s.mu.Unlock()
	var releaseCancelOnce sync.Once
	releaseCancel := func() {
		releaseCancelOnce.Do(func() {
			s.mu.Lock()
			delete(s.cancelFuncs, id)
			s.mu.Unlock()
		})
	}
	defer func() {
		cancel(context.Canceled)
		releaseCancel()
	}()
	if task.StartedAt == 0 {
		task.Attempts++
		task.StartedAt = time.Now().UnixMilli()
	}
	s.update(&task, shared.DownloadTaskResolving, "createDownloadPlan", "")

	resource := task.Resource
	if resource.ID == "" {
		s.fail(&task, errors.New("resource not found"))
		return
	}
	if resource.Kind == shared.ResourceKindCollection || len(task.Items) > 0 {
		if len(task.Items) == 0 {
			s.fail(&task, errors.New("collection has no downloadable children"))
			return
		}
		s.executeCollection(taskCtx, &task, releaseCancel)
		return
	}
	if resource.Lifecycle.Availability == shared.ResourceAvailabilityNeedsRefresh ||
		(resource.Lifecycle.ExpiresAt > 0 && resource.Lifecycle.ExpiresAt <= time.Now().UnixMilli()) {
		refreshed, status, err := s.plugins.RefreshResource(taskCtx, resource, shared.DownloadOptions{})
		if err != nil {
			s.fail(&task, err)
			return
		}
		if status != shared.ResourceRefreshOK {
			s.fail(&task, fmt.Errorf("resource requires recapture: %s", status))
			return
		}
		resource = refreshed
	}
	plan := task.Plan
	if len(plan.Inputs) == 0 {
		var err error
		plan, err = s.plugins.CreateDownloadPlan(taskCtx, resource, shared.DownloadOptions{})
		if err != nil {
			s.fail(&task, err)
			return
		}
	}
	task.Resource, task.Plan, task.Resumable, task.Recording = resource, plan, planIsResumable(plan), planIsRecording(plan)
	resource.Lifecycle.LastResolvedAt = time.Now().UnixMilli()
	resource.Lifecycle.UpdatedAt = resource.Lifecycle.LastResolvedAt
	task.Resource = resource
	if err := taskCtx.Err(); err != nil {
		if errors.Is(context.Cause(taskCtx), errTaskPaused) {
			releaseCancel()
			s.update(&task, shared.DownloadTaskPaused, "", "")
			return
		}
		s.update(&task, shared.DownloadTaskCancelled, "", "cancelled")
		return
	}
	step := "download"
	if task.Recording {
		step = "recording"
	}
	s.update(&task, shared.DownloadTaskDownloading, step, "")
	path, err := s.resources.RunDownloadPlan(taskCtx, resource, plan, task.SaveDirectory, shared.DownloadExecution{
		TaskID: task.ID, WorkDir: task.TempPath,
	})
	if err != nil {
		if errors.Is(context.Cause(taskCtx), errTaskPaused) {
			releaseCancel()
			s.update(&task, shared.DownloadTaskPaused, "", "")
			return
		}
		if s.ctx.Err() != nil {
			s.update(&task, shared.DownloadTaskInterrupted, "", "application exited before the task completed")
			return
		}
		if isCancelledDownload(err) || s.taskState(id) == shared.DownloadTaskCancelled {
			_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
			s.update(&task, shared.DownloadTaskCancelled, "", "cancelled")
			return
		}
		s.fail(&task, err)
		return
	}
	task.OutputPath, task.FinishedAt = path, time.Now().UnixMilli()
	if s.taskState(id) == shared.DownloadTaskCancelled {
		s.update(&task, shared.DownloadTaskCancelled, "", "cancelled; partial output was preserved")
		return
	}
	_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
	s.update(&task, shared.DownloadTaskCompleted, "", "")
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Scheduler) executeCollection(ctx context.Context, task *shared.DownloadTaskRecord, releaseCancel func()) {
	resource := task.Resource
	folderName := naming.TruncateFilenameSegment(naming.SanitizeFilenameSegment(resource.Title), naming.MaxFilenameSegmentBytes)
	if folderName == "" {
		folderName = "collection-" + resource.ID
	}
	directory := filepath.Join(task.SaveDirectory, folderName)
	if err := os.MkdirAll(directory, 0755); err != nil {
		s.fail(task, fmt.Errorf("create collection directory: %w", err))
		return
	}
	task.OutputPath = directory
	completed := int64(0)
	for _, item := range task.Items {
		if item.State == shared.DownloadTaskCompleted {
			completed++
		}
	}
	task.Downloaded, task.Total = completed, int64(len(task.Items))
	s.update(task, shared.DownloadTaskDownloading, "collection", "")

	failed := 0
	for index := range task.Items {
		if task.Items[index].State == shared.DownloadTaskCompleted {
			continue
		}
		if err := ctx.Err(); err != nil {
			if errors.Is(context.Cause(ctx), errTaskPaused) {
				releaseCancel()
				s.update(task, shared.DownloadTaskPaused, "", "")
				return
			}
			if s.ctx.Err() != nil {
				s.update(task, shared.DownloadTaskInterrupted, "", "application exited before the task completed")
				return
			}
			_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
			s.update(task, shared.DownloadTaskCancelled, "", "cancelled")
			return
		}
		item := &task.Items[index]
		item.State = shared.DownloadTaskDownloading
		child := item.Resource
		var itemErr error
		if child.State == shared.ResourceStatePartial || !contains(child.Capabilities, shared.ResourceCapabilityDownload) {
			itemErr = errors.New("child resource is not downloadable")
		} else {
			if child.Lifecycle.Availability == shared.ResourceAvailabilityNeedsRefresh ||
				(child.Lifecycle.ExpiresAt > 0 && child.Lifecycle.ExpiresAt <= time.Now().UnixMilli()) {
				var status string
				child, status, itemErr = s.plugins.RefreshResource(ctx, child, shared.DownloadOptions{})
				if itemErr == nil && status != shared.ResourceRefreshOK {
					itemErr = fmt.Errorf("resource requires recapture: %s", status)
				}
			}
			plan := item.Plan
			if itemErr == nil && len(plan.Inputs) == 0 {
				plan, itemErr = s.plugins.CreateDownloadPlan(ctx, child, shared.DownloadOptions{})
			}
			if itemErr == nil {
				item.Resource, item.Plan = child, plan
				item.OutputPath, itemErr = s.resources.RunDownloadPlan(ctx, child, plan, directory, shared.DownloadExecution{
					TaskID: task.ID, WorkDir: filepath.Join(task.TempPath, fmt.Sprintf("item-%06d", index)),
				})
			}
		}
		if itemErr != nil {
			if errors.Is(context.Cause(ctx), errTaskPaused) {
				item.State = shared.DownloadTaskPaused
				releaseCancel()
				s.update(task, shared.DownloadTaskPaused, "", "")
				return
			}
			if s.ctx.Err() != nil {
				item.State = shared.DownloadTaskInterrupted
				s.update(task, shared.DownloadTaskInterrupted, "", "application exited before the task completed")
				return
			}
			if isCancelledDownload(itemErr) {
				_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
				s.update(task, shared.DownloadTaskCancelled, "", "cancelled")
				return
			}
			item.State = shared.DownloadTaskFailed
			failed++
		} else {
			item.State = shared.DownloadTaskCompleted
		}
		task.Downloaded++
		task.UpdatedAt = time.Now().UnixMilli()
		s.mu.Lock()
		s.tasks[task.ID] = *task
		s.mu.Unlock()
		_ = s.persist(*task)
		s.resources.EmitDownloadTaskEvent(*task)
	}
	if failed > 0 {
		s.fail(task, fmt.Errorf("%d of %d collection children failed", failed, len(task.Items)))
		return
	}
	task.FinishedAt = time.Now().UnixMilli()
	_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
	s.update(task, shared.DownloadTaskCompleted, "", "")
}

func (s *Scheduler) fail(task *shared.DownloadTaskRecord, err error) {
	task.FinishedAt = time.Now().UnixMilli()
	s.update(task, shared.DownloadTaskFailed, "", err.Error())
}

func (s *Scheduler) update(task *shared.DownloadTaskRecord, state, step, message string) {
	task.State, task.Step, task.Error, task.UpdatedAt = state, step, message, time.Now().UnixMilli()
	s.mu.Lock()
	// HTTP download progress is reported through Progress and therefore lives in
	// the scheduler's current task record, while execute keeps its own snapshot.
	// Preserve the current counters when that snapshot transitions state so a
	// pause or completion event cannot overwrite the latest visible progress.
	if current, exists := s.tasks[task.ID]; exists && len(task.Items) == 0 {
		task.Downloaded, task.Total = current.Downloaded, current.Total
	}
	s.tasks[task.ID] = *task
	if !taskOwnsResource(state) {
		delete(s.byResource, task.ResourceID)
	}
	s.mu.Unlock()
	_ = s.persist(*task)
	s.resources.EmitDownloadTaskEvent(*task)
}

func (s *Scheduler) persist(task shared.DownloadTaskRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.Upsert(task)
}

func (s *Scheduler) taskState(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id].State
}

func (s *Scheduler) List() []shared.DownloadTaskRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]shared.DownloadTaskRecord, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt == tasks[j].CreatedAt {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].CreatedAt > tasks[j].CreatedAt
	})
	return tasks
}

func (s *Scheduler) Progress(resourceID string, downloaded, total int64) {
	s.mu.Lock()
	id := s.byResource[resourceID]
	task, exists := s.tasks[id]
	if !exists {
		s.mu.Unlock()
		return
	}
	task.Downloaded, task.Total, task.UpdatedAt = downloaded, total, time.Now().UnixMilli()
	s.tasks[id] = task
	shouldPersist := time.Since(s.lastProgress[id]) >= 500*time.Millisecond || (total > 0 && downloaded >= total)
	if shouldPersist {
		s.lastProgress[id] = time.Now()
	}
	s.mu.Unlock()
	if shouldPersist {
		_ = s.persist(task)
		s.resources.EmitDownloadTaskEvent(task)
	}
}

func (s *Scheduler) Processing(id string) {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists || task.State != shared.DownloadTaskDownloading {
		return
	}
	s.update(&task, shared.DownloadTaskProcessing, "process", "")
}

func (s *Scheduler) Cancel(resourceID string) error {
	s.mu.RLock()
	id := s.byResource[resourceID]
	task, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists {
		return errors.New("task not found")
	}
	if task.State == shared.DownloadTaskPending || task.State == shared.DownloadTaskPaused {
		task.FinishedAt = time.Now().UnixMilli()
		_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
		s.update(&task, shared.DownloadTaskCancelled, "", "cancelled")
		return nil
	}
	s.mu.RLock()
	cancel := s.cancelFuncs[id]
	s.mu.RUnlock()
	if cancel != nil {
		cancel(context.Canceled)
	}
	_ = s.resources.CancelActive(resourceID)
	return nil
}

func (s *Scheduler) CancelTask(id string) error {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists || !taskOwnsResource(task.State) {
		return errors.New("active task not found")
	}
	return s.Cancel(task.ResourceID)
}

func (s *Scheduler) StopRecording(id string) error {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()
	if !exists || !task.Recording || !activeDownloadTaskState(task.State) {
		return errors.New("active recording not found")
	}
	return s.Cancel(task.ResourceID)
}

func (s *Scheduler) Pause(id string) (shared.DownloadTaskRecord, error) {
	s.mu.Lock()
	task, exists := s.tasks[id]
	pausableState := task.State == shared.DownloadTaskPending || task.State == shared.DownloadTaskResolving || task.State == shared.DownloadTaskDownloading
	if !exists || !pausableState || !task.Resumable {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, errors.New("task cannot be paused")
	}
	if task.State == shared.DownloadTaskPending {
		task.State, task.Step, task.Error, task.UpdatedAt = shared.DownloadTaskPaused, "", "", time.Now().UnixMilli()
		if err := s.persist(task); err != nil {
			s.mu.Unlock()
			return shared.DownloadTaskRecord{}, err
		}
		s.tasks[id] = task
		s.mu.Unlock()
		s.resources.EmitDownloadTaskEvent(task)
		return task, nil
	}
	task.State, task.UpdatedAt = shared.DownloadTaskPausing, time.Now().UnixMilli()
	if err := s.persist(task); err != nil {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, err
	}
	s.tasks[id] = task
	cancel := s.cancelFuncs[id]
	s.mu.Unlock()
	s.resources.EmitDownloadTaskEvent(task)
	if cancel != nil {
		cancel(errTaskPaused)
	}
	return task, nil
}

func (s *Scheduler) Resume(id string) (shared.DownloadTaskRecord, error) {
	s.mu.Lock()
	task, exists := s.tasks[id]
	resumableState := task.State == shared.DownloadTaskPaused || task.State == shared.DownloadTaskInterrupted
	if !exists || !resumableState || !task.Resumable {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, errors.New("task cannot be resumed")
	}
	task.State, task.Step, task.Error = shared.DownloadTaskPending, "", ""
	task.FinishedAt = 0
	task.Resumes++
	task.UpdatedAt = time.Now().UnixMilli()
	for index := range task.Items {
		if task.Items[index].State == shared.DownloadTaskPaused || task.Items[index].State == shared.DownloadTaskDownloading {
			task.Items[index].State = shared.DownloadTaskPending
		}
	}
	if err := s.persist(task); err != nil {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, err
	}
	s.tasks[id], s.byResource[task.ResourceID] = task, id
	s.mu.Unlock()
	s.resources.EmitDownloadTaskEvent(task)
	s.queue <- id
	return task, nil
}

func (s *Scheduler) Retry(id string) (shared.DownloadTaskRecord, error) {
	s.mu.Lock()
	task, exists := s.tasks[id]
	if !exists || activeDownloadTaskState(task.State) {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, errors.New("task cannot be retried")
	}
	if active := s.byResource[task.ResourceID]; active != "" {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, errors.New("resource already has an active task")
	}
	task.State, task.Step, task.Error = shared.DownloadTaskPending, "", ""
	task.Downloaded, task.Total, task.StartedAt, task.FinishedAt = 0, 0, 0, 0
	task.OutputPath = ""
	task.UpdatedAt = time.Now().UnixMilli()
	if err := s.persist(task); err != nil {
		s.mu.Unlock()
		return shared.DownloadTaskRecord{}, err
	}
	s.tasks[id], s.byResource[task.ResourceID] = task, id
	s.mu.Unlock()
	s.resources.EmitDownloadTaskEvent(task)
	s.queue <- id
	return task, nil
}

func (s *Scheduler) Delete(id string) error {
	s.mu.Lock()
	task, exists := s.tasks[id]
	if !exists {
		s.mu.Unlock()
		return errors.New("task not found")
	}
	if taskOwnsResource(task.State) || s.cancelFuncs[id] != nil {
		s.mu.Unlock()
		return errors.New("active task cannot be deleted")
	}
	if s.store != nil {
		if err := s.store.Delete([]string{id}); err != nil {
			s.mu.Unlock()
			return err
		}
	}
	delete(s.tasks, id)
	delete(s.lastProgress, id)
	s.mu.Unlock()
	_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
	s.resources.EmitDownloadTaskRemoved(id)
	return nil
}

func (s *Scheduler) ClearFinished() (int, error) {
	s.mu.Lock()
	ids := make([]string, 0)
	for id, task := range s.tasks {
		if taskOwnsResource(task.State) || s.cancelFuncs[id] != nil {
			continue
		}
		ids = append(ids, id)
	}
	if s.store != nil {
		if err := s.store.Delete(ids); err != nil {
			s.mu.Unlock()
			return 0, err
		}
	}
	workspaces := make([]shared.DownloadTaskRecord, 0, len(ids))
	for _, id := range ids {
		workspaces = append(workspaces, s.tasks[id])
		delete(s.tasks, id)
		delete(s.lastProgress, id)
	}
	s.mu.Unlock()
	for _, task := range workspaces {
		_ = s.cleanupWorkspace(task.ID, task.SaveDirectory, task.TempPath)
	}
	for _, id := range ids {
		s.resources.EmitDownloadTaskRemoved(id)
	}
	return len(ids), nil
}

func (s *Scheduler) Close() {
	s.closeOnce.Do(func() {
		s.workerMu.Lock()
		s.started = false
		s.workerMu.Unlock()
		s.cancel()
		s.mu.RLock()
		for _, cancelTask := range s.cancelFuncs {
			cancelTask(context.Canceled)
		}
		s.mu.RUnlock()
		s.resources.CancelAllActive()
		s.mu.Lock()
		for id, task := range s.tasks {
			if activeDownloadTaskState(task.State) {
				task.State, task.Error, task.UpdatedAt = shared.DownloadTaskInterrupted, "application exited before the task completed", time.Now().UnixMilli()
				s.tasks[id] = task
				_ = s.persist(task)
			}
		}
		s.mu.Unlock()
		s.wg.Wait()
		if s.store != nil {
			_ = s.store.Close()
		}
	})
}

func (s *Scheduler) CleanupWorkspaces() error {
	if s == nil || s.config == nil {
		return nil
	}
	configuredDirectory := strings.TrimSpace(s.config.Snapshot().SaveDirectory)
	if configuredDirectory == "" {
		return nil
	}
	saveDirectory, err := filepath.Abs(configuredDirectory)
	if err != nil {
		return fmt.Errorf("resolve current save directory: %w", err)
	}
	workspaceRoot := filepath.Join(saveDirectory, taskWorkspaceDirectory)
	if filepath.Dir(workspaceRoot) != filepath.Clean(saveDirectory) || filepath.Base(workspaceRoot) != taskWorkspaceDirectory {
		return errors.New("invalid download workspace root")
	}
	return os.RemoveAll(workspaceRoot)
}

func (s *Scheduler) cleanupWorkspace(taskID, saveDirectory, path string) error {
	if taskID == "" || saveDirectory == "" || path == "" || filepath.Base(taskID) != taskID || taskID == "." {
		return nil
	}
	expectedPath := filepath.Join(filepath.Clean(saveDirectory), taskWorkspaceDirectory, taskID)
	cleanPath := filepath.Clean(path)
	if cleanPath != expectedPath {
		return nil
	}
	workspaceRoot := filepath.Dir(cleanPath)
	if filepath.Base(cleanPath) != taskID || filepath.Base(workspaceRoot) != taskWorkspaceDirectory {
		return nil
	}
	if err := os.RemoveAll(cleanPath); err != nil {
		return err
	}
	_ = os.Remove(workspaceRoot)
	return nil
}
