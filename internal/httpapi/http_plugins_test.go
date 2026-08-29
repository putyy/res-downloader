package httpapi

import (
	"net/http"
	shared "res-downloader/internal/model"
	"testing"
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
