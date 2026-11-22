package naming

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
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

func TestFilenameTemplateRegexReplacement(t *testing.T) {
	for _, tc := range []struct {
		name     string
		template string
		value    string
		want     string
	}{
		{"letters and numbers", `{{title|regex_replace:[^\p{L}\p{N}]:|default:resource|sanitize|truncate:80}}.{{ext}}`, "春天🌸#记录❤️2026", "春天记录2026.mp4"},
		{"remove hashes only", `{{title|regex_replace:#:}}`, "#春天🌸#记录", "春天🌸记录"},
		{"selected symbols", `{{title|regex_replace:[#❤♥🌸🌹\x{FE0F}]:}}`, "春天🌸#记录❤️", "春天记录"},
		{"keep separators", `{{title|regex_replace:[^\p{L}\p{N} _-]:}}`, "春天 🌸记录-2026_1", "春天 记录-2026_1"},
		{"collapse separators", `{{title|regex_replace:[^\p{L}\p{N}]+:_}}`, "春天 #🌸 记录", "春天_记录"},
		{"empty result fallback", `{{title|regex_replace:[^\p{L}\p{N}]:|default:resource}}`, "#❤️🌸", "resource"},
		{"missing value fallback", `{{author|regex_replace:#:|default:unknown}}`, "", "unknown"},
		{"chain replacements", `{{title|regex_replace:#:|regex_replace:🌸:flower|upper}}`, "#spring🌸", "SPRINGFLOWER"},
		{"quoted alternation", "{{title|regex_replace:`cat|dog`:pet}}", "cat and dog", "pet and pet"},
		{"quoted colons", "{{title|regex_replace:`a:b`:`x:y`}}", "a:b", "x:y"},
		{"quoted delimiters", "{{title|regex_replace:`[|}]`:`|}}`}}", "a|b}", "a|}}b|}}"},
		{"quoted quantifier", "{{title|regex_replace:`a{2}`:b}}", "aaaa", "bb"},
		{"capture replacement", `{{title|regex_replace:(\p{L}+)-(\d+):"${2}_${1}"}}`, "video-12", "12_video"},
		{"literal dollar", `{{title|regex_replace:#:$$}}`, "#title", "$title"},
		{"double quoted escapes", `{{title|regex_replace:"\\s+":"_"}}`, "a\t b", "a_b"},
		{"quoted replacement spacing", `{{title|regex_replace:#:" " |upper}}`, "a#b", "A B"},
		{"quoted backtick", "{{title|regex_replace:\"`\":}}", "a`b", "ab"},
		{"legacy literal quotes", "{{title|prefix:`|suffix:\"}}", "title", "`title\""},
		{"legacy literal backslashes", `{{title|prefix:\p{L}_}}`, "title", `\p{L}_title`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExpandFilenameTemplate(tc.template, map[string]string{"title": tc.value, "ext": "mp4"}, nil, time.Now())
			if err != nil || got != tc.want {
				t.Fatalf("%s with %q = %q, %v; want %q", tc.template, tc.value, got, err, tc.want)
			}
		})
	}
}

