package download

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
)

func TestDownloadsDoNotMutatePublishedHeaders(t *testing.T) {
	cfg, logger := setupDownloaderTest()
	headers := map[string]string{"X-Resource": "shared"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != cfg.UserAgent || r.Header.Get("Referer") == "" || r.Header.Get("X-Resource") != "shared" {
			t.Error("download lost configured or default headers")
		}
		w.Header().Set("Content-Length", "1")
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte("x"))
		}
	}))
	defer server.Close()
	directory := t.TempDir()
	var workers sync.WaitGroup
	workers.Add(1)
	finished, readerDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer workers.Done()
		defer close(readerDone)
		for {
			select {
			case <-finished:
				return
			default:
				if _, err := json.Marshal(headers); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	var downloads sync.WaitGroup
	for index := 0; index < 4; index++ {
		downloads.Add(1)
		go func(index int) {
			defer downloads.Done()
			fd := NewFileDownloader(server.URL, filepath.Join(directory, fmt.Sprintf("%d.bin", index)), 1, headers, cfg, logger)
			defer fd.Cancel()
			if err := fd.Start(); err != nil {
				t.Error(err)
			}
		}(index)
	}
	downloads.Wait()
	close(finished)
	<-readerDone
	workers.Wait()
	if len(headers) != 1 || headers["X-Resource"] != "shared" {
		t.Fatalf("published headers were modified: %#v", headers)
	}
}
