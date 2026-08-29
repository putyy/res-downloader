package resource

import (
	"context"
	"path/filepath"
	"res-downloader/internal/config"
	downloadengine "res-downloader/internal/download"
	"res-downloader/internal/logging"
	"res-downloader/internal/media"
	shared "res-downloader/internal/model"
	"sync"
)

type Resource struct {
	emit       func(string, ...interface{})
	config     *config.Config
	logger     *logging.Logger
	plugins    resourcePluginService
	downloads  resourceDownloadQueue
	media      *media.Engine
	captures   downloadengine.CaptureSource
	mediaMark  sync.Map
	tasks      sync.Map
	catalog    sync.Map
	groupIndex sync.Map
	catalogMux sync.Mutex
	store      *Store
	resType    map[string]bool
	resTypeMux sync.RWMutex
}

type Page struct {
	Items      []shared.ResourceView `json:"items"`
	Total      int                   `json:"total"`
	NextOffset int                   `json:"nextOffset"`
}

func (r *Resource) emitEvent(name string, data interface{}) {
	if r != nil && r.emit != nil {
		r.emit(name, data)
	}
}

func (r *Resource) SetPlugins(plugins resourcePluginService)               { r.plugins = plugins }
func (r *Resource) SetDownloads(downloads resourceDownloadQueue)           { r.downloads = downloads }
func (r *Resource) SetCaptureSource(captures downloadengine.CaptureSource) { r.captures = captures }

type cancellableDownload interface{ Cancel() }

type resourcePluginService interface {
	ProcessWASM(context.Context, shared.DownloadStep, string, uint64) (string, error)
}

type resourceDownloadQueue interface {
	Cancel(string) error
	Progress(string, int64, int64)
	Processing(string)
}

type PluginStatusProvider interface {
	Status(string) (shared.PluginStatus, bool)
}

// Candidate returns the current catalog copy for a persisted resource.
func (r *Resource) Candidate(id string) (shared.ResourceCandidate, bool) {
	stored, exists := r.catalog.Load(id)
	if !exists {
		return shared.ResourceCandidate{}, false
	}
	candidate, ok := stored.(shared.ResourceCandidate)
	return candidate, ok
}

func (r *Resource) CandidateByGroup(pluginID, groupKey string) (shared.ResourceCandidate, bool) {
	id, exists := r.groupIndex.Load(resourceGroupIndexKey(pluginID, groupKey))
	if !exists {
		return shared.ResourceCandidate{}, false
	}
	value, ok := id.(string)
	if !ok || value == "" {
		return shared.ResourceCandidate{}, false
	}
	return r.Candidate(value)
}

// SaveCandidate updates both the in-memory catalog and its durable store.
func (r *Resource) SaveCandidate(candidate shared.ResourceCandidate) error {
	r.catalog.Store(candidate.ID, candidate)
	if r.store != nil {
		return r.store.Upsert(candidate)
	}
	return nil
}

func (r *Resource) Clear()                  { r.clear() }
func (r *Resource) DeleteMany(ids []string) { r.deleteMany(ids) }
func (r *Resource) List() []shared.ResourceView {
	return r.list()
}
func (r *Resource) ListPage(offset, limit int) Page {
	items := r.list()
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 1000
	}
	if limit > 5000 {
		limit = 5000
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := min(offset+limit, len(items))
	page := Page{Items: items[offset:end], Total: len(items)}
	if end < len(items) {
		page.NextOffset = end
	}
	return page
}
func (r *Resource) Import(items []shared.ResourceView) ([]shared.ResourceView, error) {
	return r.importResources(items)
}
func (r *Resource) ChildrenOf(parent shared.ResourceCandidate) []shared.ResourceCandidate {
	return r.childrenOf(parent)
}

func (r *Resource) RunDownloadPlan(ctx context.Context, candidate shared.ResourceCandidate, plan shared.DownloadPlan, directory string, execution shared.DownloadExecution) (string, error) {
	return r.runDownloadPlanContext(ctx, candidate, plan, directory, execution)
}

func (r *Resource) CancelActive(id string) error {
	return r.cancelActive(id)
}

func (r *Resource) CancelAllActive() {
	r.tasks.Range(func(_, value interface{}) bool {
		if task, ok := value.(cancellableDownload); ok {
			task.Cancel()
		}
		return true
	})
}

func (r *Resource) EmitDownloadTaskEvent(task shared.DownloadTaskRecord) {
	r.emitEvent("downloadTaskUpdated", task)
}

func (r *Resource) EmitDownloadTaskRemoved(id string) {
	r.emitEvent("downloadTaskRemoved", map[string]string{"id": id})
}

func New(userDir string, config *config.Config, media *media.Engine, logger *logging.Logger, emit func(string, ...interface{})) *Resource {
	resources := &Resource{emit: emit, config: config, media: media, logger: logger}
	resources.resType = map[string]bool{
		"all": true, shared.ResourceTypeVideo: true, shared.ResourceTypeAudio: true,
		shared.ResourceTypeImage: true, shared.ResourceTypeDocument: true,
		shared.ResourceTypeArchive: true, shared.ResourceTypeCollection: true,
		shared.ResourceTypeOther: true,
	}
	store, err := Open(filepath.Join(userDir, "resources.db"))
	if err != nil {
		logger.Esg(err, "open resource database; using memory-only catalog")
		return resources
	}
	resources.store = store
	if err := resources.restore(); err != nil {
		logger.Esg(err, "restore resource database")
	}
	return resources
}

func (r *Resource) mediaIsMarked(key string) bool {
	_, loaded := r.mediaMark.Load(key)
	return loaded
}
