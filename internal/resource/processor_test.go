package resource

import (
	"bytes"
	"os"
	"path/filepath"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"testing"
)

func newProcessorTestResource(t *testing.T) *Resource {
	t.Helper()
	resources := New(t.TempDir(), &config.Config{}, nil, logging.New(false, ""), nil)
	t.Cleanup(resources.Close)
	return resources
}

func TestDownloadProcessorsAreTransactional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resource.bin")
	original := []byte("leave the completed download untouched")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	resources := newProcessorTestResource(t)
	err := resources.ProcessDownload(path, []shared.DownloadStep{
		{Type: "xor-prefix", Options: map[string]interface{}{"key": "AQ=="}},
		{Type: "unsupported-test-processor"},
	}, 0, true)
	if err == nil {
		t.Fatal("expected processor failure")
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("failed processor chain modified the completed download")
	}
}

func TestDownloadProcessorChainReplacesCompletedDownload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resource.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3, 4}, 0600); err != nil {
		t.Fatal(err)
	}
	resources := newProcessorTestResource(t)
	if err := resources.ProcessDownload(path, []shared.DownloadStep{
		{Type: "xor-prefix", Options: map[string]interface{}{"key": "AQI="}},
	}, 0, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 3, 4}
	if !bytes.Equal(got, want) {
		t.Fatalf("processed download = %v, expected %v", got, want)
	}
}

func TestReplaceProcessedDownloadInstallsMissingDestination(t *testing.T) {
	directory := t.TempDir()
	processedPath := filepath.Join(directory, "processed.bin")
	destinationPath := filepath.Join(directory, "download.bin")
	want := []byte("completed download")
	if err := os.WriteFile(processedPath, want, 0600); err != nil {
		t.Fatal(err)
	}

	if err := replaceProcessedDownload(processedPath, destinationPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(processedPath); !os.IsNotExist(err) {
		t.Fatalf("processed file still exists: %v", err)
	}
	got, err := os.ReadFile(destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("installed download = %q, expected %q", got, want)
	}
}
