package httpapi

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	shared "res-downloader/internal/model"
	"res-downloader/internal/operation"
	"res-downloader/internal/plugin"
	"strings"
	"testing"
)

func TestDownloadResponsePreservesCreatedTaskWhenLinkingFails(t *testing.T) {
	plugins := &plugin.PluginManager{}
	plugins.SetOperations(&operation.Service{}) // unavailable operation store
	h := &Server{plugins: plugins}
	task := shared.DownloadTaskRecord{ID: "created", ResourceID: "resource", PluginID: "test.plugin", Resource: shared.ResourceCandidate{ID: "resource", Source: shared.ResourceSource{PluginID: "test.plugin"}}}
	w := httptest.NewRecorder()
	h.respondDownloadTask(w, task, nil)
	var result struct {
		Code    int                `json:"code"`
		Message string             `json:"message"`
		Data    linkedDownloadTask `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 1 || result.Data.ID != task.ID || result.Data.OperationLinkError == "" || !strings.Contains(result.Message, "association failed") {
		t.Fatalf("association failure hid a successful task: %s", w.Body.String())
	}
}

func TestDownloadResponseKeepsCommittedIDOnSchedulerError(t *testing.T) {
	h := &Server{}
	w := httptest.NewRecorder()
	h.respondDownloadTask(w, shared.DownloadTaskRecord{ID: "created"}, errors.New("scheduler stopped"))
	var result struct {
		Code    int                `json:"code"`
		Message string             `json:"message"`
		Data    linkedDownloadTask `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Data.ID != "created" || !strings.Contains(result.Message, "created") {
		t.Fatalf("partial scheduler error hid task ID: %s", w.Body.String())
	}
}
