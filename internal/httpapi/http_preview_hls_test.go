package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"res-downloader/internal/resource"
	"strings"
	"testing"
)

func TestHLSPreviewRewritesNestedPlaylistsSegmentsAndKeys(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Preview-Token") != "allowed" {
			http.Error(writer, "missing captured header", http.StatusForbidden)
			return
		}
		if request.Header.Get("If-None-Match") != "" || request.Header.Get("If-Modified-Since") != "" {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		switch request.URL.Path {
		case "/master.m3u8":
			writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = writer.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nmedia/index.m3u8\n"))
		case "/media/index.m3u8":
			writer.Header().Set("Content-Type", "application/x-mpegurl")
			_, _ = writer.Write([]byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\n#EXTINF:4,\nsegment.ts\n#EXT-X-ENDLIST\n"))
		case "/media/key.bin":
			writer.Header().Set("Content-Type", "application/octet-stream")
			_, _ = writer.Write([]byte("0123456789abcdef"))
		case "/media/segment.ts":
			writer.Header().Set("Content-Type", "video/mp2t")
			if request.Header.Get("Range") == "bytes=2-4" {
				writer.Header().Set("Content-Range", "bytes 2-4/12")
				writer.WriteHeader(http.StatusPartialContent)
				_, _ = writer.Write([]byte("gme"))
				return
			}
			_, _ = writer.Write([]byte("segment-data"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer upstream.Close()

	logger := logging.New(false, "")
	resources := resource.New(t.TempDir(), &config.Config{}, nil, logger, nil)
	t.Cleanup(resources.Close)
	manager := plugin.NewManager(
		t.TempDir(),
		func() plugin.NetworkSettings { return plugin.NetworkSettings{} },
		nil,
		resources,
		logger,
	)
	resources.SetPlugins(manager)
	candidate := shared.ResourceCandidate{
		ID: "hls-preview", Kind: "stream.hls", PrimaryType: shared.ResourceTypeVideo,
		Capabilities: []string{shared.ResourceCapabilityPreview, shared.ResourceCapabilityDownload},
		Preview:      &shared.PreviewSpec{Renderer: "video", TrackID: "primary", MIME: "application/vnd.apple.mpegurl"},
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: "video", Executor: "hls", URL: upstream.URL + "/master.m3u8",
			MIME: "application/vnd.apple.mpegurl", Extension: ".m3u8",
			Headers: map[string]string{
				"X-Preview-Token":   "allowed",
				"If-None-Match":     `"captured-cache-entry"`,
				"If-Modified-Since": "yesterday",
			},
		}},
		Metadata: map[string]interface{}{"stream.protocol": "hls", "stream.mode": "master"},
		Source:   shared.ResourceSource{PluginID: "builtin.generic-detector"},
	}
	if err := resources.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}

	server := New(Host{Context: func() context.Context { return context.Background() }}, "test-session", &config.Config{}, nil, resources, manager, nil, nil, logger)
	master := performPreviewRequest(t, server, "/api/preview?id="+candidate.ID)
	childURL := manifestMediaURL(t, master.Body.String())
	if strings.Contains(master.Body.String(), "media/index.m3u8") || !strings.HasPrefix(childURL, "/api/preview/hls?token=") {
		t.Fatalf("master manifest was not rewritten: %q", master.Body.String())
	}
	headRequest := httptest.NewRequest(http.MethodHead, "/api/preview?id="+candidate.ID, nil)
	headRequest.Header.Set("Authorization", "Bearer "+server.SessionToken())
	headResponse := httptest.NewRecorder()
	server.HandleAPI(headResponse, headRequest)
	if headResponse.Code != http.StatusOK || headResponse.Body.Len() != 0 || headResponse.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Fatalf("HEAD manifest response: status=%d type=%q body=%q", headResponse.Code, headResponse.Header().Get("Content-Type"), headResponse.Body.String())
	}

	media := performPreviewRequest(t, server, childURL)
	segmentURL := manifestMediaURL(t, media.Body.String())
	keyMatch := regexp.MustCompile(`URI="([^"]+)"`).FindStringSubmatch(media.Body.String())
	if len(keyMatch) != 2 || !strings.HasPrefix(keyMatch[1], "/api/preview/hls?token=") {
		t.Fatalf("key URI was not rewritten: %q", media.Body.String())
	}
	key := performPreviewRequest(t, server, keyMatch[1])
	if key.Body.String() != "0123456789abcdef" {
		t.Fatalf("key body = %q", key.Body.String())
	}
	segment := performPreviewRequest(t, server, segmentURL)
	if segment.Body.String() != "segment-data" || segment.Header().Get("Content-Type") != "video/mp2t" {
		t.Fatalf("segment status=%d type=%q body=%q", segment.Code, segment.Header().Get("Content-Type"), segment.Body.String())
	}
	rangeRequest := httptest.NewRequest(http.MethodGet, segmentURL, nil)
	rangeRequest.Header.Set("Range", "bytes=2-4")
	rangeResponse := httptest.NewRecorder()
	server.HandleAPI(rangeResponse, rangeRequest)
	if rangeResponse.Code != http.StatusPartialContent || rangeResponse.Body.String() != "gme" || rangeResponse.Header().Get("Content-Range") != "bytes 2-4/12" {
		t.Fatalf("range response: status=%d range=%q body=%q", rangeResponse.Code, rangeResponse.Header().Get("Content-Range"), rangeResponse.Body.String())
	}

	expired := performPreviewRequestStatus(server, "/api/preview/hls?token=missing")
	if expired.Code != http.StatusGone {
		t.Fatalf("missing token status = %d", expired.Code)
	}
}

func performPreviewRequest(t *testing.T, server *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := performPreviewRequestStatus(server, target)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s: status=%d body=%q", target, recorder.Code, recorder.Body.String())
	}
	return recorder
}

func performPreviewRequestStatus(server *Server, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", "Bearer "+server.SessionToken())
	recorder := httptest.NewRecorder()
	server.HandleAPI(recorder, request)
	return recorder
}

func manifestMediaURL(t *testing.T, manifest string) string {
	t.Helper()
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	t.Fatalf("manifest contains no media URI: %q", manifest)
	return ""
}
