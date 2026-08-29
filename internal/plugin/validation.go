package plugin

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	shared "res-downloader/internal/model"
	"strings"
	"unicode"
)

type manifestOverridePlugin struct {
	shared.RuntimePlugin
	manifest shared.PluginManifest
}

func (p *manifestOverridePlugin) Manifest() shared.PluginManifest { return p.manifest }

func pluginMatches(manifest shared.PluginManifest, obs shared.Observation) bool {
	if obs.Stage == shared.StageRequest && !manifest.Permissions.Has("observe-request") {
		return false
	}
	if obs.Stage == shared.StageResponse && !manifest.Permissions.Has("observe-response") {
		return false
	}
	if !domainAllowed(manifest.Permissions.Domains, obs.Request.Host) {
		return false
	}
	if len(manifest.Match) == 0 {
		return true
	}
	for _, rule := range manifest.Match {
		if matchRule(rule, obs) {
			return true
		}
	}
	return false
}

func matchRule(rule shared.PluginMatchRule, obs shared.Observation) bool {
	if rule.Stage != "" && rule.Stage != obs.Stage {
		return false
	}
	if rule.Host != "" && !wildcardMatch(rule.Host, hostname(obs.Request.Host)) {
		return false
	}
	if rule.Path != "" && !wildcardMatch(rule.Path, obs.Request.Path) {
		return false
	}
	if rule.URL != "" && !wildcardMatch(rule.URL, obs.Request.URL) {
		return false
	}
	if rule.Method != "" && !strings.EqualFold(rule.Method, obs.Request.Method) {
		return false
	}
	if len(rule.ContentTypes) > 0 && (obs.Response == nil || !matchesAny(rule.ContentTypes, obs.Response.ContentType)) {
		return false
	}
	return true
}

func pluginWantsBody(runtime shared.RuntimePlugin, manifest shared.PluginManifest, obs shared.Observation) bool {
	if aware, ok := runtime.(shared.BodyAwarePlugin); ok {
		return aware.NeedsBody(obs)
	}
	matched := false
	for _, rule := range manifest.Match {
		if !matchRule(rule, obs) {
			continue
		}
		matched = true
		if rule.ReadBody == nil || *rule.ReadBody {
			return true
		}
	}
	return len(manifest.Match) == 0 || !matched
}

func domainAllowed(domains []string, host string) bool {
	if len(domains) == 0 {
		return false
	}
	return matchesAny(domains, hostname(host))
}

func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if wildcardMatch(pattern, value) {
			return true
		}
	}
	return false
}

func wildcardMatch(pattern, value string) bool {
	pattern, value = strings.ToLower(pattern), strings.ToLower(value)
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	position := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[position:], part)
		if found < 0 || (index == 0 && !strings.HasPrefix(pattern, "*") && found != 0) {
			return false
		}
		position += found + len(part)
	}
	return strings.HasSuffix(pattern, "*") || strings.HasSuffix(value, parts[len(parts)-1])
}

func hostname(host string) string {
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(parsed, "[]")
	}
	return strings.Trim(host, "[]")
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func validateDownloadPlan(plan shared.DownloadPlan) error {
	if len(plan.Inputs) == 0 {
		return errors.New("download plan requires at least one input")
	}
	if !validExtension(plan.Output.Extension) {
		return errors.New("plugin returned an invalid file extension")
	}
	available := make(map[string]struct{}, len(plan.Inputs)+len(plan.Pipeline))
	for _, input := range plan.Inputs {
		if !validIdentifier(input.ID) {
			return fmt.Errorf("download input id %q is invalid", input.ID)
		}
		if _, exists := available[input.ID]; exists {
			return fmt.Errorf("duplicate download input id %q", input.ID)
		}
		available[input.ID] = struct{}{}
		if input.Executor != "http-file" && input.Executor != "hls" && input.Executor != "ffmpeg-hls" && input.Executor != "capture-file" {
			return fmt.Errorf("unsupported acquisition executor %q", input.Executor)
		}
		if input.Executor == "capture-file" {
			if input.CaptureKey == "" {
				return fmt.Errorf("download input %q requires captureKey", input.ID)
			}
		} else {
			if input.CaptureKey != "" {
				return fmt.Errorf("download input %q uses captureKey without capture-file executor", input.ID)
			}
			if err := shared.ValidateRemoteURL(input.URL); err != nil {
				return fmt.Errorf("download input %q has an invalid URL", input.ID)
			}
		}
		if !validExtension(input.Extension) {
			return fmt.Errorf("download input %q has an invalid extension", input.ID)
		}
		for _, processor := range input.Processors {
			if err := validateDownloadProcessor(processor); err != nil {
				return fmt.Errorf("download input %q: %w", input.ID, err)
			}
		}
	}
	for _, step := range plan.Pipeline {
		if !validIdentifier(step.ID) {
			return fmt.Errorf("pipeline step id %q is invalid", step.ID)
		}
		if _, exists := available[step.ID]; exists {
			return fmt.Errorf("duplicate pipeline value id %q", step.ID)
		}
		if step.Executor != "builtin.concat" && step.Executor != "builtin.media.mux" &&
			step.Executor != "builtin.media.remux" && step.Executor != "builtin.media.extract_audio" &&
			step.Executor != "plugin.ffmpeg" {
			return fmt.Errorf("unsupported pipeline executor %q", step.Executor)
		}
		if len(step.Inputs) == 0 {
			return fmt.Errorf("pipeline step %q requires inputs", step.ID)
		}
		for _, inputID := range step.Inputs {
			if _, exists := available[inputID]; !exists {
				return fmt.Errorf("pipeline step %q references unknown input %q", step.ID, inputID)
			}
		}
		available[step.ID] = struct{}{}
	}
	if _, exists := available[plan.Output.Input]; !exists {
		return fmt.Errorf("download output references unknown input %q", plan.Output.Input)
	}
	for _, processor := range plan.Output.Processors {
		if err := validateDownloadProcessor(processor); err != nil {
			return fmt.Errorf("download output: %w", err)
		}
	}
	return nil
}

