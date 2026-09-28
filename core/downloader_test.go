package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func setDownloaderTestConfig(t *testing.T) {
	t.Helper()
	previousConfig := globalConfig
	globalConfig = &Config{
		UserAgent:  "res-downloader-test",
		UseHeaders: "default",
	}
	t.Cleanup(func() {
		globalConfig = previousConfig
	})
}

func TestContentRangeTotal(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "known total", value: "bytes 0-0/123", want: 123},
		{name: "unknown total", value: "bytes 0-0/*", want: -1},
		{name: "wrong range", value: "bytes 1-1/123", wantErr: true},
		{name: "total shorter than range", value: "bytes 0-0/0", wantErr: true},
		{name: "invalid range unit", value: "items 0-0/123", wantErr: true},
		{name: "malformed value", value: "not a content range", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := contentRangeTotal(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("contentRangeTotal(%q) unexpectedly succeeded with %d", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("contentRangeTotal(%q) failed: %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("contentRangeTotal(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestUnsatisfiedContentRangeTotal(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "empty resource", value: "bytes */0", want: 0},
		{name: "non-empty resource", value: "bytes */10", wantErr: true},
		{name: "invalid range unit", value: "items */0", wantErr: true},
		{name: "malformed value", value: "bytes 0-0/0", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := unsatisfiedContentRangeTotal(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("unsatisfiedContentRangeTotal(%q) unexpectedly succeeded with %d", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unsatisfiedContentRangeTotal(%q) failed: %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("unsatisfiedContentRangeTotal(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestIsDownloadCancellation(t *testing.T) {
	if !isDownloadCancellation(fmt.Errorf("download initialization: %w", context.Canceled)) {
		t.Fatal("wrapped context cancellation was not recognized")
	}
	if isDownloadCancellation(errors.New("download cancelled")) {
		t.Fatal("a cancellation message without the context sentinel was recognized")
	}
}

func TestWaitForRetryReturnsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelTimer := time.AfterFunc(25*time.Millisecond, cancel)
	defer cancelTimer.Stop()

	startedAt := time.Now()
	err := waitForRetry(ctx, RetryDelay)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForRetry error = %v, want context cancellation", err)
	}
	if elapsed := time.Since(startedAt); elapsed >= RetryDelay/2 {
		t.Fatalf("cancelled retry wait took %v, want less than %v", elapsed, RetryDelay/2)
	}
}

func TestFileDownloaderFallsBackToRangeProbeWhenHeadConnectionFails(t *testing.T) {
	setDownloaderTestConfig(t)

	body := bytes.Repeat([]byte("range-probe-fallback-data"), 8)
	tests := []struct {
		name         string
		headStatus   int
		probeStatus  int
		contentRange string
		wantSize     int64
	}{
		{
			name:         "known total",
			probeStatus:  http.StatusPartialContent,
			contentRange: fmt.Sprintf("bytes 0-0/%d", len(body)),
			wantSize:     int64(len(body)),
		},
		{name: "unknown total", probeStatus: http.StatusPartialContent, contentRange: "bytes 0-0/*", wantSize: -1},
		{name: "server ignores range", probeStatus: http.StatusOK, wantSize: int64(len(body))},
		{name: "head method unsupported", headStatus: http.StatusMethodNotAllowed, probeStatus: http.StatusPartialContent, contentRange: fmt.Sprintf("bytes 0-0/%d", len(body)), wantSize: int64(len(body))},
		{name: "head server error", headStatus: http.StatusInternalServerError, probeStatus: http.StatusPartialContent, contentRange: fmt.Sprintf("bytes 0-0/%d", len(body)), wantSize: int64(len(body))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var headCount, rangeProbeCount, downloadCount atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead {
					headCount.Add(1)
					if test.headStatus != 0 {
						w.WriteHeader(test.headStatus)
						return
					}
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack HEAD connection: %v", err)
						return
					}
					_ = conn.Close()
					return
				}
				if r.Header.Get("Range") == "bytes=0-0" {
					rangeProbeCount.Add(1)
					if test.probeStatus == http.StatusPartialContent {
						w.Header().Set("Accept-Ranges", "bytes")
						w.Header().Set("Content-Length", "1")
						w.Header().Set("Content-Range", test.contentRange)
						w.WriteHeader(http.StatusPartialContent)
						_, _ = w.Write(body[:1])
					} else {
						w.Header().Set("Content-Length", fmt.Sprint(len(body)))
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(body)
					}
					return
				}

				downloadCount.Add(1)
				w.Header().Set("Content-Length", fmt.Sprint(len(body)))
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			fd := NewFileDownloader(srv.URL, filepath.Join(t.TempDir(), "download.bin"), 1, map[string]string{})
			t.Cleanup(fd.cancelFunc)
			if err := fd.Start(); err != nil {
				t.Fatalf("download failed: %v", err)
			}

			got, err := os.ReadFile(fd.FileName)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, body) {
				t.Fatalf("downloaded content differs: got %d bytes, want %d", len(got), len(body))
			}
			if fd.TotalSize != test.wantSize {
				t.Fatalf("probed size = %d, want %d", fd.TotalSize, test.wantSize)
			}
			if headCount.Load() != 1 || rangeProbeCount.Load() != 1 || downloadCount.Load() != 1 {
				t.Fatalf("request counts = HEAD %d, range probe %d, download %d; want 1 each", headCount.Load(), rangeProbeCount.Load(), downloadCount.Load())
			}
		})
	}
}

func TestFileDownloaderDoesNotTrustAcceptRangesAfterProbeReturnsOK(t *testing.T) {
	setDownloaderTestConfig(t)

	body := bytes.Repeat([]byte("x"), int(MinPartSize+1))
	var headCount, rangeProbeCount, rangedDownloadCount, fullDownloadCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headCount.Add(1)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack HEAD connection: %v", err)
				return
			}
			_ = conn.Close()
			return
		}

		if r.Header.Get("Range") == "bytes=0-0" {
			rangeProbeCount.Add(1)
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}

		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			rangedDownloadCount.Add(1)
			var start, end int
			if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= len(body) {
				t.Errorf("unexpected range request %q", rangeHeader)
				http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(body)))
			w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body[start : end+1])
			return
		}

		fullDownloadCount.Add(1)
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	fd := NewFileDownloader(srv.URL, filepath.Join(t.TempDir(), "large-download.bin"), 4, map[string]string{})
	t.Cleanup(fd.cancelFunc)
	if err := fd.Start(); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	got, err := os.ReadFile(fd.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("downloaded content differs: got %d bytes, want %d", len(got), len(body))
	}
	if fd.IsMultiPart {
		t.Fatal("multipart download enabled after the range probe returned HTTP 200")
	}
	if headCount.Load() != 1 || rangeProbeCount.Load() != 1 || rangedDownloadCount.Load() != 0 || fullDownloadCount.Load() != 1 {
		t.Fatalf("request counts = HEAD %d, range probe %d, ranged download %d, full download %d; want 1, 1, 0, 1", headCount.Load(), rangeProbeCount.Load(), rangedDownloadCount.Load(), fullDownloadCount.Load())
	}
}

