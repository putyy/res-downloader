package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
	"time"
)

func TestTrustedPluginManagementRequest(t *testing.T) {
	tests := []struct {
		method  string
		origin  string
		allowed bool
	}{
		{method: http.MethodPost, origin: "wails://wails.localhost", allowed: true},
		{method: http.MethodPost, origin: "http://wails.localhost", allowed: true},
		{method: http.MethodPost, origin: "http://localhost:34115", allowed: true},
		{method: http.MethodPost, origin: "", allowed: true},
		{method: http.MethodPost, origin: "https://attacker.example", allowed: false},
		{method: http.MethodPost, origin: "null", allowed: false},
		{method: http.MethodGet, origin: "wails://wails.localhost", allowed: false},
	}
	for _, test := range tests {
		request, err := http.NewRequest(test.method, "http://127.0.0.1/api/plugins/install", nil)
		if err != nil {
			t.Fatal(err)
		}
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		if got := trustedPluginManagementRequest(request); got != test.allowed {
			t.Errorf("method=%s origin=%q: got %v, want %v", test.method, test.origin, got, test.allowed)
		}
	}
}

func TestExpiredLocalPluginArchiveIsNotReturned(t *testing.T) {
	token := strings.Repeat("a", 48)
	server := &Server{pluginArchives: map[string]pendingPluginArchive{
		token: {data: []byte("archive"), expiresAt: time.Now().Add(-time.Second)},
	}}
	if _, err := server.takePluginArchive(token); !errors.Is(err, errPluginArchiveToken) {
		t.Fatalf("expired package error = %v", err)
	}
	if len(server.pluginArchives) != 0 {
		t.Fatal("expired package remained in the confirmation cache")
	}
}

func TestLocalPluginInstallReportsUnusableConfirmation(t *testing.T) {
	server := &Server{pluginArchives: make(map[string]pendingPluginArchive)}
	for _, token := range []string{"", "invalid", strings.Repeat("a", 48)} {
		raw, _ := json.Marshal(map[string]string{"token": token})
		request := httptest.NewRequest(http.MethodPost, "/api/plugins/file/install", strings.NewReader(string(raw)))
		response := httptest.NewRecorder()
		server.installPluginFile(response, request)
		var result struct {
			Code int `json:"code"`
			Data struct {
				ErrorCode string `json:"errorCode"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Code != 0 || result.Data.ErrorCode != "plugin_package_token_expired" {
			t.Fatalf("unexpected confirmation response: %s", response.Body.String())
		}
	}
}

func TestPendingLocalPluginArchiveTokenIsSingleUse(t *testing.T) {
	server := &Server{pluginArchives: make(map[string]pendingPluginArchive)}
	token, err := server.rememberPluginArchive([]byte("archive"), shared.PluginManifest{ID: "test.local"}, "digest")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := server.takePluginArchive(token)
	if err != nil {
		t.Fatal(err)
	}
	if pending.manifest.ID != "test.local" || string(pending.data) != "archive" {
		t.Fatalf("unexpected pending package: %#v", pending)
	}
	if _, err := server.takePluginArchive(token); err == nil {
		t.Fatal("expected the local plugin token to be single-use")
	}
}
