package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	m "res-downloader/internal/model"
	"res-downloader/internal/operation"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (pm *PluginManager) SetOperations(service *operation.Service) {
	pm.mu.Lock()
	pm.operations = service
	pm.mu.Unlock()
}
func (pm *PluginManager) OperationService() *operation.Service {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.operations
}
func (pm *PluginManager) SetOperationDownloadHandler(handler func(context.Context, m.ResourceCandidate) (string, error)) {
	pm.mu.Lock()
	pm.operationDownload = handler
	pm.mu.Unlock()
}

// OperationDownloads provides actual scheduler records for reconciling downloads
// created after resource publication but before finalization completes.
func (pm *PluginManager) SetOperationDownloadLookup(lookup func() []m.DownloadTaskRecord) {
	pm.mu.Lock()
	pm.operationDownloads = lookup
	pm.mu.Unlock()
}
func (pm *PluginManager) OperationDownloads() []m.DownloadTaskRecord {
	pm.mu.RLock()
	lookup := pm.operationDownloads
	pm.mu.RUnlock()
	if lookup == nil {
		return nil
	}
	return lookup()
}
func (pm *PluginManager) SetArtifactHandler(handler func(m.OperationArtifact) (io.ReadSeekCloser, m.OperationArtifact, error)) {
	pm.mu.Lock()
	pm.artifactHandler = handler
	pm.mu.Unlock()
}
func (pm *PluginManager) OpenArtifact(a m.OperationArtifact) (io.ReadSeekCloser, m.OperationArtifact, error) {
	pm.mu.RLock()
	h := pm.artifactHandler
	pm.mu.RUnlock()
	if h == nil {
		return nil, a, errors.New("artifact reader unavailable")
	}
	return h(a)
}
func (pm *PluginManager) Operations() []m.OperationInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := []m.OperationInfo{}
	for _, p := range pm.plugins {
		manifest := p.runtime.Manifest()
		if !manifest.IsEnabled() {
			continue
		}
		for id, op := range manifest.Operations {
			out = append(out, m.OperationInfo{RuntimeID: strconv.FormatUint(pm.operationGeneration, 10), PluginID: manifest.ID, PluginVersion: manifest.Version, OperationID: id, Definition: op})
		}
	}
	return out
}
func (pm *PluginManager) Sessions() []m.OperationSession {
	infos := pm.Operations()
	out := []m.OperationSession{}
	if pm.pages == nil {
		return out
	}
	pm.pages.mu.Lock()
	defer pm.pages.mu.Unlock()
	pm.pages.pruneLocked(time.Now())
	for _, p := range pm.pages.sessions {
		if p.documentClosed {
			continue
		}
		p.eventsMu.Lock()
		connected := p.events > 0
		p.eventsMu.Unlock()
		view := m.OperationSession{PluginID: p.pluginID, ScriptID: p.scriptID, PageSessionID: p.id, Title: p.title, PageURL: p.pageURL, Connected: connected, Ready: p.ready, Login: p.login, Revision: p.revision, Operations: []string{}}
		if view.Login == "" {
			view.Login = "unknown"
		}
		u, _ := url.Parse(p.pageURL)
		for _, info := range infos {
			if info.PluginID != p.pluginID || info.Definition.PageScript != p.scriptID {
				continue
			}
			if info.RuntimeID != strconv.FormatUint(p.operationGeneration, 10) {
				continue
			}
			if len(info.Definition.PageMatch) > 0 && (u == nil || !matchesPageScript(m.PluginPageScript{Match: info.Definition.PageMatch}, m.RequestSnapshot{URL: p.pageURL, Host: u.Host, Path: u.Path})) {
				continue
			}
			view.Operations = append(view.Operations, info.OperationID)
		}
		sort.Strings(view.Operations)
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PageSessionID < out[j].PageSessionID })
	return out
}