func TestFileDownloaderFallsBackToFullGetForEmptyResource(t *testing.T) {
	setDownloaderTestConfig(t)

	var headCount, rangeProbeCount, downloadCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headCount.Add(1)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack HEAD connection: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		if r.Header.Get("Range") == "bytes=0-0" {
			rangeProbeCount.Add(1)
			w.Header().Set("Content-Range", "bytes */0")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		downloadCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	fd := NewFileDownloader(srv.URL, filepath.Join(t.TempDir(), "empty.bin"), 1, map[string]string{})
	t.Cleanup(fd.cancelFunc)
	if err := fd.Start(); err != nil {
		t.Fatalf("empty resource download failed: %v", err)
	}

	info, err := os.Stat(fd.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("empty resource size = %d, want 0", info.Size())
	}
	if headCount.Load() != 1 || rangeProbeCount.Load() != 1 || downloadCount.Load() != 1 {
		t.Fatalf("request counts = HEAD %d, range probe %d, download %d; want 1 each", headCount.Load(), rangeProbeCount.Load(), downloadCount.Load())
	}
}

func TestFileDownloaderCancelInterruptsHeadRequest(t *testing.T) {
	setDownloaderTestConfig(t)

	headStarted := make(chan struct{})
	releaseHead := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			close(headStarted)
			select {
			case <-r.Context().Done():
			case <-releaseHead:
			}
		}
	}))
	defer srv.Close()

	targetPath := filepath.Join(t.TempDir(), "cancelled.bin")
	existingContent := []byte("keep the pre-existing file")
	if err := os.WriteFile(targetPath, existingContent, 0600); err != nil {
		t.Fatal(err)
	}
	fd := NewFileDownloader(srv.URL, targetPath, 1, map[string]string{})
	done := make(chan error, 1)
	go func() {
		done <- fd.Start()
	}()

	select {
	case <-headStarted:
	case <-time.After(time.Second):
		fd.Cancel()
		close(releaseHead)
		t.Fatal("HEAD request did not start")
	}

	fd.Cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("download error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		close(releaseHead)
		t.Fatal("cancelling did not interrupt the HEAD request")
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("pre-existing output was removed while cancelling preflight: %v", err)
	}
	if !bytes.Equal(got, existingContent) {
		t.Fatalf("pre-existing output changed: got %q, want %q", got, existingContent)
	}
}

