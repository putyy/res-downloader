package plugin

import (
	"context"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin/native"
	"strings"
	"testing"
	"time"
)

func TestDeclarativePluginFixture(t *testing.T) {
	directory := filepath.Join("testdata", "plugins", "declarative")
	manifest, err := ValidatePluginDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "test.declarative" {
		t.Fatalf("manifest id = %q", manifest.ID)
	}
	if err := ReplayPluginFixture(directory, filepath.Join(directory, "fixture.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestExternalPluginCannotUseOfficialPrefix(t *testing.T) {
	directory := t.TempDir()
	manifest := `{
  "id":"Official.example","name":"Impersonation","version":"1.0.0","apiVersion":1,
  "runtime":"javascript","entry":"main.js",
  "permissions":{"domains":["example.com"],"capabilities":[]},"match":[]
}`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte("function onObservation() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadExternalPlugin(directory); err == nil || !strings.Contains(err.Error(), "official.") {
		t.Fatalf("expected reserved official prefix error, got %v", err)
	}
}

func TestBundledPluginRejectsModifiedContent(t *testing.T) {
	root := t.TempDir()
	if err := installBundledPlugins(root); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "official.wechat")
	entry := filepath.Join(directory, "main.js")
	content, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\n// modified after distribution\n")...)
	if err := os.WriteFile(entry, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadBundledPlugin(directory); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected modified bundled plugin to be rejected, got %v", err)
	}
}

func TestJavaScriptPluginTopLevelTimeout(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte("while (true) {}"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := shared.PluginManifest{ID: "test.timeout", Name: "Timeout", Version: "1", APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js"}
	runtime, err := newJavaScriptPlugin(directory, manifest)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = runtime.Handle(context.Background(), shared.Observation{})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("JavaScript timeout took too long")
	}
}

func TestJavaScriptCorrelationFindUsesJSONFieldNames(t *testing.T) {
	directory := t.TempDir()
	script := `function onObservation(observation, api) {
  if (observation.request.url === "https://example.com/register") {
    api.correlate.register({
      groupKey: "item-1",
      trackId: "audio-1",
      role: "audio",
      aliases: ["https://cdn.example/audio.m4a"]
    });
    return {decision: "continue"};
  }
  var refs = api.correlate.find("https://cdn.example/audio.m4a");
  return {
    decision: "continue",
    diagnostics: [
      refs[0].groupKey,
      refs[0].trackId,
      refs[0].role,
      typeof refs[0].GroupKey
    ]
  };
}`
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := shared.PluginManifest{
		ID: "test.correlation-json-fields", Name: "Correlation fields", Version: "1",
		APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js",
	}
	runtime, err := newJavaScriptPlugin(directory, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Handle(context.Background(), shared.Observation{
		Request: shared.RequestSnapshot{URL: "https://example.com/register"},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Handle(context.Background(), shared.Observation{
		Request: shared.RequestSnapshot{URL: "https://example.com/find"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"item-1", "audio-1", "audio", "undefined"}
	if len(result.Diagnostics) != len(want) {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for index := range want {
		if result.Diagnostics[index] != want[index] {
			t.Fatalf("diagnostics[%d] = %q, want %q", index, result.Diagnostics[index], want[index])
		}
	}
}

func TestJavaScriptEntryCannotEscapeThroughSymlink(t *testing.T) {
	root := t.TempDir()
	pluginDirectory := filepath.Join(root, "plugin")
	if err := os.Mkdir(pluginDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.js")
	if err := os.WriteFile(outside, []byte("function onObservation() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(pluginDirectory, "main.js")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := newJavaScriptPlugin(pluginDirectory, shared.PluginManifest{
		ID: "test.symlink", Name: "Symlink", Version: "1", APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js",
	})
	if err == nil {
		t.Fatal("expected JavaScript symlink escape to be rejected")
	}
}

func TestSanitizeObservationRemovesUnpermittedBodies(t *testing.T) {
	obs := shared.Observation{
		Stage:    shared.StageResponse,
		Request:  shared.RequestSnapshot{Body: "request-secret"},
		Response: &shared.ResponseSnapshot{Body: "response-secret"},
	}
	got := sanitizeObservation(obs, shared.PluginPermissions{Capabilities: []string{"observe-response"}})
	if got.Request.Body != "" || got.Response.Body != "" {
		t.Fatal("plugin received a body without a read-body capability")
	}
	if obs.Request.Body == "" || obs.Response.Body == "" {
		t.Fatal("sanitising mutated the shared observation")
	}
}

func TestObservationBodyIsLimitedPerPlugin(t *testing.T) {
	observation := shared.Observation{
		Request:  shared.RequestSnapshot{Body: "request-body"},
		Response: &shared.ResponseSnapshot{Body: "response-body"},
	}
	limitObservationBody(&observation, shared.PluginPermissions{BodyLimit: 4})
	if observation.Request.Body != "requ" || !observation.Request.Truncated ||
		observation.Response.Body != "resp" || !observation.Response.Truncated {
		t.Fatalf("observation was not limited: %#v", observation)
	}
}

func TestValidatePluginSettings(t *testing.T) {
	schema := map[string]interface{}{
		"properties": map[string]interface{}{
			"quality": map[string]interface{}{"type": "string", "enum": []interface{}{"high", "low"}},
			"enabled": map[string]interface{}{"type": "boolean"},
		},
	}
	if err := validatePluginSettings(schema, map[string]interface{}{"quality": "high", "enabled": true}); err != nil {
		t.Fatal(err)
	}
	if err := validatePluginSettings(schema, map[string]interface{}{"quality": "invalid"}); err == nil {
		t.Fatal("expected enum validation error")
	}
}

func TestValidateGenericDetectorSettings(t *testing.T) {
	manifest := (&native.DefaultPlugin{}).Manifest()
	valid := map[string]interface{}{"rules": []shared.CaptureRule{{
		ID: "custom", Enabled: true,
		Match: shared.CaptureRuleMatch{MIME: []string{"application/example"}},
		Resource: shared.CaptureRuleResource{
			Kind: "archive.example", Executor: "http-file",
		},
	}}}
	if err := validatePluginSettings(manifest.SettingsSchema, valid); err != nil {
		t.Fatal(err)
	}
	invalid := map[string]interface{}{"rules": []shared.CaptureRule{{
		ID: "catch-all", Enabled: true,
		Resource: shared.CaptureRuleResource{Kind: "stream.binary", Executor: "http-file"},
	}}}
	if err := validatePluginSettings(manifest.SettingsSchema, invalid); err == nil {
		t.Fatal("expected invalid capture rules to be rejected")
	}
}

func TestPluginFileExtensionCannotEscapeDownloadDirectory(t *testing.T) {
	for _, extension := range []string{"/../../outside", ".mp4/../../outside", "mp4", "."} {
		if validExtension(extension) {
			t.Errorf("extension %q should be rejected", extension)
		}
	}
	for _, extension := range []string{"", ".mp4", ".tar.gz"} {
		if !validExtension(extension) {
			t.Errorf("extension %q should be accepted", extension)
		}
	}
}

func TestJavaScriptPluginFixture(t *testing.T) {
	directory := filepath.Join("testdata", "plugins", "javascript")
	manifest, err := ValidatePluginDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "test.javascript" {
		t.Fatalf("manifest id = %q", manifest.ID)
	}
	if err := ReplayPluginFixture(directory, filepath.Join(directory, "fixture.json")); err != nil {
		t.Fatal(err)
	}
}

func TestFallbackDownloadPlanKeepsResourceProcessors(t *testing.T) {
	manager := &PluginManager{}
	processor := shared.DownloadStep{
		Type: "xor-prefix", Options: map[string]interface{}{"key": "AQI="},
	}
	plan, err := manager.Resolve(context.Background(), shared.ResourceCandidate{
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: "video", URL: "https://cdn.example/video.mp4", Processors: []shared.DownloadStep{processor},
		}},
	}, shared.DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Inputs[0].Processors) != 1 || plan.Inputs[0].Processors[0].Type != processor.Type {
		t.Fatalf("fallback processors = %#v", plan.Inputs[0].Processors)
	}
}

func TestInstallBundledPlugins(t *testing.T) {
	directory := t.TempDir()
	if err := installBundledPlugins(directory); err != nil {
		t.Fatal(err)
	}
	manifest, err := ValidateBundledPluginDirectory(filepath.Join(directory, "official.wechat"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "official.wechat" {
		t.Fatalf("installed plugin id = %q", manifest.ID)
	}
}

func TestWildcardMatch(t *testing.T) {
	cases := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"*.example.com", "api.example.com", true},
		{"*.example.com", "example.com", false},
		{"/api/*/video", "/api/v1/video", true},
		{"/api/*/video", "/other/v1/video", false},
	}
	for _, item := range cases {
		if got := wildcardMatch(item.pattern, item.value); got != item.want {
			t.Errorf("wildcardMatch(%q, %q) = %v, want %v", item.pattern, item.value, got, item.want)
		}
	}
}