func (pm *PluginManager) ReloadIdentity(sessionID string) (m.OperationReloadIdentity, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pm.pages == nil {
		return m.OperationReloadIdentity{}, false
	}
	pm.pages.mu.RLock()
	defer pm.pages.mu.RUnlock()
	p := pm.pages.sessions[sessionID]
	if p == nil || p.documentClosed || p.operationGeneration != pm.operationGeneration {
		return m.OperationReloadIdentity{}, false
	}
	return m.OperationReloadIdentity{PluginID: p.pluginID, ScriptID: p.scriptID, PageURL: p.pageURL,
		Login: p.login, Context: p.context, RuntimeID: strconv.FormatUint(p.operationGeneration, 10)}, true
}
func (pm *PluginManager) ResolveOperationRequest(req m.OperationRequest) (m.OperationRequest, error) {
	if req.ResourceID == "" && req.ActionID == "" {
		return req, nil
	}
	source, ok := pm.resources.(interface {
		Candidate(string) (m.ResourceCandidate, bool)
	})
	if !ok {
		return req, errors.New("resource service unavailable")
	}
	resource, ok := source.Candidate(req.ResourceID)
	if !ok {
		return req, errors.New("resource not found")
	}
	definition, action, err := pm.ResolveResourceAction(resource, req.ActionID)
	if err != nil {
		return req, err
	}
	if definition.Kind != m.PluginActionOperation || req.PluginID != resource.Source.PluginID || req.OperationID != definition.Operation {
		return req, errors.New("resource operation ownership mismatch")
	}
	req.Input = action.Data
	return req, nil
}
func (pm *PluginManager) Send(execution m.OperationExecution, req m.OperationRequest, kind string) error {
	pm.pages.mu.Lock()
	defer pm.pages.mu.Unlock()
	p := pm.pages.sessions[execution.PageSessionID]
	if p == nil || p.documentClosed || p.pluginID != execution.PluginID || p.revision != execution.Revision {
		return errors.New("page changed")
	}
	p.eventsMu.Lock()
	connected := p.events > 0
	p.eventsMu.Unlock()
	if !connected {
		return errors.New("page disconnected")
	}
	message := map[string]interface{}{"protocol": 1, "type": "operation-" + kind, "executionId": execution.ExecutionID, "operationId": execution.OperationID, "revision": execution.Revision}
	if kind == "invoke" {
		message["reloadCount"] = execution.ReloadCount
		message["input"] = req.Input
		message["cursor"] = req.Cursor
		message["limit"] = req.Limit
		if req.ResourceID != "" {
			message["resource"] = map[string]string{"id": req.ResourceID, "actionId": req.ActionID}
		}
	}
	raw, err := json.Marshal(message)
	if err != nil || len(raw) > maxPageBridgeMessageSize {
		return errors.New("operation message too large")
	}
	select {
	case p.messages <- raw:
		return nil
	default:
		return errors.New("page queue full")
	}
}

type operationOrigin struct {
	scheme string
	host   string
	port   uint16
}

func parseOperationOrigin(raw string) (operationOrigin, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Opaque != "" || u.Hostname() == "" || strings.HasSuffix(u.Host, ":") {
		return operationOrigin{}, false
	}
	origin := operationOrigin{scheme: strings.ToLower(u.Scheme), host: strings.ToLower(u.Hostname())}
	switch origin.scheme {
	case "http":
		origin.port = 80
	case "https":
		origin.port = 443
	default:
		return operationOrigin{}, false
	}
	if strings.HasPrefix(u.Host, "[") || strings.Contains(origin.host, ":") {
		address, err := netip.ParseAddr(origin.host)
		if !strings.HasPrefix(u.Host, "[") || err != nil || !address.Is6() || address.Zone() != "" {
			return operationOrigin{}, false
		}
		origin.host = address.String()
	}
	if port := u.Port(); port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			return operationOrigin{}, false
		}
		origin.port = uint16(number)
	}
	return origin, true
}

func sameOperationOrigin(pageURL, sessionOrigin string) bool {
	// MITM requests may retain a default CONNECT port that browser URLs omit.
	page, pageOK := parseOperationOrigin(pageURL)
	session, sessionOK := parseOperationOrigin(sessionOrigin)
	return pageOK && sessionOK && page == session
}

