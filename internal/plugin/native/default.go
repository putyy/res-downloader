package native

import (
	"context"
	"mime"
	"net/url"
	"path"
	shared "res-downloader/internal/model"
	"strings"
	"sync"
	"time"
)

const hlsManifestBodyLimit = 1024 * 1024

type DefaultPlugin struct {
	hlsMu    sync.Mutex
	hlsRoots map[string]time.Time
}

func (p *DefaultPlugin) Manifest() shared.PluginManifest {
	return shared.PluginManifest{
		ID:     "builtin.generic-detector",
		Name:   "Generic resource detector",
		Author: shared.PluginAuthor{Name: "res-downloader", URL: "https://github.com/putyy/res-downloader"},
		Locales: map[string]shared.PluginLocale{
			"zh-CN": {Name: "通用资源探测器", Description: "使用可编辑的结构化规则发现普通网络资源。"},
			"zh":    {Name: "通用资源探测器", Description: "使用可编辑的结构化规则发现普通网络资源。"},
			"en":    {Name: "Generic resource detector", Description: "Discovers ordinary network resources using editable structured rules."},
		},
		Version:       "1.0.0",
		APIVersion:    shared.PluginAPIVersion,
		Runtime:       "native",
		Priority:      -1000,
		ResourceKinds: DefaultResourceKinds(),
		SettingsSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"rules": map[string]interface{}{
					"type": "array", "format": "capture-rules", "default": DefaultCaptureRules(),
				},
			},
		},
		Requires: shared.PluginRequirements{FFmpeg: ">=4.0"},
		Permissions: shared.PluginPermissions{
			Domains:      []string{"*"},
			Capabilities: []string{"observe-response", "read-response-body", "emit-resource", "media.ffmpeg.network"},
			BodyLimit:    hlsManifestBodyLimit,
		},
		Match: []shared.PluginMatchRule{{Stage: shared.StageResponse, Host: "*", ReadBody: boolPointer(false)}},
	}
}

func (p *DefaultPlugin) Handle(_ context.Context, obs shared.Observation) (shared.PluginResult, error) {
	if obs.Stage != shared.StageResponse || obs.Response == nil {
		return shared.PluginResult{}, nil
	}
	if p.isKnownHLSSegment(obs.Request.URL) {
		return shared.PluginResult{}, nil
	}
	rules, err := DecodeCaptureRules(obs.Settings["rules"])
	if err != nil {
		return shared.PluginResult{}, err
	}
	rule, size := MatchCaptureRule(rules, obs)
	trackMIME := obs.Response.ContentType
	inferredFrom := ""
	originalRuleID := ""
	if rule != nil {
		originalRuleID = rule.ID
	}
	if isGenericBinaryMIME(obs.Response.ContentType) && (rule == nil || rule.Resource.Kind == "stream.binary") {
		if inferred, canonicalMIME, source := inferGenericBinaryCaptureRule(rules, obs, size); inferred != nil {
			rule = inferred
			inferredFrom = source
			if canonicalMIME != "" {
				trackMIME = canonicalMIME
			}
		}
	}
	if rule == nil {
		return shared.PluginResult{}, nil
	}
	role := rule.Resource.Role
	if role == "" {
		role = "primary"
	}
	executor := rule.Resource.Executor
	if executor == "" {
		executor = "http-file"
	}
	extension := CaptureRuleExtension(*rule, obs.Request.URL)
	kind := rule.Resource.Kind
	title := captureTitle(obs.Response.Headers)
	primaryType := ""
	metadata := map[string]interface{}{"detector.ruleId": rule.ID}
	if inferredFrom != "" {
		metadata["detector.inferredFrom"] = inferredFrom
		if originalRuleID != "" {
			metadata["detector.fallbackRuleId"] = originalRuleID
		}
		metadata["detector.responseMime"] = obs.Response.ContentType
	}
	traits := []string(nil)
	if executor == "hls" {
		primaryType = shared.ResourceTypeVideo
		if title == "" {
			if parsed, err := url.Parse(obs.Request.URL); err == nil {
				title = strings.TrimSuffix(path.Base(parsed.Path), path.Ext(parsed.Path))
			}
		}
		manifest := normalizedHLSManifest(obs.Response.Body)
		if obs.Response.Body != "" && !strings.HasPrefix(manifest, "#EXTM3U") {
			return shared.PluginResult{}, nil
		}
		mode := classifyHLSManifest(manifest)
		p.rememberHLSRoot(obs.Request.URL)
		metadata["stream.protocol"] = "hls"
		metadata["stream.mode"] = mode
		metadata["stream.manifestUrl"] = obs.Request.URL
		traits = append(traits, shared.ResourceTraitSegmented, shared.ResourceTraitStreaming)
		if mode == "live" {
			kind, executor, extension = "stream.live", "ffmpeg-hls", ".mp4"
			metadata["stream.requiresFFmpeg"] = true
			traits = append(traits, shared.ResourceTraitLive)
		}
	}
	if kind == "stream.live" {
		primaryType = shared.ResourceTypeVideo
		traits = append(traits, shared.ResourceTraitStreaming, shared.ResourceTraitLive)
		if executor == "ffmpeg-hls" {
			metadata["stream.requiresFFmpeg"] = true
			if _, exists := metadata["stream.protocol"]; !exists {
				metadata["stream.protocol"] = "flv"
				metadata["stream.mode"] = "live"
			}
		}
	} else if kind == "stream.hls" {
		primaryType = shared.ResourceTypeVideo
	}
	groupKey := ""
	if metadata["stream.protocol"] == "hls" {
		groupKey = hlsGroupKey(executor, obs.Request.URL)
	}
	resource := shared.ResourceCandidate{
		GroupKey:    groupKey,
		Kind:        kind,
		PrimaryType: primaryType,
		Title:       title,
		Traits:      traits,
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: role, Executor: executor, URL: obs.Request.URL,
			MIME: trackMIME, Extension: extension, Size: size,
			Headers: flattenHeaders(obs.Request.Headers),
		}},
		RequiredTracks: []string{role},
		Capabilities:   append([]string(nil), rule.Resource.Capabilities...),
		Metadata:       metadata,
		Source: shared.ResourceSource{
			PageURL: firstHeader(obs.Request.Headers, "Referer"),
			Domain:  obs.Request.Host,
		},
	}
	if rule.Resource.PreviewRenderer != "" {
		mode := rule.Resource.PreviewMode
		if mode == "" {
			mode = "proxy"
		}
		resource.Preview = &shared.PreviewSpec{
			Renderer: rule.Resource.PreviewRenderer, Mode: mode,
			MIME: trackMIME, TrackID: "primary",
		}
	}
	return shared.PluginResult{Resources: []shared.ResourceCandidate{resource}}, nil
}

