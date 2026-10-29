package download

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	shared "res-downloader/internal/model"
)

func TestXHSImageFallback(t *testing.T) {
	primary := "https://sns-webpic-qc.xhscdn.com/expiry/signature/image.webp?token=example"
	backup := "https://sns-webpic-backup.xhscdn.com/expiry/signature/image.webp?token=example"
	for _, tc := range []struct {
		name       string
		primaryErr error
		want       []string
		wantErr    bool
	}{
		{"success does not request backup", nil, []string{primary}, false},
		{"404 requests backup", &downloadHTTPStatusError{statusCode: http.StatusNotFound}, []string{primary, backup}, false},
		{"server error requests backup", &downloadHTTPStatusError{statusCode: http.StatusServiceUnavailable}, []string{primary, backup}, false},
		{"network error requests backup", &net.DNSError{Err: "network unavailable", Name: "fixture"}, []string{primary, backup}, false},
		{"access denied does not request backup", &downloadHTTPStatusError{statusCode: http.StatusForbidden}, []string{primary}, true},
		{"local error does not request backup", errors.New("disk full"), []string{primary}, true},
		{"cancel does not request backup", context.Canceled, []string{primary}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := shared.DownloadInput{ID: "image", URL: primary, Options: map[string]interface{}{"xiaohongshuBackup": true}}
			var visited []string
			path, err := runXHSImageFallback(context.Background(), input, func(input shared.DownloadInput, backup bool) (string, error) {
				visited = append(visited, input.URL)
				if len(visited) == 1 && tc.primaryErr != nil {
					return "", tc.primaryErr
				}
				return "saved", nil
			})
			if !reflect.DeepEqual(visited, tc.want) {
				t.Fatalf("visited %v, want %v", visited, tc.want)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && path != "saved" {
				t.Fatalf("unexpected path %q", path)
			}
		})
	}
}

func TestXHSFallbackDownloadsFileAfterPrimary404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/primary" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("backup image bytes"))
	}))
	defer server.Close()
	cfg, logger := setupDownloaderTest()
	runner := NewPlanRunnerContext(context.Background(), nil, cfg, logger, nil, nil, t.TempDir())
	input := shared.DownloadInput{ID: "image", URL: "https://sns-webpic-qc.xhscdn.com/fixture", Options: map[string]interface{}{"xiaohongshuBackup": true}}
	path, err := runXHSImageFallback(context.Background(), input, func(attempt shared.DownloadInput, backup bool) (string, error) {
		if backup {
			attempt.URL = server.URL + "/backup"
		} else {
			attempt.URL = server.URL + "/primary"
		}
		return runner.acquireHTTPOnce(attempt, t.TempDir(), 1, backup)
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "backup image bytes" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestXHSFallbackRejectsUntrustedOrDisabledInputs(t *testing.T) {
	for _, input := range []shared.DownloadInput{
		{URL: "https://sns-webpic-qc.xhscdn.com/fixture"},
		{URL: "https://sns-webpic-qc.xhscdn.com.attacker.example/fixture", Options: map[string]interface{}{"xiaohongshuBackup": true}},
		{URL: "http://sns-webpic-qc.xhscdn.com/fixture", Options: map[string]interface{}{"xiaohongshuBackup": true}},
	} {
		calls := 0
		_, err := runXHSImageFallback(context.Background(), input, func(shared.DownloadInput, bool) (string, error) {
			calls++
			return "", &downloadHTTPStatusError{statusCode: http.StatusNotFound}
		})
		if err == nil || calls != 1 {
			t.Fatalf("input %q: calls=%d err=%v", input.URL, calls, err)
		}
	}
}

