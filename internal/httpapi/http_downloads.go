package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
)

type batchDownloadTaskResult struct {
	ID      string                     `json:"id"`
	Success bool                       `json:"success"`
	Error   string                     `json:"error,omitempty"`
	Task    *shared.DownloadTaskRecord `json:"task,omitempty"`
}

func (h *Server) createDownload(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	candidate, exists := h.resources.Candidate(data.ID)
	if !exists {
		h.error(w, "resource not found")
		return
	}
	if candidate.State == shared.ResourceStatePartial {
		h.error(w, "resource is still waiting for required tracks")
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	task, err := h.downloads.Enqueue(candidate)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, task)
}

func (h *Server) downloadTasks(w http.ResponseWriter, _ *http.Request) {
	if h.downloads == nil {
		h.success(w, []shared.DownloadTaskRecord{})
		return
	}
	h.success(w, h.downloads.List())
}

func (h *Server) retryDownload(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	task, err := h.downloads.Retry(data.ID)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, task)
}

func (h *Server) pauseDownloadTask(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	task, err := h.downloads.Pause(data.ID)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, task)
}

func (h *Server) resumeDownloadTask(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	task, err := h.downloads.Resume(data.ID)
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, task)
}

func (h *Server) cancelDownloadTask(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	if err := h.downloads.CancelTask(data.ID); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *Server) stopRecordingTask(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	if err := h.downloads.StopRecording(data.ID); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *Server) deleteDownloadTask(w http.ResponseWriter, r *http.Request) {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	if err := h.downloads.Delete(data.ID); err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w)
}

func (h *Server) batchDownloadTasks(w http.ResponseWriter, r *http.Request) {
	var data struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	if h.downloads == nil {
		h.error(w, "download scheduler is unavailable")
		return
	}
	if len(data.IDs) == 0 {
		h.error(w, "task ids are required")
		return
	}
	if data.Action != "pause" && data.Action != "resume" && data.Action != "cancel" && data.Action != "retry" && data.Action != "delete" {
		h.error(w, "unsupported batch action")
		return
	}

	results := make([]batchDownloadTaskResult, 0, len(data.IDs))
	succeeded := 0
	seen := make(map[string]struct{}, len(data.IDs))
	for _, id := range data.IDs {
		result := batchDownloadTaskResult{ID: id}
		if id == "" {
			result.Error = "task id is required"
			results = append(results, result)
			continue
		}
		if _, exists := seen[id]; exists {
			result.Error = "duplicate task id"
			results = append(results, result)
			continue
		}
		seen[id] = struct{}{}

		var task shared.DownloadTaskRecord
		var err error
		switch data.Action {
		case "pause":
			task, err = h.downloads.Pause(id)
		case "resume":
			task, err = h.downloads.Resume(id)
		case "cancel":
			err = h.downloads.CancelTask(id)
		case "retry":
			task, err = h.downloads.Retry(id)
		case "delete":
			err = h.downloads.Delete(id)
		default:
			err = errors.New("unsupported batch action")
		}
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Success = true
			succeeded++
			if task.ID != "" {
				result.Task = &task
			}
		}
		results = append(results, result)
	}

	h.success(w, respData{
		"succeeded": succeeded,
		"failed":    len(results) - succeeded,
		"results":   results,
	})
}

func (h *Server) clearDownloadTasks(w http.ResponseWriter, _ *http.Request) {
	if h.downloads == nil {
		h.success(w, respData{"count": 0})
		return
	}
	count, err := h.downloads.ClearFinished()
	if err != nil {
		h.error(w, err.Error())
		return
	}
	h.success(w, respData{"count": count})
}

func (h *Server) exportResources(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.error(w, err.Error())
		return
	}
	fileName := filepath.Join(h.config.Snapshot().SaveDirectory, "res-downloader-"+shared.GetCurrentDateTimeFormatted()+".txt")
	if err := os.WriteFile(fileName, []byte(data.Content), 0644); err != nil {
		h.error(w, err.Error())
		return
	}
	_ = shared.OpenFolder(fileName)
	h.success(w, respData{"file_name": fileName})
}