func (pm *PluginManager) handleOperationRequest(request *http.Request, p *pageBridgeSession, action string) *http.Response {
	reply := func(status int, data interface{}, err error) *http.Response {
		if err != nil {
			return pageBridgeJSONResponse(request, status, map[string]interface{}{"ok": false, "error": err.Error()})
		}
		return pageBridgeJSONResponse(request, status, map[string]interface{}{"ok": true, "data": data})
	}
	if !pm.pages.allowMessage(p) {
		return reply(429, nil, errors.New("page rate limit exceeded"))
	}
	service := pm.OperationService()
	if service == nil {
		return reply(503, nil, errors.New("operation service unavailable"))
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, maxPageBridgeMessageSize+1))
	if err != nil || len(raw) > maxPageBridgeMessageSize {
		return reply(413, nil, errors.New("operation request too large"))
	}
	decode := func(target interface{}) error {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if e := d.Decode(target); e != nil {
			return e
		}
		if d.Decode(new(interface{})) != io.EOF {
			return errors.New("unexpected trailing data")
		}
		return nil
	}
	pm.pages.mu.RLock()
	revision := p.revision
	pm.pages.mu.RUnlock()
	switch action {
	case "operation-ready":
		var state struct {
			Title   string `json:"title"`
			PageURL string `json:"pageUrl"`
			Ready   bool   `json:"ready"`
			Login   string `json:"login"`
			Context string `json:"context"`
		}
		if err = decode(&state); err != nil {
			return reply(400, nil, err)
		}
		if !sameOperationOrigin(state.PageURL, p.origin) || len(state.PageURL) > 2048 || len(state.Title) > 512 || len(state.Context) > 128 || (state.Login != "unknown" && state.Login != "authenticated" && state.Login != "required") {
			return reply(400, nil, errors.New("invalid page readiness"))
		}
		pm.pages.mu.Lock()
		if pm.pages.sessions[p.id] != p || p.documentClosed {
			pm.pages.mu.Unlock()
			return reply(409, nil, errors.New("page expired"))
		}
		// Another ready request may have changed the revision before this lock.
		revision = p.revision
		if p.pageURL != state.PageURL || p.context != state.Context || p.login != state.Login || p.ready != state.Ready {
			revision, err = randomPageBridgeValue(16)
			if err == nil {
				p.revision = revision
			}
		}
		if err == nil {
			p.pageURL = state.PageURL
			p.title = state.Title
			p.context = state.Context
			p.login = state.Login
			p.ready = state.Ready
		}
		pm.pages.mu.Unlock()
		return reply(200, map[string]string{"revision": revision}, err)
	case "operation-claim":
		var input struct {
			ExecutionID string `json:"executionId"`
			Revision    string `json:"revision"`
		}
		if err = decode(&input); err == nil {
			if input.Revision != revision {
				err = errors.New("page revision changed")
			} else {
				err = service.Claim(input.ExecutionID, p.id, revision)
			}
		}
	case "operation-reload", "operation-resume", "operation-reload-abort":
		var input struct {
			ExecutionID string `json:"executionId"`
			Revision    string `json:"revision"`
			Token       string `json:"token,omitempty"`
		}
		if err = decode(&input); err == nil {
			if len(input.ExecutionID) != 32 || len(input.Token) > 128 || input.Revision != revision {
				err = errors.New("invalid operation reload request")
			} else if action == "operation-reload" {
				var ticket m.OperationReloadTicket
				ticket, err = service.PrepareReload(input.ExecutionID, p.id, revision)
				if err == nil {
					return reply(200, ticket, nil)
				}
			} else if action == "operation-resume" {
				err = waitForOperationReload(request.Context(), func() error {
					return service.ResumeReload(input.ExecutionID, input.Token, p.id, revision)
				})
			} else {
				err = service.AbandonReload(input.ExecutionID, input.Token, p.id, revision)
			}
		}
	case "operation-report":
		var report m.OperationReport
		if err = decode(&report); err == nil {
			// Reports belong to the invoked revision, even after setState has
			// advanced the page. The service only accepts stale-owner terminal
			// reports as stop acknowledgements, never as a new result.
			if report.Revision == "" {
				err = errors.New("operation report requires revision")
			} else {
				err = service.Report(request.Context(), p.id, report.Revision, report)
			}
		}
	}
	if err != nil {
		return reply(409, nil, err)
	}
	return reply(200, nil, nil)
}