func TestFileDownloaderCancelRemovesCreatedPartialOutput(t *testing.T) {
	setDownloaderTestConfig(t)

	downloadStarted := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "8")
			return
		}
		close(downloadStarted)
		<-r.Context().Done()
	}))
	defer srv.Close()

	targetPath := filepath.Join(t.TempDir(), "partial.bin")
	fd := NewFileDownloader(srv.URL, targetPath, 1, map[string]string{})
	done := make(chan error, 1)
	go func() {
		done <- fd.Start()
	}()

	select {
	case <-downloadStarted:
	case <-time.After(time.Second):
		fd.Cancel()
		t.Fatal("full download request did not start")
	}
	if _, err := os.Stat(targetPath); err != nil {
		fd.Cancel()
		t.Fatalf("output was not created before the download request: %v", err)
	}

	fd.Cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("download error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelling did not interrupt the full download request")
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("created partial output still exists after cancellation (stat error: %v)", err)
	}
}

func TestFileDownloaderCancelAfterCompletionPreservesOutput(t *testing.T) {
	setDownloaderTestConfig(t)

	body := []byte("done")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	fd := NewFileDownloader(srv.URL, filepath.Join(t.TempDir(), "complete.bin"), 1, map[string]string{})
	if err := fd.Start(); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	fd.Cancel()
	got, err := os.ReadFile(fd.FileName)
	if err != nil {
		t.Fatalf("completed output was removed by a late cancel: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("completed output = %q, want %q", got, body)
	}
	if fd.ctx.Err() != nil {
		t.Fatalf("late cancel changed completed downloader context: %v", fd.ctx.Err())
	}
}

func TestFileDownloaderCancelInterruptsRangeProbe(t *testing.T) {
	setDownloaderTestConfig(t)

	var headCount, rangeProbeCount, downloadCount atomic.Int32
	rangeProbeStarted := make(chan struct{})
	releaseRangeProbe := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headCount.Add(1)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack HEAD connection: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		if r.Header.Get("Range") == "bytes=0-0" {
			rangeProbeCount.Add(1)
			close(rangeProbeStarted)
			select {
			case <-r.Context().Done():
			case <-releaseRangeProbe:
			}
			return
		}
		downloadCount.Add(1)
	}))
	defer srv.Close()

	fd := NewFileDownloader(srv.URL, filepath.Join(t.TempDir(), "range-cancelled.bin"), 1, map[string]string{})
	t.Cleanup(fd.cancelFunc)
	done := make(chan error, 1)
	go func() {
		done <- fd.Start()
	}()

	select {
	case <-rangeProbeStarted:
	case <-time.After(time.Second):
		fd.Cancel()
		close(releaseRangeProbe)
		t.Fatal("range probe did not start")
	}

	fd.Cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("download error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		fd.Cancel()
		close(releaseRangeProbe)
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("cancelling did not interrupt the range probe")
	}

	if got := headCount.Load(); got != 1 {
		t.Fatalf("HEAD requests = %d, want 1", got)
	}
	if got := rangeProbeCount.Load(); got != 1 {
		t.Fatalf("range probe requests = %d, want 1", got)
	}
	if got := downloadCount.Load(); got != 0 {
		t.Fatalf("full download requests = %d, want 0 after cancellation", got)
	}
}

func TestFileDownloaderCancelInterruptsRetryWait(t *testing.T) {
	setDownloaderTestConfig(t)

	var headCount, rangeProbeCount atomic.Int32
	rangeProbeFailed := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			headCount.Add(1)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack HEAD connection: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		if r.Header.Get("Range") == "bytes=0-0" {
			rangeProbeCount.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			select {
			case rangeProbeFailed <- struct{}{}:
			default:
			}
		}
	}))
	defer srv.Close()

	fd := NewFileDownloader(srv.URL, filepath.Join(t.TempDir(), "retry-cancelled.bin"), 1, map[string]string{})
	done := make(chan error, 1)
	go func() {
		done <- fd.Start()
	}()

	select {
	case <-rangeProbeFailed:
	case <-time.After(time.Second):
		fd.Cancel()
		t.Fatal("range probe did not fail")
	}

	cancelTimer := time.AfterFunc(100*time.Millisecond, fd.Cancel)
	defer cancelTimer.Stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("download error = %v, want context cancellation", err)
		}
	case <-time.After(RetryDelay + time.Second):
		fd.Cancel()
		t.Fatalf("cancellation did not interrupt the %v retry wait", RetryDelay)
	}

	if got := headCount.Load(); got != 1 {
		t.Fatalf("HEAD requests = %d, want 1", got)
	}
	if got := rangeProbeCount.Load(); got != 1 {
		t.Fatalf("range probe requests = %d, want 1", got)
	}
}