func validateMediaPlanPermissions(media *mediaEngine, manifest shared.PluginManifest, plan shared.DownloadPlan) error {
	usesFFmpeg := false
	for _, input := range plan.Inputs {
		if input.Executor == "ffmpeg-hls" && !manifest.Permissions.Has("media.ffmpeg.network") {
			return errors.New("ffmpeg-hls requires media.ffmpeg.network permission")
		}
		if input.Executor == "ffmpeg-hls" {
			usesFFmpeg = true
		}
	}
	for _, step := range plan.Pipeline {
		switch step.Executor {
		case "builtin.media.mux", "builtin.media.remux", "builtin.media.extract_audio":
			usesFFmpeg = true
			if !manifest.Permissions.Has("media.basic") {
				return fmt.Errorf("%s requires media.basic permission", step.Executor)
			}
		case "plugin.ffmpeg":
			usesFFmpeg = true
			if !manifest.Permissions.Has("media.ffmpeg") {
				return errors.New("plugin.ffmpeg requires media.ffmpeg permission")
			}
		}
	}
	if usesFFmpeg && (media == nil || !media.SatisfiesFFmpeg(manifest.Requires.FFmpeg)) {
		return fmt.Errorf("plugin requires FFmpeg %s; configure a compatible executable in Settings", manifest.Requires.FFmpeg)
	}
	return nil
}

func validateDownloadProcessor(processor shared.DownloadStep) error {
	switch processor.Type {
	case "xor-prefix":
		key, _ := processor.Options["key"].(string)
		if key == "" {
			return errors.New("xor-prefix processor requires key")
		}
		if _, err := base64.StdEncoding.DecodeString(key); err != nil {
			return fmt.Errorf("xor-prefix processor key must be base64: %w", err)
		}
		return nil
	case wasmProcessorType:
		processorID, _ := processor.Options[wasmProcessorIDOption].(string)
		if processorID == "" {
			return errors.New("plugin-wasm processor requires options.processor")
		}
		if owner, exists := processor.Options[wasmProcessorOwnerKey]; exists {
			if ownerID, ok := owner.(string); !ok || ownerID == "" {
				return errors.New("plugin-wasm processor has an invalid bound owner")
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported download processor %q", processor.Type)
	}
}

func validExtension(extension string) bool {
	if extension == "" {
		return true
	}
	if len(extension) > 20 || !strings.HasPrefix(extension, ".") {
		return false
	}
	for _, char := range extension[1:] {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '_' && char != '-' {
			return false
		}
	}
	return len(extension) > 1
}

func mergeResponsePatch(base, next *shared.ResponsePatch) *shared.ResponsePatch {
	if base == nil {
		base = &shared.ResponsePatch{Headers: map[string]string{}}
	}
	if next.StatusCode != 0 {
		base.StatusCode = next.StatusCode
	}
	if next.Body != nil {
		base.Body = next.Body
	}
	if base.Headers == nil {
		base.Headers = map[string]string{}
	}
	for key, value := range next.Headers {
		base.Headers[key] = value
	}
	return base
}

func applySnapshotPatch(response *shared.ResponseSnapshot, patch *shared.ResponsePatch) {
	if patch.StatusCode != 0 {
		response.StatusCode = patch.StatusCode
	}
	if patch.Body != nil {
		response.Body = *patch.Body
	}
	for key, value := range patch.Headers {
		response.Headers[key] = []string{value}
		if strings.EqualFold(key, "Content-Type") {
			response.ContentType = value
		}
	}
}
