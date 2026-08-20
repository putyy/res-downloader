package core

import (
	"net/http"
	"testing"
)

func TestSetHeadersDropsCapturedRange(t *testing.T) {
	oldConfig := globalConfig
	globalConfig = &Config{UseHeaders: "default"}
	t.Cleanup(func() { globalConfig = oldConfig })

	downloader := NewFileDownloader("https://example.com/video.mp4", "video.mp4", 1, map[string]string{
		"Range":      "bytes=0-65535",
		"If-Range":   "etag-value",
		"User-Agent": "test-agent",
		"Referer":    "https://example.com/",
	})
	request, err := http.NewRequest(http.MethodGet, downloader.Url, nil)
	if err != nil {
		t.Fatal(err)
	}

	downloader.setHeaders(request)

	if got := request.Header.Get("Range"); got != "" {
		t.Fatalf("captured Range header leaked into full download: %q", got)
	}
	if got := request.Header.Get("If-Range"); got != "" {
		t.Fatalf("captured If-Range header leaked into full download: %q", got)
	}
	if got := request.Header.Get("User-Agent"); got != "test-agent" {
		t.Fatalf("User-Agent was not preserved: %q", got)
	}
	if got := request.Header.Get("Referer"); got != "https://example.com/" {
		t.Fatalf("Referer was not preserved: %q", got)
	}
}

func TestSetHeadersDropsCapturedRangeWithCustomHeaderList(t *testing.T) {
	oldConfig := globalConfig
	globalConfig = &Config{UseHeaders: "User-Agent,Referer,Range,If-Range"}
	t.Cleanup(func() { globalConfig = oldConfig })

	downloader := NewFileDownloader("https://example.com/video.mp4", "video.mp4", 1, map[string]string{
		"Range":      "bytes=0-65535",
		"If-Range":   "etag-value",
		"User-Agent": "test-agent",
	})
	request, err := http.NewRequest(http.MethodGet, downloader.Url, nil)
	if err != nil {
		t.Fatal(err)
	}

	downloader.setHeaders(request)

	if got := request.Header.Get("Range"); got != "" {
		t.Fatalf("captured Range header leaked with custom headers: %q", got)
	}
	if got := request.Header.Get("If-Range"); got != "" {
		t.Fatalf("captured If-Range header leaked with custom headers: %q", got)
	}
	if got := request.Header.Get("User-Agent"); got != "test-agent" {
		t.Fatalf("User-Agent was not preserved: %q", got)
	}
}
