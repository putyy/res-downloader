package native

import (
	"context"
	shared "res-downloader/internal/model"
	"testing"
	"time"
)

func TestDefaultPluginEmitsCandidate(t *testing.T) {
	plugin := &DefaultPlugin{}
	result, err := plugin.Handle(context.Background(), shared.Observation{
		Stage:    shared.StageResponse,
		Request:  shared.RequestSnapshot{URL: "https://cdn.example/video", Host: "cdn.example"},
		Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "video/mp4"},
	})
	if err != nil || len(result.Resources) != 1 {
		t.Fatalf("handle: resources=%d err=%v", len(result.Resources), err)
	}
	if result.Resources[0].Kind != "media.video" {
		t.Fatalf("kind = %q", result.Resources[0].Kind)
	}
	if result.Resources[0].Tracks[0].Extension != ".mp4" || result.Resources[0].Tracks[0].Executor != "http-file" {
		t.Fatalf("track = %#v", result.Resources[0].Tracks[0])
	}
}

func TestDefaultPluginInfersKnownFilesFromGenericBinaryMIME(t *testing.T) {
	plugin := &DefaultPlugin{}
	tests := []struct {
		name               string
		url                string
		contentType        string
		contentDisposition string
		wantKind           string
		wantPrimaryType    string
		wantRole           string
		wantExtension      string
		wantMIME           string
		wantRenderer       string
		wantInferredFrom   string
	}{
		{
			name: "m4a from URL", url: "https://cdn.example/music/track.M4A?token=1",
			contentType: "application/octet-stream;charset=UTF-8",
			wantKind:    "media.audio", wantPrimaryType: shared.ResourceTypeAudio, wantRole: "audio",
			wantExtension: ".m4a", wantMIME: "audio/mp4", wantRenderer: "audio", wantInferredFrom: "url-extension",
		},
		{
			name: "PDF from disposition", url: "https://cdn.example/download?id=1",
			contentType: "application/octet-stream", contentDisposition: "attachment; filename*=UTF-8''report%2Epdf",
			wantKind: "document.pdf", wantPrimaryType: shared.ResourceTypeDocument, wantRole: "document",
			wantExtension: ".pdf", wantMIME: "application/pdf", wantRenderer: "pdf", wantInferredFrom: "content-disposition-extension",
		},
		{
			name: "ZIP from generic download MIME", url: "https://cdn.example/releases/source.zip",
			contentType: "application/force-download",
			wantKind:    "file.archive", wantPrimaryType: shared.ResourceTypeArchive, wantRole: "archive",
			wantExtension: ".zip", wantMIME: "application/zip", wantInferredFrom: "url-extension",
		},
		{
			name: "PNG with missing MIME", url: "https://cdn.example/images/cover.png",
			contentType: "",
			wantKind:    "media.image", wantPrimaryType: shared.ResourceTypeImage, wantRole: "image",
			wantExtension: ".png", wantMIME: "image/png", wantRenderer: "image", wantInferredFrom: "url-extension",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headers := shared.HeaderMap{}
			if test.contentDisposition != "" {
				headers["Content-Disposition"] = []string{test.contentDisposition}
			}
			result, err := plugin.Handle(context.Background(), shared.Observation{
				Stage:    shared.StageResponse,
				Request:  shared.RequestSnapshot{URL: test.url, Host: "cdn.example"},
				Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: test.contentType, Headers: headers},
			})
			if err != nil || len(result.Resources) != 1 {
				t.Fatalf("resources=%d err=%v", len(result.Resources), err)
			}
			resource := result.Resources[0]
			shared.NormalizeResourceCandidate(&resource, time.Unix(1, 0))
			track := resource.Tracks[0]
			if resource.Kind != test.wantKind || resource.PrimaryType != test.wantPrimaryType {
				t.Fatalf("resource kind=%q primaryType=%q", resource.Kind, resource.PrimaryType)
			}
			if track.Role != test.wantRole || track.Extension != test.wantExtension || track.MIME != test.wantMIME {
				t.Fatalf("track=%#v", track)
			}
			if got, _ := resource.Metadata["detector.inferredFrom"].(string); got != test.wantInferredFrom {
				t.Fatalf("inferredFrom=%q", got)
			}
			if test.wantRenderer == "" {
				if resource.Preview != nil {
					t.Fatalf("unexpected preview=%#v", resource.Preview)
				}
			} else if resource.Preview == nil || resource.Preview.Renderer != test.wantRenderer || resource.Preview.MIME != test.wantMIME {
				t.Fatalf("preview=%#v", resource.Preview)
			}
		})
	}
}