func waitForOperationReload(ctx context.Context, resume func() error) error {
	// Browser navigation, sendBeacon(close), and SSE disconnect are independent
	// requests. Allow their short transport overlap without consuming the ticket
	// or submitting a second execution. Every attempt revalidates the owner,
	// cancellation, page identity and original deadline under the service lock.
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := resume()
		if !errors.Is(err, operation.ErrReloadPageConnected) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return err
		case <-tick.C:
		}
	}
}

func (pm *PluginManager) Finalize(ctx context.Context, execution m.OperationExecution, definition m.OperationDefinition, data json.RawMessage) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
	pm.reloadMu.Lock()
	defer pm.reloadMu.Unlock()
	resources := []string{}
	downloads := []string{}
	artifacts := []m.OperationArtifact{}
	failure := func(err error) (json.RawMessage, []string, []string, []m.OperationArtifact, error) {
		return data, resources, downloads, artifacts, err
	}
	pm.mu.RLock()
	plugins := append([]managedPlugin(nil), pm.plugins...)
	download := pm.operationDownload
	pm.mu.RUnlock()
	var selected *managedPlugin
	for _, item := range plugins {
		manifest := item.runtime.Manifest()
		if manifest.ID == execution.PluginID && manifest.Version == execution.PluginVersion && manifest.IsEnabled() {
			cp := item
			selected = &cp
			break
		}
	}
	if selected == nil {
		return failure(errors.New("plugin unavailable"))
	}
	manifest := selected.runtime.Manifest()
	pm.pages.mu.RLock()
	p := pm.pages.sessions[execution.PageSessionID]
	var contextValue m.PageMessageContext
	if p != nil {
		contextValue = m.PageMessageContext{PageSessionID: p.id, ScriptID: p.scriptID, PageURL: p.pageURL, Origin: p.origin}
	}
	pm.pages.mu.RUnlock()
	if p == nil || contextValue.ScriptID != definition.PageScript {
		return failure(errors.New("page unavailable"))
	}
	contextValue.Settings = pm.pluginSettings(manifest.ID)
	var pageData interface{}
	if err := json.Unmarshal(data, &pageData); err != nil {
		return failure(err)
	}
	result := m.PageMessageResult{OK: true}
	handler, ok := pageMessageHandler(selected.runtime)
	// The synchronous hook is a result mapper, never an asynchronous page executor.
	if ok {
		var handled bool
		err := pm.runtimeState(manifest.ID).run(ctx, func(callCtx context.Context) error {
			var e error
			result, handled, e = handler.HandlePageMessage(callCtx, map[string]interface{}{"type": "operation-result", "operationId": execution.OperationID, "executionId": execution.ExecutionID, "data": pageData}, contextValue)
			return e
		})
		if err != nil {
			return failure(err)
		}
		if handled && !result.OK {
			return failure(errors.New("plugin rejected result"))
		}
		if !handled {
			result = m.PageMessageResult{OK: true}
		}
	}
	if result.Data != nil {
		raw, err := json.Marshal(result.Data)
		if err != nil {
			return failure(err)
		}
		data = raw
	}
	var value interface{}
	if len(data) > operation.MaxResultBytes || json.Unmarshal(data, &value) != nil || operation.ValidateValue(definition.OutputSchema, value) != nil {
		return failure(errors.New("invalid mapped result"))
	}
	effects := map[string]bool{}
	for _, e := range definition.Effects {
		effects[e] = true
	}
	if (len(result.Resources) > 0 && (!effects["publish"] || !manifest.Permissions.Has("emit-resource"))) || (result.AutoDownload && (!effects["download"] || !manifest.Permissions.Has("enqueue-download"))) {
		return failure(errors.New("operation effect is not permitted"))
	}
	rawResult, marshalErr := json.Marshal(result)
	if marshalErr != nil || len(rawResult) > 2*1024*1024 {
		return failure(errors.New("mapped resources exceed 2 MiB"))
	}
	trackCount := 0
	for _, candidate := range result.Resources {
		trackCount += len(candidate.Tracks)
	}
	if trackCount > 128 {
		return failure(errors.New("operation exceeds 128 tracks"))
	}
	if len(result.Resources) > operation.MaxItems || (result.AutoDownload && len(result.Resources) > maxPageAutoDownloads) {
		return failure(errors.New("too many resources"))
	}
	// Validate every candidate before publishing any of them. Ownership always comes from the host.
	for i := range result.Resources {
		c := &result.Resources[i]
		c.ID = ""
		c.ParentID = ""
		c.DedupeKey = ""
		pm.mu.RLock()
		digest := pm.statuses[manifest.ID].Digest
		pm.mu.RUnlock()
		c.Source = m.ResourceSource{PluginID: manifest.ID, PluginVersion: manifest.Version, PluginDigest: digest, PageURL: contextValue.PageURL}
		if c.GroupKey == "" {
			return failure(errors.New("operation resources require groupKey"))
		}
		if resourceUsesCapture(*c) && !manifest.Permissions.Has("capture-response-body") {
			return failure(errors.New("capture permission required"))
		}
		for _, track := range c.Tracks {
			if track.URL != "" {
				u, e := url.Parse(track.URL)
				if e != nil || !domainAllowed(manifest.Permissions.Domains, u.Hostname()) {
					return failure(errors.New("resource outside plugin domains"))
				}
			}
		}
		if err := validateResourceActions(manifest, c.Actions); err != nil {
			return failure(err)
		}
		if err := validateCandidate(c); err != nil {
			return failure(err)
		}
		if len(c.Tracks) > 0 {
			plan, err := pm.CreateDownloadPlan(ctx, *c, m.DownloadOptions{})
			if err != nil {
				return failure(err)
			}
			for _, input := range plan.Inputs {
				if input.URL != "" {
					u, e := url.Parse(input.URL)
					if e != nil || !domainAllowed(manifest.Permissions.Domains, u.Hostname()) {
						return failure(errors.New("download input outside plugin domains"))
					}
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return failure(err)
	}
	// Keep page generation stable while committing host effects. The plugin
	// mapper and download-plan work have already finished outside this lock.
	pm.pages.mu.RLock()
	current := pm.pages.sessions[execution.PageSessionID]
	if current != p || current.documentClosed || current.revision != execution.Revision || !current.ready {
		pm.pages.mu.RUnlock()
		return failure(errors.New("page changed before publication"))
	}
	published := []m.ResourceCandidate{}
	if len(result.Resources) > 0 {
		sink, ok := pm.resources.(interface {
			PublishOperationCandidates([]m.ResourceCandidate) ([]m.ResourceCandidate, error)
		})
		if !ok {
			pm.pages.mu.RUnlock()
			return failure(errors.New("durable resource publication unavailable"))
		}
		var err error
		published, err = sink.PublishOperationCandidates(result.Resources)
		if err != nil {
			pm.pages.mu.RUnlock()
			return failure(err)
		}
	}
	pm.pages.mu.RUnlock()
	for _, stored := range published {
		resources = append(resources, stored.ID)
	}
	for _, stored := range published {

		for _, track := range stored.Tracks {
			if track.Executor == "capture-file" && len(track.Processors) == 0 {
				artifactID, e := randomPageBridgeValue(16)
				if e != nil {
					return failure(e)
				}
				artifacts = append(artifacts, m.OperationArtifact{ArtifactID: artifactID, ResourceID: stored.ID, MIME: track.MIME, Size: track.Size, Status: "available", CaptureKey: scopedCaptureKey(manifest.ID, track.CaptureKey), ExpiresAt: time.Now().Add(24 * time.Hour).UnixMilli()})
			}
		}
		if result.AutoDownload {
			if err := ctx.Err(); err != nil {
				return failure(err)
			}
			if download == nil {
				return failure(errors.New("download service unavailable"))
			}
			taskID, err := download(ctx, stored)
			if taskID != "" {
				downloads = append(downloads, taskID)
			}
			if err != nil {
				return failure(err)
			}
			artifactID, e := randomPageBridgeValue(16)
			if e != nil {
				return failure(e)
			}
			mime := "application/octet-stream"
			if len(stored.Tracks) == 1 {
				mime = stored.Tracks[0].MIME
			}
			artifacts = append(artifacts, m.OperationArtifact{ArtifactID: artifactID, ResourceID: stored.ID, DownloadTaskID: taskID, MIME: mime, Status: "pending"})
		}
	}
	return data, resources, downloads, artifacts, nil
}
