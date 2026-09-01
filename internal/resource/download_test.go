package resource

import (
	"context"
	"os"
	"path/filepath"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"sync"
	"testing"
)

type synchronizedCaptureSource struct {
	contents map[string][]byte
	ready    chan struct{}
	release  chan struct{}
}

func (s *synchronizedCaptureSource) CopyComplete(ctx context.Context, key, destination string, progress func(int64, int64)) error {
	select {
	case s.ready <- struct{}{}:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	select {
	case <-s.release:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	content := s.contents[key]
	if err := os.WriteFile(destination, content, 0600); err != nil {
		return err
	}
	if progress != nil {
		progress(int64(len(content)), int64(len(content)))
	}
	return nil
}

func TestConcurrentDownloadsReserveDistinctDestinationPaths(t *testing.T) {
	directory := t.TempDir()
	captures := &synchronizedCaptureSource{
		contents: map[string][]byte{"first": []byte("first file"), "second": []byte("a different second file")},
		ready:    make(chan struct{}),
		release:  make(chan struct{}),
	}
	resources := &Resource{
		config: &config.Config{
			FilenameTemplate: "resource.{{ext}}",
			FilenameConflict: "rename",
			TaskNumber:       1,
		},
		logger:   logging.New(false, ""),
		captures: captures,
		outputs:  make(map[string]struct{}),
	}

	type result struct {
		path string
		err  error
	}
	results := make(chan result, 2)
	var downloads sync.WaitGroup
	for _, item := range []struct {
		id         string
		captureKey string
	}{
		{id: "first-resource", captureKey: "first"},
		{id: "second-resource", captureKey: "second"},
	} {
		item := item
		downloads.Add(1)
		go func() {
			defer downloads.Done()
			candidate := shared.ResourceCandidate{ID: item.id}
			plan := shared.DownloadPlan{
				Inputs: []shared.DownloadInput{{ID: "input", Executor: "capture-file", CaptureKey: item.captureKey}},
				Output: shared.DownloadOutput{Input: "input", Extension: ".bin"},
			}
			path, err := resources.runDownloadPlanContext(context.Background(), candidate, plan, directory, shared.DownloadExecution{
				TaskID:  item.id,
				WorkDir: filepath.Join(directory, ".tasks", item.id),
			})
			results <- result{path: path, err: err}
		}()
	}

	<-captures.ready
	<-captures.ready
	close(captures.release)
	downloads.Wait()
	close(results)

	paths := make(map[string]struct{}, 2)
	contents := make(map[string]struct{}, 2)
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		paths[filepath.Base(result.path)] = struct{}{}
		content, err := os.ReadFile(result.path)
		if err != nil {
			t.Fatal(err)
		}
		contents[string(content)] = struct{}{}
	}
	if _, exists := paths["resource.bin"]; !exists {
		t.Fatalf("paths = %v, expected resource.bin", paths)
	}
	if _, exists := paths["resource(1).bin"]; !exists {
		t.Fatalf("paths = %v, expected resource(1).bin", paths)
	}
	if len(contents) != 2 {
		t.Fatalf("downloaded contents = %v, expected both files", contents)
	}
}
