package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	shared "res-downloader/internal/model"
	"testing"
)

func TestReadResponseBodyForPluginsDecodesGzipAndPreservesStream(t *testing.T) {
	want := []byte(`{"url":"https://example.com/video.mp4"}`)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response := &http.Response{Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(compressed.Bytes()))}
	response.Header.Set("Content-Encoding", "gzip")
	got, truncated := readResponseBodyForPlugins(response, 1024)
	if truncated || !bytes.Equal(got, want) {
		t.Fatalf("decoded = %q, truncated=%v", got, truncated)
	}
	preserved, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preserved, compressed.Bytes()) {
		t.Fatal("reading the plugin snapshot consumed the proxied response stream")
	}
}

func TestApplyHTTPResponsePatchUpdatesLengthAndEncoding(t *testing.T) {
	response := &http.Response{Header: make(http.Header), Body: io.NopCloser(bytes.NewReader([]byte("old")))}
	response.Header.Set("Content-Encoding", "gzip")
	body := "new body"
	applyHTTPResponsePatch(response, &shared.ResponsePatch{StatusCode: 201, Body: &body})
	if response.StatusCode != 201 || response.ContentLength != int64(len(body)) {
		t.Fatalf("unexpected patched response: %#v", response)
	}
	if response.Header.Get("Content-Encoding") != "" {
		t.Fatal("content encoding was not cleared after a plain-text body patch")
	}
}