func TestFilenameTemplateRegexValidation(t *testing.T) {
	for _, template := range []string{
		`{{title|regex_replace:[^\p{L}\p{N}]:|default:resource|sanitize}}.{{ext}}`,
		`{{title|regex_replace:^$:/}}.{{ext}}`,
		"{{title|regex_replace:`cat|dog`:`pet:friend`|sanitize}}.{{ext}}",
		"{{title|regex_replace:`a{2}`:b}}.{{ext}}",
		`{{title|regex_replace:"\\s+":"_"}}.{{ext}}`,
	} {
		if err := ValidateFilenameTemplate(template); err != nil {
			t.Fatalf("valid regex template %q: %v", template, err)
		}
	}
	for _, template := range []string{
		`{{title|regex_replace}}.{{ext}}`,
		`{{title|regex_replace:#}}.{{ext}}`,
		`{{title|regex_replace::}}.{{ext}}`,
		`{{title|regex_replace:[:}}.{{ext}}`,
		`{{title|regex_replace:(?=a):}}.{{ext}}`,
		"{{title|regex_replace:`a:b`:x}}.{{ext}}/../file",
		"{{title|regex_replace:`unterminated:}}.{{ext}}",
		`{{title|regex_replace:"\p{L}":}}.{{ext}}`,
		`{{title|regex_replace:"a"x:b}}.{{ext}}`,
		`{{title|regex_replace:a:"b"x}}.{{ext}}`,
		`{{title|regex_replace:a:}}/../{{ext}}`,
		`{{title|default:x|regex_replace:x:../secret}}.{{ext}}`,
		`{{title|default:x|regex_replace:x:/secret}}.{{ext}}`,
	} {
		if err := ValidateFilenameTemplate(template); err == nil {
			t.Fatalf("invalid regex template %q was accepted", template)
		}
		if _, err := RenderResourcePath(t.TempDir(), template, shared.ResourceCandidate{Title: "x"},
			shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}, time.Now()); err == nil {
			t.Fatalf("invalid regex template %q was rendered", template)
		}
	}
}

func TestFilenameRegexLimits(t *testing.T) {
	full := strings.Repeat("a", maxFilenameRegexBytes)
	for _, tc := range []struct {
		value, pattern, replacement, want string
		wantErr                           error
	}{
		{full, "^(.+)$", "$1", full, nil},
		{full, "^(.+)$", "$1$1", "", errFilenameRegexSize},
		{full, "a$", "aa", "", errFilenameRegexSize},
		{full, "^", "x", "", errFilenameRegexSize},
		{full + "a", "a", "", "", errFilenameRegexSize},
		{full, strings.Repeat("()", 20) + "a", "$0", "", errFilenameRegexMatches},
	} {
		got, err := replaceFilenameRegex(regexp.MustCompile(tc.pattern), tc.value, tc.replacement)
		if got != tc.want || err != tc.wantErr {
			t.Errorf("pattern %q: got %d bytes, %v; want %d bytes, %v", tc.pattern, len(got), err, len(tc.want), tc.wantErr)
		}
	}
	pattern := regexp.MustCompile(`(?P<word>\w+)|$`)
	for _, replacement := range []string{"", "$", "$$$1", "${word}_$0", "$unknown$${1}", "x${bad$1}", "$$x$$$"} {
		const input = "a b!"
		got, err := replaceFilenameRegex(pattern, input, replacement)
		if err != nil || got != pattern.ReplaceAllString(input, replacement) {
			t.Errorf("replacement %q: got %q, %v", replacement, got, err)
		}
	}
	chain := strings.Repeat("|regex_replace:a:aa", 17)
	if err := ValidateFilenameTemplate("{{title|default:a" + chain + "|truncate:80}}"); err != errFilenameRegexSize {
		t.Fatalf("oversized default chain: %v", err)
	}
	if _, err := ExpandFilenameTemplate("{{title"+chain+"|truncate:80}}", map[string]string{"title": "a"}, nil, time.Now()); err != errFilenameRegexSize {
		t.Fatalf("oversized title chain: %v", err)
	}
}

