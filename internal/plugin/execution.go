package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	shared "res-downloader/internal/model"
	"strings"
	"time"
)

const (
	maxPluginResponseCaptures = 8
	maxPluginCaptureKeySize   = 512
)

// Process runs matching plugins for one captured observation and publishes the
// validated resources they emit. Runtime execution lives outside the manager's
// discovery/state file so both responsibilities can evolve independently.
func (m *PluginManager) Process(ctx context.Context, obs shared.Observation) shared.PluginResult {
	m.mu.RLock()
	registered := append([]managedPlugin(nil), m.plugins...)
	m.mu.RUnlock()

	combined := shared.PluginResult{Decision: shared.DecisionContinue}
	handledByExternal := false
	for _, item := range registered {
		manifest := item.runtime.Manifest()
		if manifest.ID == "builtin.generic-detector" && handledByExternal {
			continue
		}
		if !manifest.IsEnabled() || !pluginMatches(manifest, obs) {
			continue
		}
		pluginObs := sanitizeObservation(obs, manifest.Permissions)
		if !pluginWantsBody(item.runtime, manifest, obs) {
			pluginObs.Request.Body = ""
			if pluginObs.Response != nil {
				pluginObs.Response.Body = ""
			}
		} else {
			limitObservationBody(&pluginObs, manifest.Permissions)
		}
		pluginObs.Settings = m.pluginSettings(manifest.ID)
		var result shared.PluginResult
		err := m.runtimeState(manifest.ID).run(ctx, func(callCtx context.Context) error {
			var callErr error
			result, callErr = item.runtime.Handle(callCtx, pluginObs)
			return callErr
		})
		if err != nil {
			combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": "+err.Error())
			m.logger.Esg(err, "plugin "+manifest.ID)
			continue
		}
		if len(result.Resources) > maxPluginResources {
			combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": resource result was truncated")
			result.Resources = result.Resources[:maxPluginResources]
		}
		if len(result.Resources) > 0 && manifest.Permissions.Has("emit-resource") {
			for index := range result.Resources {
				result.Resources[index].Source.PluginID = manifest.ID
				result.Resources[index].Source.PluginVersion = manifest.Version
				m.mu.RLock()
				result.Resources[index].Source.PluginDigest = m.statuses[manifest.ID].Digest
				m.mu.RUnlock()
				result.Resources[index].ParentID = ""
				if resourceUsesCapture(result.Resources[index]) && !manifest.Permissions.Has("capture-response-body") {
					combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": capture-file resource requires capture-response-body")
					continue
				}
				if err := validateResourceActions(manifest, result.Resources[index].Actions); err != nil {
					combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": "+err.Error())
					continue
				}
				if err := validateCandidate(&result.Resources[index]); err != nil {
					combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": "+err.Error())
					continue
				}
				combined.Resources = append(combined.Resources, result.Resources[index])
			}
		}
		if result.Patch != nil && manifest.Permissions.Has("modify-response") && obs.Response != nil && !obs.Response.Truncated {
			limit := manifest.Permissions.BodyLimit
			if limit <= 0 {
				limit = defaultPluginBodyLimit
			}
			if result.Patch.Body != nil && int64(len(*result.Patch.Body)) > limit {
				combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": response patch exceeds bodyLimit")
			} else {
				combined.Patch = mergeResponsePatch(combined.Patch, result.Patch)
				applySnapshotPatch(obs.Response, result.Patch)
			}
		}
		if result.SyntheticResponse != nil && manifest.Permissions.Has("intercept-request") {
			if len(result.SyntheticResponse.Body) <= maxSyntheticResponseSize {
				combined.SyntheticResponse = result.SyntheticResponse
			} else {
				combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": synthetic response is too large")
			}
		}
		if len(result.Captures) > 0 && manifest.Permissions.Has("capture-response-body") &&
			obs.Stage == shared.StageResponse && obs.Response != nil &&
			(obs.Response.StatusCode == 200 || obs.Response.StatusCode == 206) {
			if len(result.Captures) > maxPluginResponseCaptures {
				combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": response captures were truncated")
				result.Captures = result.Captures[:maxPluginResponseCaptures]
			}
			for _, capture := range result.Captures {
				if err := validateResponseCapture(capture); err != nil {
					combined.Diagnostics = append(combined.Diagnostics, manifest.ID+": "+err.Error())
					continue
				}
				capture.Key = scopedCaptureKey(manifest.ID, capture.Key)
				if capture.Mode == "" {
					capture.Mode = "range-file"
				}
				combined.Captures = append(combined.Captures, capture)
			}
		}
		combined.Diagnostics = append(combined.Diagnostics, result.Diagnostics...)
		if result.Handled {
			combined.Handled = true
			if !item.builtin {
				handledByExternal = true
			}
		}
		if result.Decision == shared.DecisionStop {
			combined.Decision = shared.DecisionStop
			break
		}
	}

	m.resources.PublishCandidates(m.resources.FilterSelectedCandidates(combined.Resources))
	return combined
}

