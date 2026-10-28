package httpapi

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"path/filepath"
	"res-downloader/internal/capture"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"res-downloader/internal/resource"
	"strings"
	"testing"
)

func TestCapturePreviewHTTPContract(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", "example.preview")
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"example.preview","name":"Preview fixture","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":["observe-response","capture-response-body"]}}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.js"), []byte("function createDownloadPlan() { return null; }"), 0600); err != nil {
		t.Fatal(err)
	}
	logger := logging.New(false, "")
	resources := resource.New(root, &config.Config{}, nil, logger, nil)
	t.Cleanup(resources.Close)
	store, err := capture.New(filepath.Join(root, "captures"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resources.SetCaptureSource(store)
	manager := plugin.NewManager(root, func() plugin.NetworkSettings { return plugin.NetworkSettings{} }, nil, resources, logger)
	if status, ok := manager.Status("example.preview"); !ok || !status.Loaded {
		t.Fatalf("fixture plugin failed to load: %#v", status)
	}
	server := New(Host{Context: context.Background}, "fixture-session", &config.Config{}, nil, resources, manager, nil, nil, logger)
	content := "中文\n0123456789"
	key := "example.preview\x00asset"
	if err := store.StartStream(key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendStream(key, []byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteStream(key); err != nil {
		t.Fatal(err)
	}
	candidate := shared.ResourceCandidate{ID: "capture-preview", GroupKey: "asset", Kind: "document.text", PrimaryType: "document",
		Source: shared.ResourceSource{PluginID: "example.preview"}, Capabilities: []string{"preview", "download"},
		Tracks:  []shared.ResourceTrack{{ID: "file", Role: "primary", Executor: "capture-file", CaptureKey: "asset", Extension: ".txt", MIME: "text/plain; charset=utf-8"}},
		Preview: &shared.PreviewSpec{Renderer: "text", TrackID: "file", MIME: "text/plain; charset=utf-8"}}
	request := func(method, byteRange string, auth bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/preview?id="+candidate.ID, nil)
		if auth {
			r.Header.Set("Authorization", "Bearer "+server.SessionToken())
		}
		if byteRange != "" {
			r.Header.Set("Range", byteRange)
		}
		w := httptest.NewRecorder()
		server.HandleAPI(w, r)
		return w
	}
	if err := resources.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, byteRange, want, contentRange string
		status                                int
	}{
		{"GET", "", content, "", 200}, {"HEAD", "", "", "", 200},
		{"GET", "bytes=7-9", "012", "bytes 7-9/17", 206},
		{"GET", "bytes=-3", "789", "bytes 14-16/17", 206},
		{"GET", "bytes=99-", "", "bytes */17", 416},
	} {
		w := request(tc.method, tc.byteRange, true)
		if w.Code != tc.status || (tc.status != 416 && w.Body.String() != tc.want) || w.Header().Get("Content-Range") != tc.contentRange {
			t.Fatalf("%s %s: %d %q %q", tc.method, tc.byteRange, w.Code, w.Body.String(), w.Header().Get("Content-Range"))
		}
		if tc.status != 416 && w.Header().Get("Content-Type") != candidate.Preview.MIME {
			t.Fatal("text MIME lost")
		}
	}
	// File source is independent of renderer/type; preview can serve arbitrary
	// binary bytes without forcing UTF-8 decoding or a text Content-Type.
	for _, format := range []struct{ renderer, mime string }{{"image", "image/png"}, {"audio", "audio/mp4"}, {"video", "video/mp4"}, {"pdf", "application/pdf"}} {
		candidate.Tracks[0].MIME = format.mime
		candidate.Preview.MIME = format.mime
		candidate.Preview.Renderer = format.renderer
		if err := resources.SaveCandidate(candidate); err != nil {
			t.Fatal(err)
		}
		w := request("GET", "bytes=0-1", true)
		if w.Code != 206 || w.Header().Get("Content-Type") != format.mime || w.Body.String() != content[:2] {
			t.Fatalf("binary %s: %d", format.mime, w.Code)
		}
	}
	if w := request("GET", "", false); w.Code == 200 {
		t.Fatal("missing API authentication accepted")
	}
	candidate.Tracks[0].CaptureKey = "missing"
	if err := resources.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "", true); w.Code != 409 {
		t.Fatalf("missing cache: %d %s", w.Code, w.Body.String())
	}
	// A raw key cannot escape the owner namespace, even if another plugin has
	// a complete object whose key would otherwise match.
	candidate.Tracks[0].CaptureKey = key
	if err := resources.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "", true); w.Code == 200 {
		t.Fatal("raw scoped key escaped namespace")
	}
	candidate.Tracks[0].CaptureKey = "asset"
	candidate.Source.PluginID = "builtin.generic-detector"
	if err := resources.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "", true); w.Code != 502 || !strings.Contains(w.Body.String(), "capture-response-body") {
		t.Fatalf("permission: %d %s", w.Code, w.Body.String())
	}
}

func TestCapturePreviewProcessorsAndBounds(t *testing.T) {
	store, err := capture.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	resources := resource.New(t.TempDir(), &config.Config{}, nil, logging.New(false, ""), nil)
	defer resources.Close()
	resources.SetCaptureSource(store)
	server := &Server{resources: resources}
	key := "example.preview\x00file"
	write := func(content []byte) {
		t.Helper()
		if err := store.StartStream(key); err != nil {
			t.Fatal(err)
		}
		if _, err := store.AppendStream(key, content); err != nil {
			t.Fatal(err)
		}
		if err := store.CompleteStream(key); err != nil {
			t.Fatal(err)
		}
	}
	content := "caption"
	xorKey := make([]byte, len(content))
	encrypted := []byte(content)
	for i := range xorKey {
		xorKey[i] = 90
		encrypted[i] ^= 90
	}
	write(encrypted)
	processors := []shared.DownloadStep{{Type: "xor-prefix", Options: map[string]interface{}{"key": base64.StdEncoding.EncodeToString(xorKey)}}}
	input := shared.DownloadInput{ID: "file", Executor: "capture-file", CaptureKey: key}
	candidate := shared.ResourceCandidate{Preview: &shared.PreviewSpec{Renderer: "text", MIME: "text/plain; charset=utf-8"}}
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/api/preview", nil)
		r.Header.Set("Range", "bytes=-3")
		w := httptest.NewRecorder()
		server.serveCapturePreview(w, r, candidate, input, processors)
		if method == "GET" && (w.Code != 206 || w.Body.String() != "ion" || w.Header().Get("Content-Range") != "bytes 4-6/7") {
			t.Fatalf("processed GET: %d %s", w.Code, w.Body.String())
		}
		if method == "HEAD" && (w.Code != 206 || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "3" || w.Header().Get("Content-Range") != "bytes 4-6/7") {
			t.Fatalf("processed HEAD: %d %s", w.Code, w.Body.String())
		}
	}
	write([]byte(strings.Repeat("x", int(processedPreviewChunkSize)+1)))
	r := httptest.NewRequest("GET", "/api/preview", nil)
	w := httptest.NewRecorder()
	server.serveCapturePreview(w, r, candidate, input, processors)
	if w.Code != 422 {
		t.Fatalf("oversized processed preview: %d", w.Code)
	}
	r.Header.Set("Range", "bytes=0-1")
	w = httptest.NewRecorder()
	server.serveCapturePreview(w, r, candidate, input, nil)
	if w.Code != 206 || w.Body.String() != "xx" {
		t.Fatalf("unprocessed file was size-limited: %d", w.Code)
	}
}