func TestDefaultPluginKeepsSpecificMIMEAndUnknownBinaryExtensions(t *testing.T) {
	plugin := &DefaultPlugin{}
	tests := []struct {
		name        string
		url         string
		contentType string
		wantKind    string
		wantRule    string
	}{
		{name: "specific MIME wins", url: "https://cdn.example/movie.mp4", contentType: "audio/mpeg", wantKind: "media.audio", wantRule: "audio-mpeg"},
		{name: "unknown binary stays binary", url: "https://cdn.example/data.bin", contentType: "application/octet-stream", wantKind: "stream.binary", wantRule: "binary-file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := plugin.Handle(context.Background(), shared.Observation{
				Stage:    shared.StageResponse,
				Request:  shared.RequestSnapshot{URL: test.url, Host: "cdn.example"},
				Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: test.contentType},
			})
			if err != nil || len(result.Resources) != 1 {
				t.Fatalf("resources=%d err=%v", len(result.Resources), err)
			}
			resource := result.Resources[0]
			if resource.Kind != test.wantKind || resource.Metadata["detector.ruleId"] != test.wantRule {
				t.Fatalf("resource=%#v", resource)
			}
			if _, inferred := resource.Metadata["detector.inferredFrom"]; inferred {
				t.Fatalf("unexpected inference metadata=%#v", resource.Metadata)
			}
		})
	}
}

func TestDefaultPluginUsesStructuredCustomRule(t *testing.T) {
	plugin := &DefaultPlugin{}
	rules := []shared.CaptureRule{{
		ID: "subtitle", Enabled: true, Priority: 10,
		Match: shared.CaptureRuleMatch{
			MIME: []string{"text/*"}, URL: []string{"*.vtt*"}, Status: []int{206}, MinSize: 10,
		},
		Resource: shared.CaptureRuleResource{
			Kind: "subtitle.webvtt", Role: "subtitle", Extension: ".vtt", Executor: "http-file",
			Capabilities: []string{shared.ResourceCapabilityDownload, shared.ResourceCapabilityCopy},
		},
	}}
	result, err := plugin.Handle(context.Background(), shared.Observation{
		Stage: shared.StageResponse,
		Request: shared.RequestSnapshot{
			URL: "https://cdn.example/captions.vtt?token=1", Host: "cdn.example", Path: "/captions.vtt",
		},
		Response: &shared.ResponseSnapshot{
			StatusCode: 206, ContentType: "text/vtt; charset=utf-8",
			Headers: shared.HeaderMap{"Content-Length": []string{"20"}},
		},
		Settings: map[string]interface{}{"rules": rules},
	})
	if err != nil || len(result.Resources) != 1 {
		t.Fatalf("handle: resources=%d err=%v", len(result.Resources), err)
	}
	if result.Resources[0].Kind != "subtitle.webvtt" || result.Resources[0].Metadata["detector.ruleId"] != "subtitle" {
		t.Fatalf("resource = %#v", result.Resources[0])
	}
}

func TestValidateCaptureRulesRejectsCatchAllAndDuplicateIDs(t *testing.T) {
	rule := shared.CaptureRule{
		ID: "all", Enabled: true,
		Resource: shared.CaptureRuleResource{Kind: "stream.binary", Executor: "http-file"},
	}
	if err := ValidateCaptureRules([]shared.CaptureRule{rule}); err == nil {
		t.Fatal("expected a catch-all rule without matchers to be rejected")
	}
	rule.Match.MIME = []string{"application/*"}
	if err := ValidateCaptureRules([]shared.CaptureRule{rule, rule}); err == nil {
		t.Fatal("expected duplicate rule IDs to be rejected")
	}
}

func TestCaptureRulesUsePriorityBeforeListOrder(t *testing.T) {
	low := shared.CaptureRule{
		ID: "low", Enabled: true, Priority: 1,
		Match: shared.CaptureRuleMatch{MIME: []string{"video/*"}},
		Resource: shared.CaptureRuleResource{
			Kind: "media.low", Executor: "http-file",
		},
	}
	high := low
	high.ID = "high"
	high.Priority = 10
	high.Resource.Kind = "media.high"
	rule, _ := MatchCaptureRule([]shared.CaptureRule{low, high}, shared.Observation{
		Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "video/mp4"},
	})
	if rule == nil || rule.ID != "high" {
		t.Fatalf("matched rule = %#v", rule)
	}
}

func TestDefaultPluginClassifiesHLSManifest(t *testing.T) {
	plugin := &DefaultPlugin{}
	tests := []struct {
		name     string
		body     string
		wantKind string
		wantMode string
		wantExec string
	}{
		{name: "vod", body: "\ufeff  #EXTM3U\n#EXTINF:5,\na.ts\n#EXT-X-ENDLIST\n", wantKind: "stream.hls", wantMode: "vod", wantExec: "hls"},
		{name: "live", body: "#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:12\n#EXTINF:5,\na.ts\n", wantKind: "stream.live", wantMode: "live", wantExec: "ffmpeg-hls"},
		{name: "master", body: "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000\nvideo.m3u8\n", wantKind: "stream.hls", wantMode: "master", wantExec: "hls"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := plugin.Handle(context.Background(), shared.Observation{
				Stage:    shared.StageResponse,
				Request:  shared.RequestSnapshot{URL: "https://cdn.example/live/index.m3u8?token=1", Host: "cdn.example", Path: "/live/index.m3u8"},
				Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "application/vnd.apple.mpegurl; charset=utf-8", Body: test.body},
			})
			if err != nil || len(result.Resources) != 1 {
				t.Fatalf("resources=%d err=%v", len(result.Resources), err)
			}
			resource := result.Resources[0]
			if resource.Kind != test.wantKind || resource.Metadata["stream.mode"] != test.wantMode || resource.Tracks[0].Executor != test.wantExec {
				t.Fatalf("resource=%#v", resource)
			}
		})
	}
}

