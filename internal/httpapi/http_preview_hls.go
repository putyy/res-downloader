package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	shared "res-downloader/internal/model"
	"strings"
	"time"
)

const (
	hlsPreviewTTL          = 10 * time.Minute
	hlsPreviewManifestSize = 2 * 1024 * 1024
	maxHLSPreviewTargets   = 20000
)

var hlsURIAttribute = regexp.MustCompile(`URI="([^"]+)"`)

type hlsPreviewTarget struct {
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

func isHLSPreview(candidate shared.ResourceCandidate, plan shared.DownloadPlan) bool {
	protocol, _ := candidate.Metadata["stream.protocol"].(string)
	if strings.EqualFold(protocol, "hls") || candidate.Kind == "stream.hls" {
		return true
	}
	mime := ""
	if candidate.Preview != nil {
		mime = candidate.Preview.MIME
	}
	if mime == "" {
		mime = candidate.Technical.MIME
	}
	if isHLSPreviewMIME(mime) {
		return true
	}
	for _, input := range plan.Inputs {
		if input.Executor == "hls" {
			return true
		}
	}
	return false
}

func isHLSPreviewMIME(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch value {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl":
		return true
	default:
		return false
	}
}

func (h *Server) previewHLSTarget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "Missing HLS preview token", http.StatusBadRequest)
		return
	}
	target, exists := h.hlsPreviewTarget(token)
	if !exists {
		http.Error(w, "HLS preview token expired", http.StatusGone)
		return
	}
	h.serveHLSPreview(w, r, target, false)
}

func (h *Server) serveHLSPreview(w http.ResponseWriter, r *http.Request, target hlsPreviewTarget, forceManifest bool) {
	if err := shared.ValidateRemoteURL(target.URL); err != nil {
		http.Error(w, "Invalid HLS preview URL", http.StatusBadRequest)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target.URL, nil)
	if err != nil {
		http.Error(w, "Failed to prepare HLS preview", http.StatusInternalServerError)
		return
	}
	applyPreviewRequestHeaders(request, target.Headers)
	if value := r.Header.Get("Range"); value != "" && !forceManifest {
		request.Header.Set("Range", value)
	}
	response, err := h.previewHTTPClient().Do(request)
	if err != nil {
		http.Error(w, "Failed to fetch HLS preview", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()

	manifest := forceManifest || isHLSPreviewMIME(response.Header.Get("Content-Type")) || strings.EqualFold(path.Ext(request.URL.Path), ".m3u8")
	if manifest && response.StatusCode >= 200 && response.StatusCode < 300 {
		if r.Method == http.MethodHead {
			copyPreviewHeaders(w.Header(), response.Header)
			prepareRewrittenManifestHeaders(w.Header())
			w.WriteHeader(http.StatusOK)
			return
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, hlsPreviewManifestSize+1))
		if readErr != nil {
			http.Error(w, "Failed to read HLS manifest", http.StatusBadGateway)
			return
		}
		if len(body) > hlsPreviewManifestSize {
			http.Error(w, "HLS manifest exceeds preview limit", http.StatusBadGateway)
			return
		}
		normalized := strings.TrimLeft(strings.TrimPrefix(string(body), "\ufeff"), " \t\r\n")
		if !strings.HasPrefix(normalized, "#EXTM3U") {
			http.Error(w, "Invalid HLS manifest", http.StatusBadGateway)
			return
		}
		rewritten, rewriteErr := h.rewriteHLSManifest(string(body), request.URL, target.Headers)
		if rewriteErr != nil {
			http.Error(w, "Failed to prepare HLS manifest", http.StatusInternalServerError)
			return
		}
		copyPreviewHeaders(w.Header(), response.Header)
		prepareRewrittenManifestHeaders(w.Header())
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, rewritten)
		return
	}

	copyPreviewHeaders(w.Header(), response.Header)
	preparePreviewResponseHeaders(w.Header(), response.StatusCode)
	w.WriteHeader(response.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, response.Body)
	}
}

func (h *Server) rewriteHLSManifest(manifest string, baseURL *url.URL, headers map[string]string) (string, error) {
	lineEnding := "\n"
	if strings.Contains(manifest, "\r\n") {
		lineEnding = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "#") {
			rewritten, err := h.registerHLSReference(baseURL, trimmed, headers)
			if err != nil {
				return "", err
			}
			lines[index] = rewritten
			continue
		}
		matches := hlsURIAttribute.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		var builder strings.Builder
		cursor := 0
		for _, match := range matches {
			builder.WriteString(line[cursor:match[2]])
			rewritten, err := h.registerHLSReference(baseURL, line[match[2]:match[3]], headers)
			if err != nil {
				return "", err
			}
			builder.WriteString(rewritten)
			cursor = match[3]
		}
		builder.WriteString(line[cursor:])
		lines[index] = builder.String()
	}
	return strings.Join(lines, lineEnding), nil
}

func (h *Server) registerHLSReference(baseURL *url.URL, reference string, headers map[string]string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(reference))
	if err != nil {
		return "", err
	}
	resolved := baseURL.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return reference, nil
	}
	token, err := h.registerHLSPreviewTarget(hlsPreviewTarget{
		URL: resolved.String(), Headers: cloneStringMap(headers), ExpiresAt: time.Now().Add(hlsPreviewTTL),
	})
	if err != nil {
		return "", err
	}
	return "/api/preview/hls?token=" + url.QueryEscape(token), nil
}

func (h *Server) registerHLSPreviewTarget(target hlsPreviewTarget) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	now := time.Now()
	h.hlsPreviewMu.Lock()
	defer h.hlsPreviewMu.Unlock()
	if h.hlsPreviews == nil {
		h.hlsPreviews = make(map[string]hlsPreviewTarget)
	}
	for key, existing := range h.hlsPreviews {
		if existing.ExpiresAt.Before(now) {
			delete(h.hlsPreviews, key)
		}
	}
	if len(h.hlsPreviews) >= maxHLSPreviewTargets {
		return "", errors.New("too many active HLS preview targets")
	}
	target.ExpiresAt = now.Add(hlsPreviewTTL)
	h.hlsPreviews[token] = target
	return token, nil
}

func (h *Server) hlsPreviewTarget(token string) (hlsPreviewTarget, bool) {
	now := time.Now()
	h.hlsPreviewMu.Lock()
	defer h.hlsPreviewMu.Unlock()
	target, exists := h.hlsPreviews[token]
	if !exists || target.ExpiresAt.Before(now) {
		delete(h.hlsPreviews, token)
		return hlsPreviewTarget{}, false
	}
	target.ExpiresAt = now.Add(hlsPreviewTTL)
	h.hlsPreviews[token] = target
	return target, true
}

func prepareRewrittenManifestHeaders(headers http.Header) {
	headers.Del("Content-Length")
	headers.Del("Content-Range")
	headers.Del("Content-Encoding")
	headers.Del("Accept-Ranges")
	headers.Set("Content-Type", "application/vnd.apple.mpegurl")
	headers.Set("Cache-Control", "no-store")
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
