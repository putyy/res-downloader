package plugin

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestJavaScriptHookAPILogging(t *testing.T) {
	for _, test := range []struct {
		name     string
		legacy   bool
		logger   bool
		settings map[string]interface{}
		wantLogs bool
	}{
		{name: "logging", logger: true, settings: map[string]interface{}{"enableLog": true}, wantLogs: true},
		{name: "missing setting", logger: true},
		{name: "disabled", logger: true, settings: map[string]interface{}{"enableLog": false}},
		{name: "string true", logger: true, settings: map[string]interface{}{"enableLog": "true"}},
		{name: "numeric true", logger: true, settings: map[string]interface{}{"enableLog": 1}},
		{name: "without logger", settings: map[string]interface{}{"enableLog": true}},
		{name: "legacy single argument", legacy: true, logger: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			script := `
function onObservation(observation, api) {
  observation.settings = {enableLog: true};
  api.log("observation");
  return {decision: "continue"};
}
function onPageMessage(message, context, api) {
  context.settings = {enableLog: true};
  api.log("page");
  return {};
}
function checkAPI(api, hook) {
  if (Object.keys(api).sort().join(",") !== "log,pluginVersion") {
    throw new Error("unexpected hook capabilities");
  }
  if (api.pluginVersion !== "1.2.3") throw new Error("incorrect version");
  api.log(hook);
}
function createDownloadPlan(input, api) {
  input.options.settings = {enableLog: true};
  checkAPI(api, "plan");
  return {inputs: [{id: "video", executor: "http-file", url: input.resource.tracks[0].url}],
          output: {input: "video", extension: ".mp4"}};
}
function refreshResource(input, api) {
  input.options.settings = {enableLog: true};
  checkAPI(api, "refresh");
  return {status: "refreshed", resource: input.resource};
}`
			if test.legacy {
				script = strings.ReplaceAll(script, "input, api", "input")
				script = strings.ReplaceAll(script, `checkAPI(api, "plan");`, "")
				script = strings.ReplaceAll(script, `checkAPI(api, "refresh");`, "")
			}
			if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			var logs bytes.Buffer
			services := pluginRuntimeServices{}
			if test.logger {
				services.logger = &Logger{Logger: zerolog.New(&logs)}
			}
			manifest := shared.PluginManifest{
				ID: "test.hook-api", Name: "Hook API", Version: "1.2.3",
				APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js",
				// Even with page-bridge permission, resource hooks receive only the base API.
				Permissions: shared.PluginPermissions{Capabilities: []string{"inject-page-script", "page-bridge"}},
			}
			runtime, err := newJavaScriptPlugin(directory, manifest, services)
			if err != nil {
				t.Fatal(err)
			}
			resource := shared.ResourceCandidate{
				GroupKey: "demo", Kind: "media.video",
				Source: shared.ResourceSource{PluginID: manifest.ID},
				Tracks: []shared.ResourceTrack{{ID: "video", Role: "video", URL: "https://example.com/demo.mp4"}},
			}
			if _, err := runtime.Handle(context.Background(), shared.Observation{Settings: test.settings}); err != nil {
				t.Fatalf("observation: %v", err)
			}
			if _, called, err := runtime.(*javaScriptPlugin).HandlePageMessage(context.Background(), nil, shared.PageMessageContext{Settings: test.settings}); err != nil || !called {
				t.Fatalf("page message: called=%v, err=%v", called, err)
			}
			options := shared.DownloadOptions{Settings: test.settings}
			plan, called, err := runtime.Resolve(context.Background(), resource, options)
			if err != nil || !called {
				t.Fatalf("resolve: called=%v, err=%v", called, err)
			}
			if len(plan.Inputs) != 1 || plan.Inputs[0].URL != resource.Tracks[0].URL || plan.Output.Input != "video" {
				t.Fatalf("unexpected plan: %#v", plan)
			}
			refreshed, called, err := runtime.(shared.ResourceRefresher).RefreshResource(context.Background(), resource, options)
			if err != nil || !called {
				t.Fatalf("refresh: called=%v, err=%v", called, err)
			}
			if refreshed.Status != "refreshed" || refreshed.Resource.GroupKey != resource.GroupKey {
				t.Fatalf("unexpected refresh: %#v", refreshed)
			}
			if test.wantLogs {
				for _, message := range []string{"plugin test.hook-api: observation", "plugin test.hook-api: page", "plugin test.hook-api: plan", "plugin test.hook-api: refresh"} {
					if !strings.Contains(logs.String(), message) {
						t.Fatalf("missing log %q: %s", message, logs.String())
					}
				}
			} else if logs.Len() != 0 {
				t.Fatalf("unexpected logs: %s", logs.String())
			}
		})
	}
}
