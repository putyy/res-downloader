package httpapi

import (
	"net/http"
	shared "res-downloader/internal/model"
	"testing"
)

func TestPreviewSelectsDeclaredTrackFromMultiInputPlan(t *testing.T) {
	candidate := shared.ResourceCandidate{Preview: &shared.PreviewSpec{Renderer: "video", TrackID: "video"}}
	plan := shared.DownloadPlan{
		Inputs: []shared.DownloadInput{
			{ID: "audio", Executor: "http-file", URL: "https://cdn.example/audio.m4a"},
			{ID: "video", Executor: "http-file", URL: "https://cdn.example/video.mp4"},
		},
		Pipeline: []shared.PipelineStep{{ID: "joined", Executor: "builtin.concat", Inputs: []string{"video", "audio"}}},
		Output:   shared.DownloadOutput{Input: "joined"},
	}
	input, ok := selectPreviewInput(candidate, plan)
	if !ok || input.ID != "video" {
		t.Fatalf("preview input = %#v, %v", input, ok)
	}
}

func TestPreviewRangeOffset(t *testing.T) {
	for _, test := range []struct {
		contentRange string
		requestRange string
		want         uint64
	}{
		{"bytes 5242880-10485759/20000000", "bytes=0-1", 5242880},
		{"", "bytes=37-100", 37},
		{"", "bytes=-100", 0},
	} {
		if got := previewRangeOffset(test.contentRange, test.requestRange); got != test.want {
			t.Fatalf("previewRangeOffset(%q, %q) = %d, expected %d", test.contentRange, test.requestRange, got, test.want)
		}
	}
}

func TestPreparePreviewResponseHeadersDropsContentRangeFromFullResponse(t *testing.T) {
	headers := http.Header{"Content-Range": []string{"bytes 0-9/10"}}
	preparePreviewResponseHeaders(headers, http.StatusOK)
	if headers.Get("Content-Range") != "" {
		t.Fatal("full response retained a partial-response Content-Range header")
	}

	headers.Set("Content-Range", "bytes 0-4/10")
	preparePreviewResponseHeaders(headers, http.StatusPartialContent)
	if headers.Get("Content-Range") == "" {
		t.Fatal("partial response lost its Content-Range header")
	}
}

func TestApplyDeclaredPreviewMIMEOnlyReplacesGenericContentType(t *testing.T) {
	candidate := shared.ResourceCandidate{Preview: &shared.PreviewSpec{Renderer: "audio", MIME: "audio/mp4"}}

	headers := http.Header{"Content-Type": []string{"application/octet-stream;charset=UTF-8"}}
	applyDeclaredPreviewMIME(headers, candidate)
	if got := headers.Get("Content-Type"); got != "audio/mp4" {
		t.Fatalf("generic Content-Type = %q", got)
	}

	headers.Set("Content-Type", "video/mp4")
	applyDeclaredPreviewMIME(headers, candidate)
	if got := headers.Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("specific Content-Type was overwritten: %q", got)
	}
}

func TestBoundedProcessedPreviewRange(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"", "bytes=0-4194303"},
		{"bytes=0-", "bytes=0-4194303"},
		{"bytes=100-", "bytes=100-4194403"},
		{"bytes=100-200", "bytes=100-200"},
		{"bytes=100-9999999", "bytes=100-4194403"},
		{"bytes=-9999999", "bytes=-4194304"},
		{"bytes=20-10", "bytes=0-4194303"},
		{"items=0-10", "bytes=0-4194303"},
		{"bytes=0-10,20-30", "bytes=0-4194303"},
	} {
		if got := boundedProcessedPreviewRange(test.input); got != test.want {
			t.Errorf("boundedProcessedPreviewRange(%q) = %q, expected %q", test.input, got, test.want)
		}
	}
}
