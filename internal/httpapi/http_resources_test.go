package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/resource"
	"testing"
)

func TestUpdateResourcePersistsTitle(t *testing.T) {
	logger := logging.New(false, "")
	userDir := t.TempDir()
	resources := resource.New(userDir, &config.Config{}, nil, logger, nil)
	if err := resources.SaveCandidate(shared.ResourceCandidate{
		ID: "resource-1", DedupeKey: "resource-1", Kind: "media.video", Title: "Original title",
	}); err != nil {
		t.Fatal(err)
	}
	server := New(Host{}, "test-session", &config.Config{}, nil, resources, nil, nil, nil, logger)

	request := httptest.NewRequest(http.MethodPost, "/api/resources/update", bytes.NewBufferString(`{
		"id":"resource-1",
		"title":"Edited title"
	}`))
	request.Header.Set("Authorization", "Bearer test-session")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	if handled := server.HandleAPI(recorder, request); !handled {
		t.Fatal("update resource API was not handled")
	}
	var response ResponseData
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 1 {
		t.Fatalf("response = %#v", response)
	}
	updated, exists := resources.Candidate("resource-1")
	if !exists || updated.Title != "Edited title" {
		t.Fatalf("updated resource = %#v, exists = %v", updated, exists)
	}

	resources.Close()
	reopened := resource.New(userDir, &config.Config{}, nil, logger, nil)
	defer reopened.Close()
	persisted, exists := reopened.Candidate("resource-1")
	if !exists || persisted.Title != "Edited title" {
		t.Fatalf("persisted resource = %#v, exists = %v", persisted, exists)
	}
}

func TestUpdateResourceRejectsUnknownID(t *testing.T) {
	logger := logging.New(false, "")
	resources := resource.New(t.TempDir(), &config.Config{}, nil, logger, nil)
	defer resources.Close()
	server := New(Host{}, "test-session", &config.Config{}, nil, resources, nil, nil, nil, logger)

	request := httptest.NewRequest(http.MethodPost, "/api/resources/update", bytes.NewBufferString(`{"id":"missing","title":"Edited"}`))
	request.Header.Set("Authorization", "Bearer test-session")
	recorder := httptest.NewRecorder()
	server.HandleAPI(recorder, request)

	var response ResponseData
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 || response.Message != resource.ErrResourceNotFound.Error() {
		t.Fatalf("response = %#v", response)
	}
}
