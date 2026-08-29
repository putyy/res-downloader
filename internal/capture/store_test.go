package capture

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamCaptureCompletesForCaptureFile(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.StartStream("plugin\x00video"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendStream("plugin\x00video", []byte("video-")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendStream("plugin\x00video", []byte("bytes")); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteStream("plugin\x00video"); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "capture.bin")
	if err := store.CopyComplete(context.Background(), "plugin\x00video", destination, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "video-bytes" {
		t.Fatalf("capture is %q", raw)
	}
}