func validateResponseCapture(capture shared.ResponseCapture) error {
	key := strings.TrimSpace(capture.Key)
	if key == "" || len(key) > maxPluginCaptureKeySize || strings.IndexByte(key, 0) >= 0 {
		return fmt.Errorf("response capture key must contain 1 to %d safe bytes", maxPluginCaptureKeySize)
	}
	if capture.Mode != "" && capture.Mode != "range-file" {
		return fmt.Errorf("response capture %q has unsupported mode %q", key, capture.Mode)
	}
	return nil
}

func scopedCaptureKey(pluginID, key string) string {
	return pluginID + "\x00" + strings.TrimSpace(key)
}

func resourceUsesCapture(candidate shared.ResourceCandidate) bool {
	for _, track := range candidate.Tracks {
		if track.Executor == "capture-file" || track.CaptureKey != "" {
			return true
		}
	}
	return false
}

func limitObservationBody(obs *shared.Observation, permissions shared.PluginPermissions) {
	limit := permissions.BodyLimit
	if limit <= 0 {
		limit = defaultPluginBodyLimit
	}
	if limit <= 0 {
		return
	}
	if int64(len(obs.Request.Body)) > limit {
		obs.Request.Body = obs.Request.Body[:limit]
		obs.Request.Truncated = true
	}
	if obs.Response != nil && int64(len(obs.Response.Body)) > limit {
		obs.Response.Body = obs.Response.Body[:limit]
		obs.Response.Truncated = true
	}
}

func validateResourceActions(manifest shared.PluginManifest, actions []shared.ResourceAction) error {
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		if _, exists := seen[action.ID]; exists {
			return fmt.Errorf("resource action %q is duplicated", action.ID)
		}
		seen[action.ID] = struct{}{}
		if _, exists := manifest.Actions[action.ID]; !exists {
			return fmt.Errorf("resource action %q is not declared by the plugin", action.ID)
		}
		raw, err := json.Marshal(action.Data)
		if err != nil || len(raw) > maxPluginWASMOptions {
			return fmt.Errorf("resource action %q data exceeds %d bytes", action.ID, maxPluginWASMOptions)
		}
	}
	return nil
}

func (m *PluginManager) ResolveFileAction(resource shared.ResourceCandidate, actionID string) (shared.PluginActionDefinition, shared.DownloadStep, error) {
	m.mu.RLock()
	status, exists := m.statuses[resource.Source.PluginID]
	m.mu.RUnlock()
	if !exists || !status.Loaded || status.Builtin {
		return shared.PluginActionDefinition{}, shared.DownloadStep{}, fmt.Errorf("plugin %q is unavailable", resource.Source.PluginID)
	}
	definition, exists := status.Manifest.Actions[actionID]
	if !exists || definition.Kind != shared.PluginActionProcessFile {
		return shared.PluginActionDefinition{}, shared.DownloadStep{}, fmt.Errorf("resource action %q is unavailable", actionID)
	}
	var selected *shared.ResourceAction
	for index := range resource.Actions {
		if resource.Actions[index].ID == actionID {
			selected = &resource.Actions[index]
			break
		}
	}
	if selected == nil {
		return shared.PluginActionDefinition{}, shared.DownloadStep{}, fmt.Errorf("resource does not provide action %q", actionID)
	}
	options := map[string]interface{}{"processor": definition.Processor}
	if configured, ok := selected.Data["options"].(map[string]interface{}); ok {
		for key, value := range configured {
			if key != wasmProcessorIDOption && key != wasmProcessorOwnerKey {
				options[key] = value
			}
		}
	}
	step := []shared.DownloadStep{{Type: wasmProcessorType, Options: options}}
	if err := m.bindDownloadProcessors(resource.Source.PluginID, step); err != nil {
		return shared.PluginActionDefinition{}, shared.DownloadStep{}, err
	}
	if err := validateDownloadProcessor(step[0]); err != nil {
		return shared.PluginActionDefinition{}, shared.DownloadStep{}, err
	}
	return definition, step[0], nil
}

