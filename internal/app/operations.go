package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	m "res-downloader/internal/model"
	"res-downloader/internal/operation"
	"time"
)

func (r *Runtime) initialiseOperations() error {
	r.Plugins.SetOperationDownloadLookup(r.Downloads.List)
	r.Plugins.SetOperationDownloadHandler(func(ctx context.Context, candidate m.ResourceCandidate) (string, error) {
		if !r.Downloads.DurableAvailable() {
			return "", errors.New("download database unavailable")
		}
		task, err := r.Downloads.EnqueueContext(ctx, candidate)
		return task.ID, err
	})
	r.Plugins.SetArtifactHandler(func(a m.OperationArtifact) (io.ReadSeekCloser, m.OperationArtifact, error) {
		a.Status = "unavailable"
		if a.CaptureKey != "" {
			if a.ExpiresAt <= time.Now().UnixMilli() {
				a.Status = "expired"
				return nil, a, errors.New("capture expired")
			}
			reader, updatedAt, err := r.Captures.OpenComplete(a.CaptureKey)
			if err != nil {
				return nil, a, err
			}
			expiry := updatedAt.Add(24 * time.Hour).UnixMilli()
			if expiry < a.ExpiresAt {
				a.ExpiresAt = expiry
			}
			if a.ExpiresAt <= time.Now().UnixMilli() {
				reader.Close()
				a.Status = "expired"
				return nil, a, errors.New("capture expired")
			}
			size, err := reader.Seek(0, io.SeekEnd)
			if err == nil {
				_, err = reader.Seek(0, io.SeekStart)
			}
			if err != nil {
				reader.Close()
				return nil, a, err
			}
			a.Size = size
			a.Status = "available"
			return reader, a, nil
		}
		for _, task := range r.Downloads.List() {
			if task.ID != a.DownloadTaskID || task.ResourceID != a.ResourceID || task.PluginID != a.PluginID {
				continue
			}
			if task.State != m.DownloadTaskCompleted || task.OutputPath == "" {
				switch task.State {
				case m.DownloadTaskPending, m.DownloadTaskResolving, m.DownloadTaskDownloading, m.DownloadTaskProcessing, m.DownloadTaskPausing, m.DownloadTaskPaused:
					a.Status = "pending"
				}
				return nil, a, errors.New("download output unavailable")
			}
			info, err := os.Lstat(task.OutputPath)
			if err != nil || !info.Mode().IsRegular() {
				return nil, a, errors.New("output missing or not a regular file")
			}
			file, err := os.Open(task.OutputPath)
			if err != nil {
				return nil, a, err
			}
			actual, err := file.Stat()
			if err != nil || !os.SameFile(info, actual) {
				file.Close()
				return nil, a, errors.New("output changed while opening")
			}
			a.Output = task.OutputPath
			a.Size = actual.Size()
			a.Status = "available"
			return file, a, nil
		}
		return nil, a, errors.New("registered output not found")
	})
	service, err := operation.New(filepath.Join(r.App.UserDir, "operations.db"), r.Plugins)
	if err != nil {
		return err
	}
	r.Operations = service
	r.Plugins.SetOperations(service)
	return nil
}