func hlsGroupKey(executor, rawURL string) string {
	if executor != "hls" && executor != "ffmpeg-hls" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "hls:" + rawURL
	}
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		switch strings.ToLower(key) {
		case "_", "_t", "t", "ts", "timestamp", "rnd", "random", "cache", "cachebuster":
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	key := "hls:" + parsed.String()
	if len(key) > 512 {
		return "hls:" + shared.Md5(parsed.String())
	}
	return key
}

func (p *DefaultPlugin) rememberHLSRoot(rawURL string) {
	root := hlsRoot(rawURL)
	if root == "" {
		return
	}
	p.hlsMu.Lock()
	defer p.hlsMu.Unlock()
	if p.hlsRoots == nil {
		p.hlsRoots = make(map[string]time.Time)
	}
	p.hlsRoots[root] = time.Now().Add(10 * time.Minute)
}

func (p *DefaultPlugin) isKnownHLSSegment(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".ts", ".m4s", ".key":
	default:
		return false
	}
	segmentURL := strings.ToLower(parsed.Scheme+"://"+parsed.Host) + parsed.Path
	now := time.Now()
	p.hlsMu.Lock()
	defer p.hlsMu.Unlock()
	for key, expiresAt := range p.hlsRoots {
		if expiresAt.Before(now) {
			delete(p.hlsRoots, key)
		}
	}
	for root := range p.hlsRoots {
		if strings.HasPrefix(segmentURL, root) {
			return true
		}
	}
	return false
}

func hlsRoot(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme+"://"+parsed.Host) + path.Dir(parsed.Path) + "/"
}

func (p *DefaultPlugin) NeedsBody(obs shared.Observation) bool {
	if obs.Stage != shared.StageResponse || obs.Response == nil {
		return false
	}
	if isHLSContentType(obs.Response.ContentType) {
		return true
	}
	parsed, err := url.Parse(obs.Request.URL)
	if err == nil && strings.EqualFold(path.Ext(parsed.Path), ".m3u8") {
		return true
	}
	disposition := firstHeader(obs.Response.Headers, "Content-Disposition")
	return strings.Contains(strings.ToLower(disposition), ".m3u8")
}

func isHLSContentType(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch value {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl":
		return true
	default:
		return false
	}
}

func normalizedHLSManifest(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	return strings.TrimLeft(value, " \t\r\n")
}

func classifyHLSManifest(value string) string {
	if value == "" || strings.Contains(value, "#EXT-X-STREAM-INF") {
		if strings.Contains(value, "#EXT-X-STREAM-INF") {
			return "master"
		}
		return "unknown"
	}
	if strings.Contains(value, "#EXT-X-ENDLIST") {
		return "vod"
	}
	return "live"
}

func (p *DefaultPlugin) Resolve(_ context.Context, resource shared.ResourceCandidate, _ shared.DownloadOptions) (shared.DownloadPlan, bool, error) {
	protocol, _ := resource.Metadata["stream.protocol"].(string)
	track := shared.PrimaryResourceTrack(resource.Tracks)
	if track == nil {
		return shared.DownloadPlan{}, false, nil
	}
	executor := track.Executor
	if protocol != "hls" && !(resource.Kind == "stream.live" && executor == "ffmpeg-hls") {
		return shared.DownloadPlan{}, false, nil
	}
	if executor != "hls" && executor != "ffmpeg-hls" {
		return shared.DownloadPlan{}, false, nil
	}
	options := map[string]interface{}{}
	mode, _ := resource.Metadata["stream.mode"].(string)
	if executor == "ffmpeg-hls" {
		options["reconnect"] = true
	} else if mode == "vod" {
		options["requireEndList"] = true
	}
	input := shared.DownloadInput{
		ID: track.ID, Executor: executor, URL: track.URL, Headers: track.Headers,
		Extension: track.Extension, Processors: track.Processors, Options: options,
	}
	return shared.DownloadPlan{
		Inputs: []shared.DownloadInput{input},
		Output: shared.DownloadOutput{Input: input.ID, Extension: input.Extension},
	}, true, nil
}

func captureTitle(headers shared.HeaderMap) string {
	disposition := firstHeader(headers, "Content-Disposition")
	if disposition == "" {
		return ""
	}
	_, parameters, err := mime.ParseMediaType(disposition)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parameters["filename"])
}

func boolPointer(value bool) *bool { return &value }

func firstHeader(headers shared.HeaderMap, key string) string {
	for name, values := range headers {
		if strings.EqualFold(name, key) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func flattenHeaders(headers shared.HeaderMap) map[string]string {
	out := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}
