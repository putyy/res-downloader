package plugin

import (
	"context"
	shared "res-downloader/internal/model"
	"sync"
	"testing"
)

type handledTestPlugin struct {
	manifest shared.PluginManifest
	result   shared.PluginResult
	mu       sync.Mutex
	calls    int
}

func (p *handledTestPlugin) Manifest() shared.PluginManifest { return p.manifest }
func (p *handledTestPlugin) Handle(context.Context, shared.Observation) (shared.PluginResult, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return p.result, nil
}
func (p *handledTestPlugin) Resolve(context.Context, shared.ResourceCandidate, shared.DownloadOptions) (shared.DownloadPlan, bool, error) {
	return shared.DownloadPlan{}, false, nil
}

type collectingResourceSink struct {
	resources []shared.ResourceCandidate
}

func (s *collectingResourceSink) FilterSelectedCandidates(candidates []shared.ResourceCandidate) []shared.ResourceCandidate {
	return candidates
}
func (s *collectingResourceSink) PublishCandidate(candidate shared.ResourceCandidate) {
	s.resources = append(s.resources, candidate)
}
func (s *collectingResourceSink) PublishCandidates(candidates []shared.ResourceCandidate) {
	s.resources = append(s.resources, candidates...)
}
func (s *collectingResourceSink) CandidateByGroup(pluginID, groupKey string) (shared.ResourceCandidate, bool) {
	for index := len(s.resources) - 1; index >= 0; index-- {
		candidate := s.resources[index]
		if candidate.Source.PluginID == pluginID && candidate.GroupKey == groupKey {
			return candidate, true
		}
	}
	return shared.ResourceCandidate{}, false
}
func (s *collectingResourceSink) RegisterTypes([]string) {}

func TestHandledExternalPluginSkipsGenericFallback(t *testing.T) {
	permissions := shared.PluginPermissions{Domains: []string{"*"}, Capabilities: []string{"observe-response", "emit-resource"}}
	external := &handledTestPlugin{
		manifest: shared.PluginManifest{ID: "example.hls", Enabled: nil, Priority: 100, Permissions: permissions},
		result: shared.PluginResult{Handled: true, Resources: []shared.ResourceCandidate{{
			Kind: "stream.custom", Tracks: []shared.ResourceTrack{{ID: "primary", URL: "https://cdn.example/live.m3u8"}},
			Capabilities: []string{shared.ResourceCapabilityDownload},
		}}},
	}
	generic := &handledTestPlugin{
		manifest: shared.PluginManifest{ID: "builtin.generic-detector", Enabled: nil, Priority: -1000, Permissions: permissions},
		result: shared.PluginResult{Resources: []shared.ResourceCandidate{{
			Kind: "stream.hls", Tracks: []shared.ResourceTrack{{ID: "primary", URL: "https://cdn.example/live.m3u8"}},
			Capabilities: []string{shared.ResourceCapabilityDownload},
		}}},
	}
	sink := &collectingResourceSink{}
	manager := &PluginManager{
		resources: sink,
		plugins:   []managedPlugin{{runtime: external}, {runtime: generic, builtin: true}},
		statuses: map[string]shared.PluginStatus{
			"example.hls": {Digest: "external"}, "builtin.generic-detector": {Digest: "builtin"},
		},
		settings: make(map[string]map[string]interface{}), runtimeStates: make(map[string]*pluginRuntimeState),
	}
	result := manager.Process(context.Background(), shared.Observation{
		Stage:    shared.StageResponse,
		Request:  shared.RequestSnapshot{URL: "https://cdn.example/live.m3u8", Host: "cdn.example", Path: "/live.m3u8"},
		Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "application/vnd.apple.mpegurl"},
	})
	if !result.Handled || len(sink.resources) != 1 || sink.resources[0].Source.PluginID != "example.hls" {
		t.Fatalf("result=%#v resources=%#v", result, sink.resources)
	}
	generic.mu.Lock()
	defer generic.mu.Unlock()
	if generic.calls != 0 {
		t.Fatalf("generic fallback was called %d times", generic.calls)
	}
}
