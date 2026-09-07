package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"res-downloader/internal/resource"
	"testing"
)

// Exercise the full route/auth/method pipeline, not only PageCommandStatuses:
// a switch handler is unreachable if knownAPIPath forgets to register its path.
func TestPageCommandStatusRoute(t *testing.T) {
	server := &Server{sessionToken: "test-session", plugins: &plugin.PluginManager{}}
	for _, tc := range []struct {
		name, method, token, origin string
		status                      int
	}{
		{"authenticated query", http.MethodPost, "test-session", "", http.StatusOK},
		{"desktop origin", http.MethodPost, "test-session", "http://wails.localhost", http.StatusOK},
		{"missing token", http.MethodPost, "", "", http.StatusUnauthorized},
		{"invalid token", http.MethodPost, "wrong", "", http.StatusUnauthorized},
		{"invalid method", http.MethodGet, "test-session", "", http.StatusMethodNotAllowed},
		{"foreign origin", http.MethodPost, "test-session", "https://example.com", http.StatusForbidden},
		{"preflight", http.MethodOptions, "", "http://localhost:5173", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, "/api/resources/page-commands", nil)
			if tc.token != "" {
				request.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			recorder := httptest.NewRecorder()
			server.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("page command API fell through to the asset handler")
			})).ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.status, recorder.Body.String())
			}
			if tc.status != http.StatusOK {
				return
			}
			var response struct {
				Code int                        `json:"code"`
				Data []shared.PageCommandStatus `json:"data"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.Code != 1 || response.Data == nil || len(response.Data) != 0 {
				t.Fatalf("expected successful empty status array, got %#v", response)
			}
		})
	}
}

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
