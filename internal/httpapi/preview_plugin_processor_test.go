package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"res-downloader/internal/resource"
	"testing"
)

func TestPreviewUsesPluginWASM(t *testing.T) {
	logger := logging.New(false, "")
	resources := resource.New(t.TempDir(), &config.Config{}, nil, logger, nil)
	t.Cleanup(resources.Close)
	userDirectory := t.TempDir()
	installPreviewWASMTestPlugin(t, userDirectory)
	manager := plugin.NewManager(
		userDirectory,
		func() plugin.NetworkSettings { return plugin.NetworkSettings{} },
		nil,
		resources,
		logger,
	)
	resources.SetPlugins(manager)

	plaintext := make([]byte, 160*1024)
	for index := range plaintext {
		plaintext[index] = byte(index % 251)
	}
	plainPath := filepath.Join(t.TempDir(), "plain.bin")
	if err := os.WriteFile(plainPath, plaintext, 0600); err != nil {
		t.Fatal(err)
	}

	candidate := shared.ResourceCandidate{
		ID: "preview-resource", Kind: "media.video",
		Capabilities: []string{shared.ResourceCapabilityPreview, shared.ResourceCapabilityDownload},
		Preview:      &shared.PreviewSpec{Renderer: "video", TrackID: "video-primary", MIME: "video/mp4"},
		Tracks: []shared.ResourceTrack{{
			ID: "video-primary", Role: "video", URL: "https://example.com/encrypted.mp4", Extension: ".mp4",
			Processors: []shared.DownloadStep{{Type: "plugin-wasm", Options: map[string]interface{}{"processor": "xor", "key": float64(90)}}},
		}},
		Source: shared.ResourceSource{PluginID: "test.preview-wasm"},
	}
	plan, err := manager.Resolve(context.Background(), candidate, shared.DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	processor := plan.Inputs[0].Processors[0]
	encryptedPath, err := manager.ProcessWASM(context.Background(), processor, plainPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(encryptedPath)
	encrypted, err := os.ReadFile(encryptedPath)
	if err != nil {
		t.Fatal(err)
	}

	lastUpstreamMethod := ""
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastUpstreamMethod = request.Method
		writer.Header().Set("Content-Type", "video/mp4")
		writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(encrypted)))
		writer.Header().Set("Accept-Ranges", "bytes")
		if request.Method == http.MethodHead {
			writer.WriteHeader(http.StatusOK)
			return
		}
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(encrypted)-1, len(encrypted)))
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(encrypted)
	}))
	defer upstream.Close()
	candidate.Tracks[0].URL = upstream.URL
	candidate.Tracks[0].Processors = plan.Inputs[0].Processors
	if err := resources.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}

	server := New(Host{}, "test-session", &config.Config{}, nil, resources, manager, nil, nil, logger)
	request := httptest.NewRequest(http.MethodGet, "/api/preview?id="+candidate.ID, nil)
	request.Header.Set("Range", fmt.Sprintf("bytes=0-%d", len(encrypted)-1))
	request.Header.Set("Authorization", "Bearer "+server.SessionToken())
	recorder := httptest.NewRecorder()
	server.HandleAPI(recorder, request)
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("preview status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Equal(recorder.Body.Bytes(), plaintext) {
		t.Fatal("preview response was not decrypted by the plugin WASM")
	}

	headRequest := httptest.NewRequest(http.MethodHead, "/api/preview?id="+candidate.ID, nil)
	headRequest.Header.Set("Authorization", "Bearer "+server.SessionToken())
	headRecorder := httptest.NewRecorder()
	server.HandleAPI(headRecorder, headRequest)
	if lastUpstreamMethod != http.MethodHead {
		t.Fatalf("preview HEAD was forwarded upstream as %s", lastUpstreamMethod)
	}
	if headRecorder.Code != http.StatusOK || headRecorder.Body.Len() != 0 {
		t.Fatalf("preview HEAD status=%d body=%q", headRecorder.Code, headRecorder.Body.String())
	}
	if headRecorder.Header().Get("Content-Length") != fmt.Sprintf("%d", len(encrypted)) || headRecorder.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("preview HEAD headers: length=%q ranges=%q", headRecorder.Header().Get("Content-Length"), headRecorder.Header().Get("Accept-Ranges"))
	}
}

func installPreviewWASMTestPlugin(t *testing.T, userDirectory string) {
	t.Helper()
	directory := filepath.Join(userDirectory, "plugins", "test.preview-wasm")
	if err := os.MkdirAll(directory, 0750); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "id":"test.preview-wasm","name":"Preview WASM test","version":"1.0.0","apiVersion":1,
  "runtime":"javascript","entry":"main.js",
  "permissions":{"domains":["example.com"],"capabilities":["process-download"]},
  "processors":{"xor":{"runtime":"wasm","entry":"decrypt.wasm","apiVersion":1}}
}`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(manifest), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(`function onObservation() { return {decision: "continue"}; }`), 0640); err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(filepath.Join("..", "..", "examples", "plugins", "wasm-xor", "decrypt.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "decrypt.wasm"), wasm, 0640); err != nil {
		t.Fatal(err)
	}
}