func TestDefaultPluginClassifiesFLVAsLiveRecording(t *testing.T) {
	plugin := &DefaultPlugin{}
	result, err := plugin.Handle(context.Background(), shared.Observation{
		Stage: shared.StageResponse,
		Request: shared.RequestSnapshot{
			URL: "https://cdn.example/live/channel.flv?token=1", Host: "cdn.example", Path: "/live/channel.flv",
		},
		Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "video/x-flv"},
	})
	if err != nil || len(result.Resources) != 1 {
		t.Fatalf("resources=%d err=%v", len(result.Resources), err)
	}
	resource := result.Resources[0]
	if resource.Kind != "stream.live" || resource.PrimaryType != shared.ResourceTypeVideo {
		t.Fatalf("resource kind=%q primaryType=%q", resource.Kind, resource.PrimaryType)
	}
	if resource.Tracks[0].Executor != "ffmpeg-hls" || resource.Tracks[0].Extension != ".flv" {
		t.Fatalf("track=%#v", resource.Tracks[0])
	}
	if resource.Metadata["stream.protocol"] != "flv" || resource.Metadata["stream.mode"] != "live" {
		t.Fatalf("metadata=%#v", resource.Metadata)
	}
	if !contains(resource.Traits, shared.ResourceTraitLive) || !contains(resource.Traits, shared.ResourceTraitStreaming) {
		t.Fatalf("traits=%#v", resource.Traits)
	}
	resource.Source.PluginID = plugin.Manifest().ID
	plan, handled, err := plugin.Resolve(context.Background(), resource, shared.DownloadOptions{})
	if err != nil || !handled || len(plan.Inputs) != 1 {
		t.Fatalf("resolve handled=%v inputs=%d err=%v", handled, len(plan.Inputs), err)
	}
	if plan.Inputs[0].Executor != "ffmpeg-hls" || plan.Inputs[0].Options["reconnect"] != true {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestDefaultPluginReadsOnlySuspectedHLSBodies(t *testing.T) {
	plugin := &DefaultPlugin{}
	if !plugin.NeedsBody(shared.Observation{
		Stage: shared.StageResponse, Request: shared.RequestSnapshot{URL: "https://cdn.example/list.m3u8"},
		Response: &shared.ResponseSnapshot{ContentType: "text/plain"},
	}) {
		t.Fatal("m3u8 URL should request a bounded response body")
	}
	if plugin.NeedsBody(shared.Observation{
		Stage: shared.StageResponse, Request: shared.RequestSnapshot{URL: "https://cdn.example/video.mp4"},
		Response: &shared.ResponseSnapshot{ContentType: "video/mp4"},
	}) {
		t.Fatal("ordinary media body should not be buffered")
	}
}

func TestDefaultPluginDetectsM3U8URLAndSuppressesSegments(t *testing.T) {
	plugin := &DefaultPlugin{}
	manifest := shared.Observation{
		Stage:    shared.StageResponse,
		Request:  shared.RequestSnapshot{URL: "https://cdn.example/channel/index.m3u8?ts=1", Host: "cdn.example", Path: "/channel/index.m3u8"},
		Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "text/plain", Body: "#EXTM3U\n#EXTINF:4,\nsegment.ts\n#EXT-X-ENDLIST\n"},
	}
	result, err := plugin.Handle(context.Background(), manifest)
	if err != nil || len(result.Resources) != 1 {
		t.Fatalf("manifest resources=%d err=%v", len(result.Resources), err)
	}
	if result.Resources[0].GroupKey != "hls:https://cdn.example/channel/index.m3u8" {
		t.Fatalf("groupKey=%q", result.Resources[0].GroupKey)
	}
	segment, err := plugin.Handle(context.Background(), shared.Observation{
		Stage:    shared.StageResponse,
		Request:  shared.RequestSnapshot{URL: "https://cdn.example/channel/parts/segment.ts", Host: "cdn.example", Path: "/channel/parts/segment.ts"},
		Response: &shared.ResponseSnapshot{StatusCode: 200, ContentType: "application/octet-stream"},
	})
	if err != nil || len(segment.Resources) != 0 {
		t.Fatalf("segment should be suppressed: resources=%d err=%v", len(segment.Resources), err)
	}
}
