package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"
	"time"
)

// Processed previews must be materialised before their WASM pipeline can run.
// Keep each request bounded so a media element cannot make the proxy buffer an
// entire encrypted video with an open-ended Range request.
const processedPreviewChunkSize int64 = 4 * 1024 * 1024

func (h *Server) preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resourceID := r.URL.Query().Get("id")
	if resourceID == "" {
		http.Error(w, "Missing resource id", http.StatusBadRequest)
		return
	}
	if h.resources == nil || h.plugins == nil {
		http.Error(w, "Resource service is not ready", http.StatusServiceUnavailable)
		return
	}
	realURL := ""
	var plan shared.DownloadPlan
	candidate, exists := h.resources.Candidate(resourceID)
	if !exists {
		http.Error(w, "Resource not found", http.StatusNotFound)
		return
	}
	if candidate.Preview == nil || !containsString(candidate.Capabilities, shared.ResourceCapabilityPreview) {
		http.Error(w, "Resource does not support preview", http.StatusUnprocessableEntity)
		return
	}
	if candidate.Lifecycle.Availability == shared.ResourceAvailabilityNeedsRefresh ||
		(candidate.Lifecycle.ExpiresAt > 0 && candidate.Lifecycle.ExpiresAt <= time.Now().UnixMilli()) {
		refreshed, status, refreshErr := h.plugins.RefreshResource(r.Context(), candidate, shared.DownloadOptions{})
		if refreshErr != nil {
			http.Error(w, "Failed to refresh preview: "+refreshErr.Error(), http.StatusBadGateway)
			return
		}
		if status != shared.ResourceRefreshOK {
			http.Error(w, "Preview requires recapture: "+status, http.StatusGone)
			return
		}
		candidate = refreshed
		_ = h.resources.SaveCandidate(candidate)
	}
	resolved, err := h.plugins.CreateDownloadPlan(r.Context(), candidate, shared.DownloadOptions{})
	if err != nil {
		http.Error(w, "Failed to resolve preview: "+err.Error(), http.StatusBadGateway)
		return
	}
	plan = resolved
	var previewProcessors []shared.DownloadStep
	var previewHeaders map[string]string
	if len(plan.Inputs) > 0 {
		input, found := selectPreviewInput(candidate, plan)
		if !found {
			http.Error(w, "Preview track is not a direct download input", http.StatusUnprocessableEntity)
			return
		}
		realURL = input.URL
		previewHeaders = input.Headers
		previewProcessors = append(previewProcessors, input.Processors...)
		previewProcessors = append(previewProcessors, plan.Output.Processors...)
	}
	if realURL == "" {
		http.Error(w, "Resource has no directly previewable input", http.StatusUnprocessableEntity)
		return
	}
	if len(previewProcessors) == 0 && isHLSPreview(candidate, plan) {
		h.serveHLSPreview(w, r, hlsPreviewTarget{
			URL: realURL, Headers: cloneStringMap(previewHeaders), ExpiresAt: time.Now().Add(hlsPreviewTTL),
		}, true)
		return
	}
	parsedURL, err := url.Parse(realURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, parsedURL.String(), nil)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}
	applyPreviewRequestHeaders(request, previewHeaders)

	rangeHeader := r.Header.Get("Range")
	if len(previewProcessors) > 0 {
		rangeHeader = boundedProcessedPreviewRange(rangeHeader)
	}
	if rangeHeader != "" {
		request.Header.Set("Range", rangeHeader)
	}

	resp, err := h.previewHTTPClient().Do(request)
	if err != nil {
		http.Error(w, "Failed to fetch the resource", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if r.Method == http.MethodHead {
		copyPreviewHeaders(w.Header(), resp.Header)
		applyDeclaredPreviewMIME(w.Header(), candidate)
		preparePreviewResponseHeaders(w.Header(), resp.StatusCode)
		w.WriteHeader(resp.StatusCode)
		return
	}

	if len(previewProcessors) > 0 && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent) {
		temp, err := os.CreateTemp("", ".res-downloader-preview-*")
		if err != nil {
			http.Error(w, "Failed to prepare preview", http.StatusInternalServerError)
			return
		}
		tempPath := temp.Name()
		defer os.Remove(tempPath)
		written, copyErr := io.Copy(temp, io.LimitReader(resp.Body, processedPreviewChunkSize+1))
		if copyErr != nil {
			_ = temp.Close()
			http.Error(w, "Failed to read preview", http.StatusBadGateway)
			return
		}
		if written > processedPreviewChunkSize {
			_ = temp.Close()
			http.Error(w, "Preview source did not honour the bounded range request", http.StatusBadGateway)
			return
		}
		if err := temp.Close(); err != nil {
			http.Error(w, "Failed to prepare preview", http.StatusInternalServerError)
			return
		}
		offset := previewRangeOffset(resp.Header.Get("Content-Range"), rangeHeader)
		if err := h.resources.ProcessDownload(tempPath, previewProcessors, offset, false); err != nil {
			http.Error(w, "Failed to process preview: "+err.Error(), http.StatusBadGateway)
			return
		}
		processed, err := os.Open(tempPath)
		if err != nil {
			http.Error(w, "Failed to open preview", http.StatusInternalServerError)
			return
		}
		defer processed.Close()
		info, err := processed.Stat()
		if err != nil {
			http.Error(w, "Failed to inspect preview", http.StatusInternalServerError)
			return
		}
		copyPreviewHeaders(w.Header(), resp.Header)
		applyDeclaredPreviewMIME(w.Header(), candidate)
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		preparePreviewResponseHeaders(w.Header(), resp.StatusCode)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, processed); err != nil {
			return
		}
		return
	}

	copyPreviewHeaders(w.Header(), resp.Header)
	applyDeclaredPreviewMIME(w.Header(), candidate)
	preparePreviewResponseHeaders(w.Header(), resp.StatusCode)
	w.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(w, resp.Body); err != nil {
		return
	}
}

