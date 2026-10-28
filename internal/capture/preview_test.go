package capture

import (
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestCapturePreviewLeasesCompleteBytes(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key := "example.plugin\x00caption"
	if _, _, err := store.OpenComplete(key); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("missing: %v", err)
	}
	if err := store.StartStream(key); err != nil {
		t.Fatal(err)
	}
	content := "中文\ncaption"
	if _, err := store.AppendStream(key, []byte(content)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.OpenComplete(key); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("partial: %v", err)
	}
	if err := store.CompleteStream(key); err != nil {
		t.Fatal(err)
	}
	reader, _, err := store.OpenComplete(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.StartStream(key); err == nil {
		t.Fatal("replaced leased stream")
	}
	if writer, err := store.Begin(key, &http.Response{ContentLength: int64(len(content)), Header: http.Header{}}); err == nil {
		writer.Close()
		t.Fatal("overwrote leased range capture")
	}
	if _, _, err := store.OpenComplete("another.plugin\x00caption"); !errors.Is(err, ErrIncomplete) {
		t.Fatal("cross-plugin capture")
	}
	if _, err := reader.Seek(7, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "caption" {
		t.Fatalf("seek: %q, %v", data, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.StartStream(key); err != nil {
		t.Fatal("lease was not released", err)
	}
}
