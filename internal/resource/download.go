package resource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	downloadengine "res-downloader/internal/download"
	shared "res-downloader/internal/model"
	"res-downloader/internal/naming"
	"strings"
	"sync"
	"time"
)

func (r *Resource) cancelActive(id string) error {
	if d, ok := r.tasks.Load(id); ok {
		task, ok := d.(cancellableDownload)
		if !ok {
			return errors.New("task cannot be cancelled")
		}
		task.Cancel()
		r.tasks.Delete(id) // 可选：取消后清理
		return nil
	}
	return errors.New("task not found")
}

func (r *Resource) runDownloadPlanContext(ctx context.Context, candidate shared.ResourceCandidate, plan shared.DownloadPlan, directory string, execution shared.DownloadExecution) (string, error) {
	configSnapshot := r.config.Snapshot()
	if directory == "" {
		return "", errors.New("save directory is empty")
	}
	if len(plan.Inputs) == 0 {
		return "", errors.New("download plan has no inputs")
	}
	savePath, err := naming.RenderResourcePath(directory, configSnapshot.FilenameTemplate, candidate, plan, time.Now())
	if err != nil {
		return "", errors.New("filename template error: " + err.Error())
	}
	savePath, releaseSavePath, err := r.reserveDownloadPath(savePath, configSnapshot.FilenameConflict)
	if err != nil {
		return "", errors.New("filename conflict: " + err.Error())
	}
	defer releaseSavePath()
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return "", errors.New("create output directory: " + err.Error())
	}
	runner := downloadengine.NewPlanRunnerContext(ctx, func(totalDownloaded, totalSize float64) {
		if r.downloads != nil {
			r.downloads.Progress(candidate.ID, int64(totalDownloaded), int64(totalSize))
		}
	}, &configSnapshot, r.logger, r.media, r, execution.WorkDir)
	runner.SetCaptureSource(r.captures)
	runner.OnProcessing(func() {
		if r.downloads != nil {
			r.downloads.Processing(execution.TaskID)
		}
	})
	r.tasks.Store(candidate.ID, runner)
	defer r.tasks.Delete(candidate.ID)
	outputPath, err := runner.Run(plan, directory, configSnapshot.TaskNumber)
	if err != nil {
		return "", err
	}
	if err := replaceProcessedDownload(outputPath, savePath); err != nil {
		_ = os.Remove(outputPath)
		return "", errors.New("install download error: " + err.Error())
	}
	return savePath, nil
}

func (r *Resource) reserveDownloadPath(path, strategy string) (string, func(), error) {
	r.outputMux.Lock()
	if r.outputs == nil {
		r.outputs = make(map[string]struct{})
	}
	resolved, err := naming.ResolveFilenameConflictWith(path, strategy, func(candidate string) bool {
		_, exists := r.outputs[filepath.Clean(candidate)]
		return exists
	})
	if err != nil {
		r.outputMux.Unlock()
		return "", nil, err
	}
	if strategy == "overwrite" {
		r.outputMux.Unlock()
		return resolved, func() {}, nil
	}
	key := filepath.Clean(resolved)
	r.outputs[key] = struct{}{}
	r.outputMux.Unlock()

	var once sync.Once
	return resolved, func() {
		once.Do(func() {
			r.outputMux.Lock()
			delete(r.outputs, key)
			r.outputMux.Unlock()
		})
	}, nil
}

func isCancelledDownload(err error) bool {
	return err != nil && (errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "cancelled") || strings.Contains(err.Error(), "canceled"))
}