func (m *PluginManager) CreateDownloadPlan(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.DownloadPlan, error) {
	m.mu.RLock()
	registered := append([]managedPlugin(nil), m.plugins...)
	m.mu.RUnlock()
	var ownerManifest *shared.PluginManifest
	for _, item := range registered {
		manifest := item.runtime.Manifest()
		if resource.Source.PluginID != "" && manifest.ID != resource.Source.PluginID {
			continue
		}
		if resource.Source.PluginID != "" {
			manifestCopy := manifest
			ownerManifest = &manifestCopy
		}
		options.Settings = m.pluginSettings(manifest.ID)
		var plan shared.DownloadPlan
		var handled bool
		err := m.runtimeState(manifest.ID).run(ctx, func(callCtx context.Context) error {
			var callErr error
			plan, handled, callErr = item.runtime.Resolve(callCtx, resource, options)
			return callErr
		})
		if err != nil {
			return shared.DownloadPlan{}, err
		}
		if handled {
			plan = normalizeDownloadPlan(plan)
			if err := bindCaptureDownloadPlan(manifest, &plan); err != nil {
				return shared.DownloadPlan{}, err
			}
			if err := m.bindDownloadPlanProcessors(manifest.ID, &plan); err != nil {
				return shared.DownloadPlan{}, err
			}
			if err := validateMediaPlanPermissions(m.media, manifest, plan); err != nil {
				return shared.DownloadPlan{}, err
			}
			return plan, validateDownloadPlan(plan)
		}
	}
	plan := fallbackDownloadPlan(resource, options)
	if ownerManifest == nil {
		ownerManifest = &shared.PluginManifest{}
	}
	if err := bindCaptureDownloadPlan(*ownerManifest, &plan); err != nil {
		return shared.DownloadPlan{}, err
	}
	if err := m.bindDownloadPlanProcessors(resource.Source.PluginID, &plan); err != nil {
		return shared.DownloadPlan{}, err
	}
	if err := validateMediaPlanPermissions(m.media, *ownerManifest, plan); err != nil {
		return shared.DownloadPlan{}, err
	}
	return plan, validateDownloadPlan(plan)
}

func (m *PluginManager) Resolve(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.DownloadPlan, error) {
	return m.CreateDownloadPlan(ctx, resource, options)
}

func (m *PluginManager) RefreshResource(ctx context.Context, resource shared.ResourceCandidate, options shared.DownloadOptions) (shared.ResourceCandidate, string, error) {
	m.mu.RLock()
	registered := append([]managedPlugin(nil), m.plugins...)
	m.mu.RUnlock()
	for _, item := range registered {
		manifest := item.runtime.Manifest()
		if manifest.ID != resource.Source.PluginID {
			continue
		}
		refresher, ok := item.runtime.(shared.ResourceRefresher)
		if !ok {
			return resource, shared.ResourceRefreshUnsupported, nil
		}
		options.Settings = m.pluginSettings(manifest.ID)
		var result shared.ResourceRefreshResult
		var handled bool
		err := m.runtimeState(manifest.ID).run(ctx, func(callCtx context.Context) error {
			var callErr error
			result, handled, callErr = refresher.RefreshResource(callCtx, resource, options)
			return callErr
		})
		if err != nil {
			return resource, "", err
		}
		if !handled {
			return resource, shared.ResourceRefreshUnsupported, nil
		}
		if result.Status != "" && result.Status != shared.ResourceRefreshOK {
			return resource, result.Status, nil
		}
		updated := result.Resource
		updated.ID = resource.ID
		updated.DedupeKey = resource.DedupeKey
		updated.GroupKey = resource.GroupKey
		updated.ParentGroupKey = resource.ParentGroupKey
		updated.ParentID = resource.ParentID
		updated.Source.PluginID = manifest.ID
		updated.Source.PluginVersion = manifest.Version
		m.mu.RLock()
		updated.Source.PluginDigest = m.statuses[manifest.ID].Digest
		m.mu.RUnlock()
		updated.Lifecycle.DiscoveredAt = resource.Lifecycle.DiscoveredAt
		updated.Lifecycle.LastResolvedAt = time.Now().UnixMilli()
		updated.Lifecycle.Availability = shared.ResourceAvailabilityAvailable
		updated.Lifecycle.UnavailableReason = ""
		if err := validateCandidate(&updated); err != nil {
			return resource, "", fmt.Errorf("plugin returned an invalid refreshed resource: %w", err)
		}
		return updated, shared.ResourceRefreshOK, nil
	}
	return resource, shared.ResourceRefreshUnsupported, nil
}

