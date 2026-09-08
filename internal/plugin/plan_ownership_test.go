package plugin

import (
	"encoding/json"
	shared "res-downloader/internal/model"
	"testing"
)

func TestBindingFallbackPlanPreservesPublishedResourceAndPlan(t *testing.T) {
	manager := &PluginManager{statuses: map[string]shared.PluginStatus{
		"test.owner": {
			Digest: "current-digest",
			Manifest: shared.PluginManifest{
				Permissions: shared.PluginPermissions{Capabilities: []string{"process-download"}},
				Processors:  map[string]shared.PluginProcessorDefinition{"decrypt": {Runtime: "wasm"}},
			},
		},
	}}
	resource := shared.ResourceCandidate{Tracks: []shared.ResourceTrack{{
		ID: "video", URL: "https://example.com/video.mp4", Headers: map[string]string{"X-Resource": "original"},
		Processors: []shared.DownloadStep{{Type: wasmProcessorType, Options: map[string]interface{}{wasmProcessorIDOption: "decrypt"}}},
	}}}
	before, err := json.Marshal(resource)
	if err != nil {
		t.Fatal(err)
	}
	plan := fallbackDownloadPlan(resource, shared.DownloadOptions{})
	plan.Output.Processors = resource.Tracks[0].Processors
	publishedPlan := plan
	planBefore, err := json.Marshal(publishedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.bindDownloadPlanProcessors("test.owner", &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Inputs[0].Processors[0].Options[wasmProcessorOwnerKey] != "test.owner" || plan.Output.Processors[0].Options[wasmProcessorDigestKey] != "current-digest" {
		t.Fatal("plan processors were not bound")
	}
	plan.Inputs[0].Headers["X-Resource"] = "download-only"
	after, err := json.Marshal(resource)
	if err != nil || string(before) != string(after) {
		t.Fatalf("published resource changed: before=%s after=%s err=%v", before, after, err)
	}
	planAfter, err := json.Marshal(publishedPlan)
	if err != nil || string(planBefore) != string(planAfter) {
		t.Fatalf("published plan changed: before=%s after=%s err=%v", planBefore, planAfter, err)
	}
}