func selectPreviewInput(candidate shared.ResourceCandidate, plan shared.DownloadPlan) (shared.DownloadInput, bool) {
	inputID := plan.Output.Input
	if candidate.Preview != nil && candidate.Preview.TrackID != "" {
		inputID = candidate.Preview.TrackID
	}
	if inputID != "" {
		for _, input := range plan.Inputs {
			if input.ID == inputID {
				return input, true
			}
		}
		return shared.DownloadInput{}, false
	}
	if len(plan.Inputs) == 1 {
		return plan.Inputs[0], true
	}
	return shared.DownloadInput{}, false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func copyPreviewHeaders(destination, source http.Header) {
	for _, key := range []string{
		"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges",
		"Cache-Control", "ETag", "Last-Modified",
	} {
		if value := source.Get(key); value != "" {
			destination.Set(key, value)
		}
	}
}

func applyDeclaredPreviewMIME(headers http.Header, candidate shared.ResourceCandidate) {
	if candidate.Preview == nil || strings.TrimSpace(candidate.Preview.MIME) == "" {
		return
	}
	current := strings.ToLower(strings.TrimSpace(strings.Split(headers.Get("Content-Type"), ";")[0]))
	switch current {
	case "", "application/octet-stream", "binary/octet-stream", "application/binary",
		"application/download", "application/x-download", "application/force-download", "application/x-force-download":
		headers.Set("Content-Type", candidate.Preview.MIME)
	}
}

func preparePreviewResponseHeaders(headers http.Header, status int) {
	if status != http.StatusPartialContent {
		headers.Del("Content-Range")
	}
}

func previewRangeOffset(contentRange, requestRange string) uint64 {
	value := contentRange
	if value == "" {
		value = requestRange
	}
	value = strings.TrimSpace(strings.TrimPrefix(value, "bytes"))
	value = strings.TrimPrefix(value, "=")
	start, _, ok := strings.Cut(value, "-")
	if !ok {
		return 0
	}
	offset, err := strconv.ParseUint(strings.TrimSpace(start), 10, 64)
	if err != nil {
		return 0
	}
	return offset
}

func boundedProcessedPreviewRange(value string) string {
	defaultRange := fmt.Sprintf("bytes=0-%d", processedPreviewChunkSize-1)
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultRange
	}
	prefix, ranges, ok := strings.Cut(value, "=")
	if !ok || !strings.EqualFold(strings.TrimSpace(prefix), "bytes") || strings.Contains(ranges, ",") {
		return defaultRange
	}
	startText, endText, ok := strings.Cut(strings.TrimSpace(ranges), "-")
	if !ok {
		return defaultRange
	}
	startText = strings.TrimSpace(startText)
	endText = strings.TrimSpace(endText)
	if startText == "" {
		suffix, err := strconv.ParseUint(endText, 10, 64)
		if err != nil || suffix == 0 {
			return defaultRange
		}
		if suffix > uint64(processedPreviewChunkSize) {
			suffix = uint64(processedPreviewChunkSize)
		}
		return fmt.Sprintf("bytes=-%d", suffix)
	}
	start, err := strconv.ParseUint(startText, 10, 64)
	if err != nil {
		return defaultRange
	}
	maximumEnd := start + uint64(processedPreviewChunkSize) - 1
	if maximumEnd < start {
		maximumEnd = ^uint64(0)
	}
	if endText == "" {
		return fmt.Sprintf("bytes=%d-%d", start, maximumEnd)
	}
	end, err := strconv.ParseUint(endText, 10, 64)
	if err != nil || end < start {
		return defaultRange
	}
	if end > maximumEnd {
		end = maximumEnd
	}
	return fmt.Sprintf("bytes=%d-%d", start, end)
}