func fallbackDownloadPlan(resource shared.ResourceCandidate, options shared.DownloadOptions) shared.DownloadPlan {
	selected := make(map[string]bool, len(options.SelectedTrackIDs))
	for _, id := range options.SelectedTrackIDs {
		selected[id] = true
	}
	inputs := make([]shared.DownloadInput, 0, len(resource.Tracks))
	for _, track := range resource.Tracks {
		if len(selected) > 0 && !selected[track.ID] {
			continue
		}
		if track.URL == "" && track.CaptureKey == "" {
			continue
		}
		inputs = append(inputs, shared.DownloadInput{
			ID: track.ID, Executor: track.Executor, URL: track.URL, CaptureKey: track.CaptureKey, Headers: track.Headers,
			Extension: track.Extension, Processors: track.Processors,
		})
		if inputs[len(inputs)-1].Executor == "" {
			inputs[len(inputs)-1].Executor = "http-file"
		}
	}
	output := shared.DownloadOutput{}
	if len(inputs) == 1 {
		output.Input = inputs[0].ID
		output.Extension = inputs[0].Extension
	}
	return shared.DownloadPlan{Inputs: inputs, Output: output}
}

func bindCaptureDownloadPlan(manifest shared.PluginManifest, plan *shared.DownloadPlan) error {
	for index := range plan.Inputs {
		input := &plan.Inputs[index]
		if input.Executor != "capture-file" {
			if input.CaptureKey != "" {
				return fmt.Errorf("download input %q uses captureKey without capture-file executor", input.ID)
			}
			continue
		}
		if !manifest.Permissions.Has("capture-response-body") {
			return fmt.Errorf("download input %q requires capture-response-body", input.ID)
		}
		if err := validateResponseCapture(shared.ResponseCapture{Key: input.CaptureKey, Mode: "range-file"}); err != nil {
			return fmt.Errorf("download input %q: %w", input.ID, err)
		}
		input.CaptureKey = scopedCaptureKey(manifest.ID, input.CaptureKey)
	}
	return nil
}

func normalizeDownloadPlan(plan shared.DownloadPlan) shared.DownloadPlan {
	if plan.Output.Input == "" && len(plan.Inputs) == 1 && len(plan.Pipeline) == 0 {
		plan.Output.Input = plan.Inputs[0].ID
	}
	if plan.Output.Extension == "" && len(plan.Inputs) == 1 {
		plan.Output.Extension = plan.Inputs[0].Extension
	}
	return plan
}

func (m *PluginManager) bindDownloadPlanProcessors(pluginID string, plan *shared.DownloadPlan) error {
	for index := range plan.Inputs {
		if err := m.bindDownloadProcessors(pluginID, plan.Inputs[index].Processors); err != nil {
			return err
		}
	}
	return m.bindDownloadProcessors(pluginID, plan.Output.Processors)
}

func (m *PluginManager) BodyLimit(obs shared.Observation) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var maximum int64
	capability := "read-response-body"
	if obs.Stage == shared.StageRequest {
		capability = "read-request-body"
	}
	for _, item := range m.plugins {
		manifest := item.runtime.Manifest()
		if pluginMatches(manifest, obs) && manifest.Permissions.Has(capability) {
			if !pluginWantsBody(item.runtime, manifest, obs) {
				continue
			}
			limit := manifest.Permissions.BodyLimit
			if limit <= 0 {
				limit = defaultPluginBodyLimit
			}
			if limit > maximum {
				maximum = limit
			}
		}
	}
	return maximum
}
