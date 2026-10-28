package resource

import (
	"net/http"
	"net/http/httptest"
	"os"
	"res-downloader/internal/config"
	downloadengine "res-downloader/internal/download"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"testing"
	"time"
)

func TestResourceModelMergesTracksByGroup(t *testing.T) {
	video := shared.ResourceCandidate{
		GroupKey: "item-1", Kind: "media.video", Title: "One",
		Tracks:         []shared.ResourceTrack{{ID: "video", Role: "video", URL: "https://cdn.example/video.mp4"}},
		RequiredTracks: []string{"video", "audio"},
		Capabilities:   []string{shared.ResourceCapabilityDownload},
		Source:         shared.ResourceSource{PluginID: "example.plugin"},
	}
	if err := validateCandidate(&video); err != nil {
		t.Fatal(err)
	}
	if video.State != shared.ResourceStatePartial {
		t.Fatalf("initial state = %q", video.State)
	}
	audio := shared.ResourceCandidate{
		GroupKey: "item-1", Kind: "media.video",
		Tracks:         []shared.ResourceTrack{{ID: "audio", Role: "audio", URL: "https://cdn.example/audio.m4a"}},
		RequiredTracks: []string{"video", "audio"},
		Source:         shared.ResourceSource{PluginID: "example.plugin"},
	}
	if err := validateCandidate(&audio); err != nil {
		t.Fatal(err)
	}
	merged := plugin.MergeResourceCandidate(video, audio)
	if len(merged.Tracks) != 2 || merged.State != shared.ResourceStateReady {
		t.Fatalf("merged resource = %#v", merged)
	}
}

func TestCaptureFileTrackIsReadyWithoutRemoteURL(t *testing.T) {
	candidate := shared.ResourceCandidate{
		GroupKey: "captured-video", Kind: "media.video",
		Tracks: []shared.ResourceTrack{{
			ID: "video", Role: "video", Executor: "capture-file", CaptureKey: "example:video", Extension: ".mp4",
		}},
		RequiredTracks: []string{"video"}, Capabilities: []string{"download"},
	}
	if err := validateCandidate(&candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.State != shared.ResourceStateReady {
		t.Fatalf("capture resource state = %q", candidate.State)
	}
}

func TestResourceMainTypeTraitsAndLifecycleAreDerived(t *testing.T) {
	candidate := shared.ResourceCandidate{
		Kind: "site.custom-video", GroupKey: "item",
		Tracks: []shared.ResourceTrack{
			{ID: "video", Role: "video", URL: "https://cdn.example/video.mp4", MIME: "video/mp4", Extension: ".mp4"},
			{ID: "audio", Role: "audio", URL: "https://cdn.example/audio.m4a", MIME: "audio/mp4", Extension: ".m4a", Processors: []shared.DownloadStep{{Type: "xor-prefix", Options: map[string]interface{}{"key": "AQ=="}}}},
		},
		RequiredTracks: []string{"video", "audio"}, Capabilities: []string{"download", "preview"},
	}
	if err := validateCandidate(&candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.PrimaryType != shared.ResourceTypeOther {
		t.Fatalf("primaryType=%q", candidate.PrimaryType)
	}
	for _, trait := range []string{shared.ResourceTraitMultiTrack, shared.ResourceTraitMergeRequired, shared.ResourceTraitEncrypted, shared.ResourceTraitDownloadable, shared.ResourceTraitPreviewable} {
		if !containsString(candidate.Traits, trait) {
			t.Fatalf("missing trait %q in %#v", trait, candidate.Traits)
		}
	}
	if candidate.Lifecycle.SchemaVersion != shared.ResourceSchemaVersion || candidate.Lifecycle.DiscoveredAt <= 0 || candidate.Lifecycle.Availability != shared.ResourceAvailabilityAvailable {
		t.Fatalf("lifecycle=%#v", candidate.Lifecycle)
	}
	expired := candidate
	expired.Lifecycle.ExpiresAt = time.Now().Add(-time.Minute).UnixMilli()
	expired.Lifecycle.Availability = ""
	normalizeResourceModel(&expired, time.Now())
	if expired.Lifecycle.Availability != shared.ResourceAvailabilityNeedsRefresh {
		t.Fatalf("expired lifecycle=%#v", expired.Lifecycle)
	}
}

func TestLiveStreamDerivesMainTypeAndTraits(t *testing.T) {
	candidate := shared.ResourceCandidate{
		Kind: "stream.live",
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: "video", Executor: "http-file",
			URL: "https://cdn.example/live.flv", MIME: "video/x-flv", Extension: ".flv",
		}},
	}
	normalizeResourceModel(&candidate, time.Now())
	if candidate.PrimaryType != shared.ResourceTypeVideo {
		t.Fatalf("primaryType=%q", candidate.PrimaryType)
	}
	for _, trait := range []string{shared.ResourceTraitLive, shared.ResourceTraitStreaming} {
		if !containsString(candidate.Traits, trait) {
			t.Fatalf("missing trait %q in %#v", trait, candidate.Traits)
		}
	}
}

