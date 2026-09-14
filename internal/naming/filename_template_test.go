package naming

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestFilenameTemplateOptionalResourceTimes(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	created := time.Date(2026, 9, 1, 18, 2, 3, 0, time.UTC).UnixMilli()
	published := created + 24*60*60*1000
	metadata := map[string]interface{}{"createdAt": created, "publishedAt": published}
	// Persistence decodes JSON numbers as float64; formatting must survive it.
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var restored map[string]interface{}
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	for _, fields := range []map[string]interface{}{metadata, restored} {
		got, err := ExpandFilenameTemplate("{{created_at:2006-01-02_15-04-05}}_{{published_at}}_{{date}}", nil, fields, now)
		if err != nil || got != "2026-09-02_02-02-03_20260903_20260914" {
			t.Fatalf("time formatting = %q, %v", got, err)
		}
		path, err := RenderResourcePath(t.TempDir(), "{{title|sanitize}}{{created_at|prefix:_}}.{{ext}}",
			shared.ResourceCandidate{Title: "旅行记录", Metadata: fields},
			shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}, now)
		if err != nil || filepath.Base(path) != "旅行记录_20260902.mp4" {
			t.Fatalf("dated filename = %q, %v", path, err)
		}
	}
	for _, raw := range []interface{}{nil, "1788285723000", "2026-09-02", true, 0, -1, 1.5, math.NaN(), math.Inf(1), int64(253402300800000), map[string]interface{}{}} {
		resource := shared.ResourceCandidate{Title: "旅行记录", Metadata: map[string]interface{}{"createdAt": raw, "publishedAt": raw}}
		path, err := RenderResourcePath(t.TempDir(), "{{title}}{{created_at|prefix:_}}{{published_at|prefix:_|suffix:_}}.{{ext}}", resource,
			shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}, now)
		if err != nil || filepath.Base(path) != "旅行记录.mp4" {
			t.Fatalf("invalid timestamp %#v: path = %q, %v", raw, path, err)
		}
	}
}

func TestFilenameTemplateConditionalAffixes(t *testing.T) {
	for _, tc := range []struct {
		template string
		value    string
		want     string
	}{
		{"{{title|prefix:_|suffix:-}}", "", ""},
		{"{{title|prefix:_|suffix:-}}", " \t ", ""},
		{"{{title|prefix:_|suffix:-}}", "标题", "_标题-"},
		{"{{title|sanitize|prefix:_}}", "...", ""},
		{"{{title|default:resource|prefix:_}}", "", "_resource"},
		{"{{title|prefix:_|default:resource}}", "", "resource"},
		{"{{title|prefix:https://}}", "example", "https://example"},
	} {
		got, err := ExpandFilenameTemplate(tc.template, map[string]string{"title": tc.value}, nil, time.Now())
		if err != nil || got != tc.want {
			t.Fatalf("%s with %q = %q, %v; want %q", tc.template, tc.value, got, err, tc.want)
		}
	}
}

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
	if got := SanitizeFilenameSegment("a＜b＞c：d＂e／f＼g｜h？i＊j.mp4"); got != "a_b_c_d_e_f_g_h_i_j.mp4" {
		t.Fatalf("sanitized full-width name = %q", got)
	}
}

func TestRenderResourcePathLimitsUnicodeFilenameBytes(t *testing.T) {
	plan := shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}
	for _, title := range []string{
		strings.Repeat("中", 80),
		strings.Repeat("😀", 80),
	} {
		path, err := RenderResourcePath(t.TempDir(), "", shared.ResourceCandidate{Title: title}, plan, time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		if len(name) > MaxFilenameSegmentBytes {
			t.Fatalf("filename uses %d bytes: %q", len(name), name)
		}
		if !utf8.ValidString(name) {
			t.Fatalf("filename is not valid UTF-8: %q", name)
		}
		if !strings.HasSuffix(name, ".mp4") {
			t.Fatalf("filename lost extension: %q", name)
		}
	}
}

func TestTruncateFilenameSegmentPreservesExtension(t *testing.T) {
	name := TruncateFilenameSegment(strings.Repeat("中文", 40)+".mp4", 40)
	if len(name) > 40 || !utf8.ValidString(name) || !strings.HasSuffix(name, ".mp4") {
		t.Fatalf("truncated filename = %q (%d bytes)", name, len(name))
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
