package naming

import (
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
	"time"
)

func TestRenderResourcePathWithMagicVariables(t *testing.T) {
	resource := shared.ResourceCandidate{
		ID: "item-1", Kind: "media.video", Title: "A: title?", Source: shared.ResourceSource{PluginID: "official.example"},
		Metadata: map[string]interface{}{"douyin.author": "Alice"},
		Tracks:   []shared.ResourceTrack{{ID: "1080p", Role: "video", URL: "https://cdn.example/video", Quality: "1080p"}},
	}
	plan := shared.DownloadPlan{
		Inputs: []shared.DownloadInput{{ID: "1080p", Executor: "http-file", URL: "https://cdn.example/video"}},
		Output: shared.DownloadOutput{Input: "1080p", Extension: ".mp4"},
	}
	path, err := RenderResourcePath(t.TempDir(), "{{meta.douyin.author}}/{{title|sanitize}}_{{quality}}_{{date:20060102}}.{{ext}}", resource, plan, time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("Alice", "A_ title__1080p_20260815.mp4")
	if !strings.HasSuffix(path, wantSuffix) {
		t.Fatalf("path = %q, want suffix %q", path, wantSuffix)
	}
}

func TestFilenameTemplateRejectsTraversalAndUnknownFilters(t *testing.T) {
	resource := shared.ResourceCandidate{Title: "../secret"}
	plan := shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".bin"}}
	if _, err := RenderResourcePath(t.TempDir(), "{{title}}", resource, plan, time.Now()); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if _, err := ExpandFilenameTemplate("{{title|execute}}", map[string]string{"title": "x"}, nil, time.Now()); err == nil {
		t.Fatal("expected unknown filter to be rejected")
	}
}

func TestSanitizeFilenameSegmentHandlesPortableNames(t *testing.T) {
	if got := SanitizeFilenameSegment("CON.txt"); got != "_CON.txt" {
		t.Fatalf("reserved name = %q", got)
	}
	if got := SanitizeFilenameSegment("a<b>c?.mp4"); got != "a_b_c_.mp4" {
		t.Fatalf("sanitized name = %q", got)
	}
}

func TestResolveFilenameConflictIncludesInFlightPaths(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "resource.mp4")
	if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	inFlight := filepath.Join(directory, "resource(1).mp4")
	resolved, err := ResolveFilenameConflictWith(path, "rename", func(candidate string) bool {
		return candidate == inFlight
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(directory, "resource(2).mp4"); resolved != want {
		t.Fatalf("resolved path = %q, expected %q", resolved, want)
	}
	if _, err := ResolveFilenameConflictWith(inFlight, "skip", func(candidate string) bool {
		return candidate == inFlight
	}); err == nil {
		t.Fatal("expected in-flight destination to be skipped")
	}
}