func TestFilenameRegexResourceDependentDirectory(t *testing.T) {
	directory := t.TempDir()
	template := `{{height|regex_replace:\D:}}/{{title|sanitize}}.{{ext}}`
	resource := shared.ResourceCandidate{Title: "标题", Tracks: []shared.ResourceTrack{{ID: "video", Role: "video", Height: 1080}}}
	plan := shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}
	if err := ValidateFilenameTemplate(template); err != nil {
		t.Fatal(err)
	}
	path, err := RenderResourcePath(directory, template, resource, plan, time.Now())
	if err != nil || path != filepath.Join(directory, "1080", "标题.mp4") {
		t.Fatalf("height directory: %q, %v", path, err)
	}
	resource.Tracks = nil
	if _, err := RenderResourcePath(directory, template, resource, plan, time.Now()); err == nil {
		t.Fatal("missing height produced an absolute path")
	}
	for _, replacement := range []string{"../bad", "/bad", "C:/bad"} {
		template := "{{title|regex_replace:标题:" + replacement + "}}.{{ext}}"
		if err := ValidateFilenameTemplate(template); err != nil {
			t.Fatal(err)
		}
		if _, err := RenderResourcePath(directory, template, resource, plan, time.Now()); err == nil {
			t.Fatalf("unsafe runtime path from %q", replacement)
		}
	}
}

func TestRenderResourcePathWithRegexReplacement(t *testing.T) {
	directory := t.TempDir()
	template := `{{author|regex_replace:[^\p{L}\p{N}]:|default:unknown|sanitize}}/{{title|regex_replace:[^\p{L}\p{N}]:|default:resource|sanitize|truncate:80}}.{{ext}}`
	path, err := RenderResourcePath(directory, template,
		shared.ResourceCandidate{Title: "春天🌸#记录❤️2026", Metadata: map[string]interface{}{"author": "作者🌹"}},
		shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}, time.Now())
	if want := filepath.Join(directory, "作者", "春天记录2026.mp4"); err != nil || path != want {
		t.Fatalf("cleaned path = %q, %v; want %q", path, err, want)
	}
	if _, err := RenderResourcePath(directory, `{{title|regex_replace:x:../secret}}.{{ext}}`,
		shared.ResourceCandidate{Title: "x"}, shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}, time.Now()); err == nil {
		t.Fatal("expected a traversal introduced by regex replacement to be rejected")
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

func TestValidateFilenameTemplateBeforeSaving(t *testing.T) {
	for _, template := range []string{
		"", "{{meta.site.author}}/{{title}}.{{ext}}",
		"{{created_at}}/{{meta.createdAt}}.{{ext}}",
		"{{date:2006/01}}/{{title|default:resource}}.{{ext}}",
		"archive{{date:/2006/01}}/{{title|sanitize}}.{{ext}}",
		"archive{{title|default:/resource}}.{{ext}}",
		"{{title|default:..}}.{{ext}}",
	} {
		if err := ValidateFilenameTemplate(template); err != nil {
			t.Fatalf("valid template %q: %v", template, err)
		}
	}
	for _, template := range []string{
		"/{{title}}.{{ext}}", "../{{title}}.{{ext}}",
		"C:/{{title}}.{{ext}}",
		"{{title|default:..}}/{{id}}.{{ext}}", "{{title|truncate:0}}.{{ext}}",
		"{{title|default:C:/resource}}.{{ext}}",
		"{{title|default:/resource}}.{{ext}}",
		"{{title|default:.}}{{author|default:.}}/{{id}}.{{ext}}",
		strings.Repeat("a", 4097),
	} {
		if err := ValidateFilenameTemplate(template); err == nil {
			t.Fatalf("invalid template %q was accepted", template)
		}
	}
}

func TestRenderResourcePathWithPrefixedDateDirectory(t *testing.T) {
	directory := t.TempDir()
	path, err := RenderResourcePath(directory, "archive{{date:/2006/01}}/{{title|sanitize}}.{{ext}}",
		shared.ResourceCandidate{Title: "resource"},
		shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}},
		time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(directory, "archive", "2026", "01", "resource.mp4"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestRenderResourcePathInsideFilesystemRoot(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	path, err := RenderResourcePath(root, "{{title}}.{{ext}}",
		shared.ResourceCandidate{Title: "resource"},
		shared.DownloadPlan{Output: shared.DownloadOutput{Extension: ".mp4"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "resource.mp4"); path != want {
		t.Fatalf("root path = %q, want %q", path, want)
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