func TestResourceKindFilterKeepsCollectionWithSelectedChildren(t *testing.T) {
	parent := shared.ResourceCandidate{
		GroupKey: "post", Kind: shared.ResourceKindCollection,
		Source: shared.ResourceSource{PluginID: "example.plugin"},
	}
	image := shared.ResourceCandidate{
		GroupKey: "post:image", ParentGroupKey: "post", Kind: "media.image",
		Source: shared.ResourceSource{PluginID: "example.plugin"},
	}
	audio := shared.ResourceCandidate{
		GroupKey: "post:audio", ParentGroupKey: "post", Kind: "media.audio",
		Source: shared.ResourceSource{PluginID: "example.plugin"},
	}
	candidates := []shared.ResourceCandidate{parent, image, audio}

	imagesOnly := &Resource{resType: map[string]bool{
		"all": false, shared.ResourceKindCollection: false, "media.image": true, "media.audio": false,
	}}
	filtered := imagesOnly.filterSelectedCandidates(candidates)
	if len(filtered) != 2 || filtered[0].Kind != shared.ResourceKindCollection || filtered[1].Kind != "media.image" {
		t.Fatalf("image filter = %#v", filtered)
	}

	collectionsOnly := &Resource{resType: map[string]bool{
		"all": false, shared.ResourceKindCollection: true, "media.image": false, "media.audio": false,
	}}
	filtered = collectionsOnly.filterSelectedCandidates(candidates)
	if len(filtered) != 3 {
		t.Fatalf("collection filter = %#v", filtered)
	}

	videoOnly := &Resource{resType: map[string]bool{
		"all": false, shared.ResourceKindCollection: false, "media.image": false, "media.audio": false, "media.video": true,
	}}
	if filtered = videoOnly.filterSelectedCandidates(candidates); len(filtered) != 0 {
		t.Fatalf("video filter retained an empty collection: %#v", filtered)
	}
}

func TestResourceKindFilterMatchesPrimaryOrDetailedKind(t *testing.T) {
	candidates := []shared.ResourceCandidate{
		{ID: "file", Kind: "media.video", PrimaryType: shared.ResourceTypeVideo},
		{ID: "hls", Kind: "stream.hls", PrimaryType: shared.ResourceTypeVideo},
		{ID: "live", Kind: "stream.live", PrimaryType: shared.ResourceTypeVideo},
	}

	videoOnly := &Resource{resType: map[string]bool{
		"all": false, shared.ResourceTypeVideo: true, "stream.hls": false, "stream.live": false,
	}}
	if filtered := videoOnly.filterSelectedCandidates(candidates); len(filtered) != 3 {
		t.Fatalf("video filter should include every video kind: %#v", filtered)
	}

	hlsOnly := &Resource{resType: map[string]bool{
		"all": false, shared.ResourceTypeVideo: false, "stream.hls": true, "stream.live": false,
	}}
	filtered := hlsOnly.filterSelectedCandidates(candidates)
	if len(filtered) != 1 || filtered[0].ID != "hls" {
		t.Fatalf("HLS kind filter = %#v", filtered)
	}

	streams := &Resource{resType: map[string]bool{
		"all": false, shared.ResourceTypeVideo: false, "stream.hls": true, "stream.live": true,
	}}
	filtered = streams.filterSelectedCandidates(candidates)
	if len(filtered) != 2 || filtered[0].ID != "hls" || filtered[1].ID != "live" {
		t.Fatalf("combined stream filters = %#v", filtered)
	}
}

func TestMultiInputDownloadPlanConcat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := "A"
		if request.URL.Path == "/b" {
			body = "B"
		}
		writer.Header().Set("Content-Length", "1")
		if request.Method != http.MethodHead {
			_, _ = writer.Write([]byte(body))
		}
	}))
	defer server.Close()

	runner := downloadengine.NewPlanRunner(nil, &config.Config{UseHeaders: "default", UserAgent: "res-downloader-test"}, logging.New(false, ""), nil, nil)
	path, err := runner.Run(shared.DownloadPlan{
		Inputs: []shared.DownloadInput{
			{ID: "a", Executor: "http-file", URL: server.URL + "/a"},
			{ID: "b", Executor: "http-file", URL: server.URL + "/b"},
		},
		Pipeline: []shared.PipelineStep{{ID: "joined", Executor: "builtin.concat", Inputs: []string{"a", "b"}}},
		Output:   shared.DownloadOutput{Input: "joined", Extension: ".bin"},
	}, t.TempDir(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "AB" {
		t.Fatalf("concat output = %q", data)
	}
}
