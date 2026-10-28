package operation

import (
	"encoding/json"
	"slices"
	"time"

	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
)

// LinkDownload attaches a scheduler-returned task to retained executions that
// published its resource. The caller must supply the actual scheduler record,
// never a client-provided path or task description. Linking cannot replay an
// operation, revive a cleaned result, or extend its original retention window.
func (s *Service) LinkDownload(task m.DownloadTaskRecord) error {
	return s.linkDownload(task, false)
}

func (s *Service) linkDownload(task m.DownloadTaskRecord, completingFinalizer bool) error {
	if task.ID == "" || task.ResourceID == "" || task.PluginID == "" || task.Resource.ID != task.ResourceID || task.Resource.Source.PluginID != task.PluginID {
		return problem("invalid_download", "download association requires a scheduler task with matching resource ownership")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return err
	}
	if s.closing && !completingFinalizer {
		return problem("storage_unavailable", "operation service is closing")
	}
	if err := s.cleanLocked(false, false); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	changed := []*record{}
	artifacts := map[string]m.OperationArtifact{}
	for _, original := range s.records {
		v := original.View
		if original.Finalizing || !Terminal(v.State) || v.PluginID != task.PluginID || !slices.Contains(v.ResourceIDs, task.ResourceID) {
			continue
		}
		copy := *original
		copy.View = clone(v)
		dirty := false
		if !slices.Contains(copy.View.DownloadTaskIDs, task.ID) {
			if len(copy.View.DownloadTaskIDs) >= MaxItems {
				return problem("association_limit", "execution download association limit reached")
			}
			copy.View.DownloadTaskIDs = append(copy.View.DownloadTaskIDs, task.ID)
			dirty = true
		}
		// A missing/cleared payload is not permission to create new retained
		// artifacts. In particular, memory-only results disappear on restart.
		retained := (v.ResultStatus == "available" || len(v.ArtifactIDs) > 0) && v.ResultExpiresAt > now && v.ResultStatus != "cleaned" && v.ResultStatus != "expired"
		linked := false
		for _, artifactID := range v.ArtifactIDs {
			if a, ok := s.artifacts[artifactID]; ok && a.DownloadTaskID == task.ID && a.ResourceID == task.ResourceID {
				linked = true
				break
			}
		}
		if retained && !linked {
			if len(copy.View.ArtifactIDs) >= MaxItems {
				return problem("association_limit", "execution artifact association limit reached")
			}
			artifactID, err := id()
			if err != nil {
				return err
			}
			mime := "application/octet-stream"
			if len(task.Resource.Tracks) == 1 && task.Resource.Tracks[0].MIME != "" {
				mime = task.Resource.Tracks[0].MIME
			}
			artifacts[artifactID] = m.OperationArtifact{ArtifactID: artifactID, ExecutionID: v.ExecutionID, PluginID: task.PluginID, ResourceID: task.ResourceID, DownloadTaskID: task.ID, MIME: mime, Status: "pending", ExpiresAt: v.ResultExpiresAt}
			copy.View.ArtifactIDs = append(copy.View.ArtifactIDs, artifactID)
			dirty = true
		}
		if dirty {
			changed = append(changed, &copy)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	updates := s.keyUpdates(changed)
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := writeKeys(tx, updates); err != nil {
			return err
		}
		for _, r := range changed {
			raw, err := json.Marshal(persistent(r))
			if err != nil {
				return err
			}
			if err = tx.Bucket([]byte("executions")).Put([]byte(r.View.ExecutionID), raw); err != nil {
				return err
			}
			if !r.Definition.PersistResult {
				continue
			}
			for _, artifactID := range r.View.ArtifactIDs {
				a, created := artifacts[artifactID]
				if !created {
					continue
				}
				raw, err = json.Marshal(storedArtifact{Value: a})
				if err != nil {
					return err
				}
				if err = tx.Bucket([]byte("artifacts")).Put([]byte(artifactID), raw); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		s.fault = err
		return s.available()
	}
	for _, r := range changed {
		s.records[r.View.ExecutionID] = r
	}
	for artifactID, a := range artifacts {
		s.artifacts[artifactID] = a
	}
	for key, value := range updates {
		s.keys[key] = value
	}
	s.nextCleanup = 0
	return nil
}

// DownloadSnapshotBackend exposes host-owned scheduler records without coupling
// operation storage to the download scheduler. It must return a current snapshot.
type DownloadSnapshotBackend interface {
	OperationDownloads() []m.DownloadTaskRecord
}

// reconcileDownloads runs after the terminal execution and its published
// resources are saved, outside s.mu but inside the finalizer WaitGroup lifetime.
// Tasks created earlier appear in this snapshot; tasks created later are linked
// by their HTTP caller against the now-terminal execution. The overlap is safe
// because LinkDownload deduplicates task and artifact references.
func (s *Service) reconcileDownloads(pluginID string, resources []string) error {
	backend, ok := s.backend.(DownloadSnapshotBackend)
	if !ok || len(resources) == 0 {
		return nil
	}
	for _, task := range backend.OperationDownloads() {
		if task.PluginID != pluginID || !slices.Contains(resources, task.ResourceID) {
			continue
		}
		// Close waits for this finalizer before closing the DB. Finish linking
		// its existing effects even if closing has already stopped new callers.
		if err := s.linkDownload(task, true); err != nil {
			return err
		}
	}
	return nil
}
