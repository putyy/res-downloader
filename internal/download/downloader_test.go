package download

import (
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
)

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupDownloaderTest() (*config.Config, *logging.Logger) {
	config := &config.Config{
		UseHeaders: "default",
		UserAgent:  "res-downloader-test",
	}
	return config, logging.New(false, "")
}

func TestDownloaderResumesFromPersistentCheckpoint(t *testing.T) {
	config, logger := setupDownloaderTest()
	body := bytes.Repeat([]byte("resumable-video-data"), 128*1024)
	var rangesMu sync.Mutex
	ranges := make([]string, 0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("ETag", `"resume-fixture"`)
		if r.Method == http.MethodHead {
			return
		}
		start := 0
		if value := r.Header.Get("Range"); value != "" {
			rangesMu.Lock()
			ranges = append(ranges, value)
			rangesMu.Unlock()
			text, _, _ := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")
			start, _ = strconv.Atoi(text)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)-start))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(body)-1, len(body)))
			w.WriteHeader(http.StatusPartialContent)
		}
		for offset := start; offset < len(body); offset += 32 * 1024 {
			end := offset + 32*1024
			if end > len(body) {
				end = len(body)
			}
			_, _ = w.Write(body[offset:end])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(time.Millisecond)
		}
	}))
	defer srv.Close()

	directory := t.TempDir()
	output := filepath.Join(directory, "input.part")
	checkpoint := output + ".json"
	ctx, cancel := context.WithCancel(context.Background())
	first := NewFileDownloaderContext(ctx, srv.URL, output, checkpoint, 1, nil, config, logger)
	var cancelOnce sync.Once
	first.progressCallback = func(downloaded, _ float64, _ int, _ float64) {
		if downloaded >= 128*1024 {
			cancelOnce.Do(cancel)
		}
	}
	if err := first.Start(); err == nil {
		t.Fatal("expected the first download to be interrupted")
	}
	if _, err := os.Stat(checkpoint); err != nil {
		t.Fatalf("checkpoint was not preserved: %v", err)
	}

	second := NewFileDownloaderContext(context.Background(), srv.URL, output, checkpoint, 1, nil, config, logger)
	if err := second.Start(); err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("resumed content differs: got %d bytes, want %d", len(got), len(body))
	}
	rangesMu.Lock()
	defer rangesMu.Unlock()
	resumed := false
	for _, value := range ranges {
		if value != fmt.Sprintf("bytes=0-%d", len(body)-1) && value != "bytes=0-" {
			resumed = true
		}
	}
	if !resumed {
		t.Fatalf("no non-zero resume range was requested: %#v", ranges)
	}
}

func TestDownloaderDoesNotReuseBrowserPlaybackRange(t *testing.T) {
	config, logger := setupDownloaderTest()
	body := bytes.Repeat([]byte("complete-video-data"), 1024)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Range"); got != "" {
			t.Errorf("unexpected inherited Range on %s request: %q", r.Method, got)
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "out.mp4")
	fd := NewFileDownloader(srv.URL, out, 1, map[string]string{
		"Range": "bytes=0-1955",
	}, config, logger)
	if err := fd.Start(); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("downloaded content differs: got %d bytes, want %d", len(got), len(body))
	}
}

func TestDownloaderContinuesUnexpectedPartialResponse(t *testing.T) {
	config, logger := setupDownloaderTest()
	body := bytes.Repeat([]byte("complete-video-data"), 1024)
	const firstPartSize = 1956
	getCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "none")
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			return
		}

		getCount++
		if getCount == 1 {
			w.Header().Set("Content-Length", fmt.Sprint(firstPartSize))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", firstPartSize-1, len(body)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body[:firstPartSize])
			return
		}

		wantRange := fmt.Sprintf("bytes=%d-%d", firstPartSize, len(body)-1)
		if got := r.Header.Get("Range"); got != wantRange {
			t.Errorf("resume Range = %q, want %q", got, wantRange)
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(body)-firstPartSize))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", firstPartSize, len(body)-1, len(body)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(body[firstPartSize:])
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "out.mp4")
	fd := NewFileDownloader(srv.URL, out, 1, map[string]string{
		"Range": "bytes=0-1955",
	}, config, logger)
	if err := fd.Start(); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("downloaded content differs: got %d bytes, want %d", len(got), len(body))
	}
}

func TestDownloaderDoesNotRetryPermanentHTTPStatus(t *testing.T) {
	config, logger := setupDownloaderTest()
	getCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
		if r.Method == http.MethodGet {
			getCount++
			http.Error(w, "forbidden", http.StatusForbidden)
		}
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "out.mp4")
	fd := NewFileDownloader(srv.URL, out, 1, nil, config, logger)
	err := fd.Start()
	if err == nil || !strings.Contains(err.Error(), "unexpected status code: 403") {
		t.Fatalf("download error = %v, want an actionable 403 error", err)
	}
	if getCount != 1 {
		t.Fatalf("GET request count = %d, want 1", getCount)
	}
}
