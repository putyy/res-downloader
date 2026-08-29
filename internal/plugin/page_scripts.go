package plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	shared "res-downloader/internal/model"
)

const (
	pageBridgePrefix          = "/.well-known/res-downloader/page-bridge/"
	maxPageBridgeMessageSize  = 64 * 1024
	maxPageCaptureChunkSize   = 32 * 1024 * 1024
	maxPageCaptureKeys        = 4
	maxPageCaptureSessionSize = int64(16 * 1024 * 1024 * 1024)
	maxPageSessionsPerPlugin  = 32
	maxPageSessionQueue       = 64
	pageSessionIdleTimeout    = 5 * time.Minute
	pageMessageWindow         = 10 * time.Second
	maxPageMessagesPerWindow  = 100
	maxPageEventConnections   = 2
	maxPageAutoDownloads      = 4
)

type PageCaptureStore interface {
	StartStream(string) error
	AppendStream(string, []byte) (int64, error)
	CompleteStream(string) error
	AbortStream(string) error
}

type loadedPageScript struct {
	definition shared.PluginPageScript
	source     string
}

type pageBridgeSession struct {
	id           string
	token        string
	pluginID     string
	scriptID     string
	pageURL      string
	host         string
	origin       string
	createdAt    time.Time
	lastSeen     time.Time
	messages     chan []byte
	done         chan struct{}
	closeOnce    sync.Once
	eventsMu     sync.Mutex
	events       int
	windowAt     time.Time
	windowN      int
	captureKeys  map[string]int64
	captureBytes int64
}

func (s *pageBridgeSession) close() { s.closeOnce.Do(func() { close(s.done) }) }

type pageBridgeHub struct {
	mu       sync.RWMutex
	sessions map[string]*pageBridgeSession
	logger   *Logger
}

func newPageBridgeHub(logger *Logger) *pageBridgeHub {
	return &pageBridgeHub{sessions: make(map[string]*pageBridgeSession), logger: logger}
}

func loadManagedPageScripts(directory string, manifest shared.PluginManifest) ([]loadedPageScript, error) {
	loaded := make([]loadedPageScript, 0, len(manifest.PageScripts))
	for _, definition := range manifest.PageScripts {
		path, err := securePluginFilePath(directory, definition.Entry)
		if err != nil {
			return nil, fmt.Errorf("page script %q: %w", definition.ID, err)
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("page script %q: %w", definition.ID, err)
		}
		if len(source) == 0 || int64(len(source)) > maxPluginPageScriptSize {
			return nil, fmt.Errorf("page script %q size must be between 1 and %d bytes", definition.ID, maxPluginPageScriptSize)
		}
		loaded = append(loaded, loadedPageScript{definition: definition, source: string(source)})
	}
	return loaded, nil
}

func (m *PluginManager) PageScripts(request shared.RequestSnapshot) []shared.PageScriptInjection {
	if m == nil || m.pages == nil {
		return nil
	}
	m.mu.RLock()
	plugins := append([]managedPlugin(nil), m.plugins...)
	m.mu.RUnlock()

	out := make([]shared.PageScriptInjection, 0)
	for _, item := range plugins {
		manifest := item.runtime.Manifest()
		if !manifest.IsEnabled() || !manifest.Permissions.Has("inject-page-script") {
			continue
		}
		for _, script := range item.pageScripts {
			if !matchesPageScript(script.definition, request) {
				continue
			}
			injection := shared.PageScriptInjection{
				PluginID: manifest.ID, PluginVersion: manifest.Version, ScriptID: script.definition.ID,
				Source: script.source, Frames: script.definition.Frames, Bridge: script.definition.Bridge,
				Capture: manifest.Permissions.Has("capture-response-body"),
			}
			if injection.Frames == "" {
				injection.Frames = "top"
			}
			if injection.Bridge && manifest.Permissions.Has("page-bridge") {
				session := m.pages.create(manifest.ID, script.definition.ID, request.URL, request.Host)
				if session != nil {
					injection.PageSessionID = session.id
					injection.BridgeToken = session.token
				}
			}
			out = append(out, injection)
		}
	}
	return out
}

func matchesPageScript(script shared.PluginPageScript, request shared.RequestSnapshot) bool {
	for _, match := range script.Match {
		if match.Host != "" && !wildcardMatch(match.Host, hostname(request.Host)) {
			continue
		}
		if match.Path != "" && !wildcardMatch(match.Path, request.Path) {
			continue
		}
		if match.URL != "" && !wildcardMatch(match.URL, request.URL) {
			continue
		}
		return true
	}
	return false
}

func (h *pageBridgeHub) create(pluginID, scriptID, pageURL, pageHost string) *pageBridgeSession {
	sessionID, err := randomPageBridgeValue(16)
	if err != nil {
		return nil
	}
	token, err := randomPageBridgeValue(24)
	if err != nil {
		return nil
	}
	now := time.Now()
	session := &pageBridgeSession{
		id: sessionID, token: token, pluginID: pluginID, scriptID: scriptID,
		pageURL: pageURL, host: pageHost, origin: pageOrigin(pageURL), createdAt: now, lastSeen: now,
		messages: make(chan []byte, maxPageSessionQueue), done: make(chan struct{}),
		captureKeys: make(map[string]int64),
	}
	h.mu.Lock()
	h.pruneLocked(now)
	pluginSessions := make([]*pageBridgeSession, 0)
	for _, current := range h.sessions {
		if current.pluginID == pluginID {
			pluginSessions = append(pluginSessions, current)
		}
	}
	if len(pluginSessions) >= maxPageSessionsPerPlugin {
		var oldest *pageBridgeSession
		for _, current := range pluginSessions {
			if pageBridgeSessionBusy(current) {
				continue
			}
			if oldest == nil || current.createdAt.Before(oldest.createdAt) {
				oldest = current
			}
		}
		if oldest == nil {
			if h.logger != nil {
				h.logger.Warn().Str("plugin", pluginID).Msg("page bridge session creation rejected because all sessions are busy")
			}
			h.mu.Unlock()
			return nil
		}
		delete(h.sessions, oldest.id)
		oldest.close()
		if h.logger != nil {
			h.logger.Info().Str("plugin", pluginID).Str("pageSessionId", oldest.id).Msg("page bridge idle session evicted at capacity")
		}
	}
	h.sessions[session.id] = session
	h.mu.Unlock()
	return session
}

func randomPageBridgeValue(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func pageOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (h *pageBridgeHub) pruneLocked(now time.Time) {
	for id, session := range h.sessions {
		if now.Sub(session.lastSeen) <= pageSessionIdleTimeout || pageBridgeSessionBusy(session) {
			continue
		}
		delete(h.sessions, id)
		session.close()
	}
}

func pageBridgeSessionBusy(session *pageBridgeSession) bool {
	if len(session.captureKeys) > 0 {
		return true
	}
	session.eventsMu.Lock()
	defer session.eventsMu.Unlock()
	return session.events > 0
}

func (h *pageBridgeHub) closeAll() {
	if h == nil {
		return
	}
	h.mu.Lock()
	count := len(h.sessions)
	for id, session := range h.sessions {
		delete(h.sessions, id)
		session.close()
	}
	h.mu.Unlock()
	if count > 0 && h.logger != nil {
		h.logger.Warn().Int("sessions", count).Msg("page bridge sessions closed by plugin reload")
	}
}

func (h *pageBridgeHub) closeSession(session *pageBridgeSession) (bool, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if current := h.sessions[session.id]; current != session {
		return false, false
	}
	if len(session.captureKeys) > 0 {
		session.lastSeen = time.Now()
		if h.logger != nil {
			h.logger.Warn().Str("plugin", session.pluginID).Str("pageSessionId", session.id).Msg("page bridge close deferred while capture is active")
		}
		return false, true
	}
	delete(h.sessions, session.id)
	session.close()
	return true, false
}

func (h *pageBridgeHub) session(id, token string) (*pageBridgeSession, bool) {
	if h == nil {
		return nil, false
	}
	h.mu.Lock()
	h.pruneLocked(time.Now())
	session, exists := h.sessions[id]
	if exists && session.token == token {
		session.lastSeen = time.Now()
	} else {
		session = nil
		exists = false
	}
	h.mu.Unlock()
	return session, exists
}

func (h *pageBridgeHub) touch(session *pageBridgeSession) {
	h.mu.Lock()
	if current := h.sessions[session.id]; current == session {
		session.lastSeen = time.Now()
	}
	h.mu.Unlock()
}

func (h *pageBridgeHub) allowMessage(session *pageBridgeSession) bool {
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	if current := h.sessions[session.id]; current != session {
		return false
	}
	if session.windowAt.IsZero() || now.Sub(session.windowAt) >= pageMessageWindow {
		session.windowAt = now
		session.windowN = 0
	}
	if session.windowN >= maxPageMessagesPerWindow {
		return false
	}
	session.windowN++
	session.lastSeen = now
	return true
}

func (h *pageBridgeHub) startCapture(session *pageBridgeSession, key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if current := h.sessions[session.id]; current != session {
		return false
	}
	if _, exists := session.captureKeys[key]; !exists {
		if len(session.captureKeys) >= maxPageCaptureKeys {
			return false
		}
		session.captureKeys[key] = 0
	} else {
		session.captureKeys[key] = 0
	}
	session.lastSeen = time.Now()
	return true
}

func (h *pageBridgeHub) reserveCaptureBytes(session *pageBridgeSession, key string, size int64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if current := h.sessions[session.id]; current != session {
		return false
	}
	if _, exists := session.captureKeys[key]; !exists || size <= 0 || size > maxPageCaptureSessionSize-session.captureBytes {
		return false
	}
	session.captureBytes += size
	session.captureKeys[key] += size
	session.lastSeen = time.Now()
	return true
}

func (h *pageBridgeHub) hasCapture(session *pageBridgeSession, key string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if current := h.sessions[session.id]; current != session {
		return false
	}
	_, exists := session.captureKeys[key]
	return exists
}

func (h *pageBridgeHub) finishCapture(session *pageBridgeSession, key string) {
	h.mu.Lock()
	if current := h.sessions[session.id]; current == session {
		delete(session.captureKeys, key)
	}
	h.mu.Unlock()
}

func (h *pageBridgeHub) releaseCaptureBytes(session *pageBridgeSession, key string, size int64) {
	h.mu.Lock()
	if current := h.sessions[session.id]; current == session {
		session.captureBytes -= size
		if session.captureBytes < 0 {
			session.captureBytes = 0
		}
		session.captureKeys[key] -= size
		if session.captureKeys[key] < 0 {
			session.captureKeys[key] = 0
		}
	}
	h.mu.Unlock()
}

func (h *pageBridgeHub) send(pluginID, sessionID string, message interface{}) error {
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(raw) > maxPageBridgeMessageSize {
		return fmt.Errorf("page message exceeds %d bytes", maxPageBridgeMessageSize)
	}
	h.mu.RLock()
	session := h.sessions[sessionID]
	h.mu.RUnlock()
	if session == nil || session.pluginID != pluginID {
		return errors.New("page session not found")
	}
	select {
	case session.messages <- raw:
		return nil
	default:
		return errors.New("page session message queue is full")
	}
}

func (h *pageBridgeHub) broadcast(pluginID string, filter map[string]interface{}, message interface{}) int {
	h.mu.RLock()
	sessions := make([]*pageBridgeSession, 0)
	for _, session := range h.sessions {
		if session.pluginID == pluginID && pageSessionMatches(session, filter) {
			sessions = append(sessions, session)
		}
	}
	h.mu.RUnlock()
	sent := 0
	for _, session := range sessions {
		if h.send(pluginID, session.id, message) == nil {
			sent++
		}
	}
	return sent
}

func (h *pageBridgeHub) list(pluginID string, filter map[string]interface{}) []map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]map[string]string, 0)
	for _, session := range h.sessions {
		if session.pluginID != pluginID || !pageSessionMatches(session, filter) {
			continue
		}
		out = append(out, map[string]string{
			"pageSessionId": session.id, "scriptId": session.scriptID, "pageUrl": session.pageURL, "origin": session.origin,
		})
	}
	return out
}

func pageSessionMatches(session *pageBridgeSession, filter map[string]interface{}) bool {
	if filter == nil {
		return true
	}
	if value, _ := filter["pageSessionId"].(string); value != "" && value != session.id {
		return false
	}
	if value, _ := filter["scriptId"].(string); value != "" && value != session.scriptID {
		return false
	}
	if value, _ := filter["pageUrl"].(string); value != "" && !wildcardMatch(value, session.pageURL) {
		return false
	}
	if value, _ := filter["host"].(string); value != "" {
		parsed, _ := url.Parse(session.pageURL)
		if parsed == nil || !wildcardMatch(value, parsed.Hostname()) {
			return false
		}
	}
	return true
}

func (m *PluginManager) HandlePageBridge(request *http.Request) (*http.Response, bool) {
	if m == nil || m.pages == nil || request == nil || !strings.HasPrefix(request.URL.Path, pageBridgePrefix) {
		return nil, false
	}
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, pageBridgePrefix), "/")
	if len(parts) != 3 {
		return pageBridgeJSONResponse(request, http.StatusNotFound, map[string]interface{}{"ok": false, "error": "page bridge not found"}), true
	}
	session, exists := m.pages.session(parts[0], parts[1])
	if !exists {
		if m.logger != nil {
			m.logger.Warn().Str("pageSessionId", parts[0]).Str("action", parts[2]).Msg("page bridge request rejected because the session was not found or the token did not match")
		}
		return pageBridgeJSONResponse(request, http.StatusForbidden, map[string]interface{}{"ok": false, "error": "page bridge session is invalid"}), true
	}
	if !pageBridgeRequestAllowed(session, request) {
		if m.logger != nil {
			m.logger.Warn().Str("pageSessionId", session.id).Str("origin", request.Header.Get("Origin")).Str("host", request.Host).Str("pageOrigin", session.origin).Str("pageHost", session.host).Msg("page bridge request rejected because its origin or host did not match")
		}
		return pageBridgeJSONResponse(request, http.StatusForbidden, map[string]interface{}{"ok": false, "error": "page bridge request origin is invalid"}), true
	}
	switch parts[2] {
	case "events":
		if request.Method != http.MethodGet {
			return pageBridgeJSONResponse(request, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false}), true
		}
		return pageBridgeEventResponse(request, session, m.pages), true
	case "message":
		if request.Method != http.MethodPost {
			return pageBridgeJSONResponse(request, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false}), true
		}
		return m.handlePageBridgeMessage(request, session), true
	case "capture-start", "capture-write", "capture-complete", "capture-abort":
		if request.Method != http.MethodPost {
			return pageBridgeJSONResponse(request, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false}), true
		}
		return m.handlePageBridgeCapture(request, session, strings.TrimPrefix(parts[2], "capture-")), true
	case "close":
		if request.Method != http.MethodPost {
			return pageBridgeJSONResponse(request, http.StatusMethodNotAllowed, map[string]interface{}{"ok": false}), true
		}
		_, deferred := m.pages.closeSession(session)
		return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true, "data": map[string]interface{}{"deferred": deferred}}), true
	default:
		return pageBridgeJSONResponse(request, http.StatusNotFound, map[string]interface{}{"ok": false}), true
	}
}

func (m *PluginManager) handlePageBridgeCapture(request *http.Request, session *pageBridgeSession, action string) *http.Response {
	if m.captures == nil || !m.pageCaptureAllowed(session.pluginID) {
		return pageBridgeJSONResponse(request, http.StatusForbidden, map[string]interface{}{"ok": false, "error": "page capture is unavailable"})
	}
	key := strings.TrimSpace(request.Header.Get("X-Res-Downloader-Capture-Key"))
	if key == "" || len(key) > maxPluginCaptureKeySize || strings.IndexByte(key, 0) >= 0 {
		return pageBridgeJSONResponse(request, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "capture key is invalid"})
	}
	scopedKey := scopedCaptureKey(session.pluginID, key)
	switch action {
	case "start":
		if !m.pages.startCapture(session, key) {
			return pageBridgeJSONResponse(request, http.StatusTooManyRequests, map[string]interface{}{"ok": false, "error": "capture limit reached"})
		}
		if err := m.captures.StartStream(scopedKey); err != nil {
			m.pages.finishCapture(session, key)
			return pageBridgeJSONResponse(request, http.StatusConflict, map[string]interface{}{"ok": false, "error": "capture could not be started"})
		}
		return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true})
	case "write":
		if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/octet-stream") {
			return pageBridgeJSONResponse(request, http.StatusUnsupportedMediaType, map[string]interface{}{"ok": false, "error": "capture segment must be binary"})
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, maxPageCaptureChunkSize+1))
		if err != nil || len(body) == 0 || len(body) > maxPageCaptureChunkSize {
			return pageBridgeJSONResponse(request, http.StatusRequestEntityTooLarge, map[string]interface{}{"ok": false, "error": "capture segment is too large"})
		}
		reserved := int64(len(body))
		if !m.pages.reserveCaptureBytes(session, key, reserved) {
			return pageBridgeJSONResponse(request, http.StatusTooManyRequests, map[string]interface{}{"ok": false, "error": "capture limit reached"})
		}
		total, appendErr := m.captures.AppendStream(scopedKey, body)
		if appendErr != nil {
			m.pages.releaseCaptureBytes(session, key, reserved)
			return pageBridgeJSONResponse(request, http.StatusConflict, map[string]interface{}{"ok": false, "error": "capture segment was rejected"})
		}
		return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true, "data": map[string]interface{}{"bytes": total}})
	case "complete":
		if !m.pages.hasCapture(session, key) {
			return pageBridgeJSONResponse(request, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "capture was not started"})
		}
		if err := m.captures.CompleteStream(scopedKey); err != nil {
			return pageBridgeJSONResponse(request, http.StatusConflict, map[string]interface{}{"ok": false, "error": "capture is incomplete"})
		}
		m.pages.finishCapture(session, key)
		return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true})
	case "abort":
		if !m.pages.hasCapture(session, key) {
			return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true})
		}
		if err := m.captures.AbortStream(scopedKey); err != nil {
			return pageBridgeJSONResponse(request, http.StatusConflict, map[string]interface{}{"ok": false, "error": "capture could not be aborted"})
		}
		m.pages.finishCapture(session, key)
		return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true})
	default:
		return pageBridgeJSONResponse(request, http.StatusNotFound, map[string]interface{}{"ok": false, "error": "page capture action not found"})
	}
}

func (m *PluginManager) pageCaptureAllowed(pluginID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, item := range m.plugins {
		manifest := item.runtime.Manifest()
		if manifest.ID == pluginID && manifest.IsEnabled() && manifest.Permissions.Has("page-bridge") && manifest.Permissions.Has("capture-response-body") {
			return true
		}
	}
	return false
}

func pageBridgeRequestAllowed(session *pageBridgeSession, request *http.Request) bool {
	expectedHost := strings.TrimSpace(session.host)
	if expectedHost == "" && session.origin != "" {
		if parsed, err := url.Parse(session.origin); err == nil {
			expectedHost = parsed.Host
		}
	}
	if expectedHost == "" || !strings.EqualFold(expectedHost, request.Host) {
		return false
	}
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.EqualFold(parsed.Host, expectedHost)
}

func (m *PluginManager) handlePageBridgeMessage(request *http.Request, session *pageBridgeSession) *http.Response {
	if !m.pages.allowMessage(session) {
		return pageBridgeJSONResponse(request, http.StatusTooManyRequests, map[string]interface{}{"ok": false, "error": "page message rate limit exceeded"})
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxPageBridgeMessageSize+1))
	if err != nil || len(body) > maxPageBridgeMessageSize {
		return pageBridgeJSONResponse(request, http.StatusRequestEntityTooLarge, map[string]interface{}{"ok": false, "error": "page message is too large"})
	}
	var message interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&message); err != nil {
		return pageBridgeJSONResponse(request, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "page message must be valid JSON"})
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return pageBridgeJSONResponse(request, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "page message must contain one JSON value"})
	}
	result, err := m.processPageMessage(request.Context(), session, message)
	if err != nil {
		if m.logger != nil {
			m.logger.Esg(err, "plugin "+session.pluginID+" page message")
		}
		return pageBridgeJSONResponse(request, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": "plugin page message failed"})
	}
	return pageBridgeJSONResponse(request, http.StatusOK, result)
}

func pageBridgeJSONResponse(request *http.Request, status int, value interface{}) *http.Response {
	body, err := json.Marshal(value)
	if err != nil || len(body) > maxPageBridgeMessageSize {
		status = http.StatusInternalServerError
		body = []byte(`{"ok":false,"error":"plugin page reply is invalid or too large"}`)
	}
	return &http.Response{
		StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Request: request,
		Header: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}, "Cache-Control": []string{"no-store"}, "X-Content-Type-Options": []string{"nosniff"}},
		Body:   io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)),
	}
}

func pageBridgeEventResponse(request *http.Request, session *pageBridgeSession, hub *pageBridgeHub) *http.Response {
	session.eventsMu.Lock()
	if session.events >= maxPageEventConnections {
		session.eventsMu.Unlock()
		return pageBridgeJSONResponse(request, http.StatusTooManyRequests, map[string]interface{}{"ok": false, "error": "too many page event connections"})
	}
	session.events++
	session.eventsMu.Unlock()
	reader, writer := io.Pipe()
	go func() {
		defer func() {
			session.eventsMu.Lock()
			session.events--
			session.eventsMu.Unlock()
			_ = writer.Close()
		}()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		_, _ = io.WriteString(writer, ": connected\n\n")
		for {
			select {
			case message := <-session.messages:
				hub.touch(session)
				if _, err := fmt.Fprintf(writer, "event: message\ndata: %s\n\n", message); err != nil {
					return
				}
			case <-ticker.C:
				hub.touch(session)
				if _, err := io.WriteString(writer, ": keepalive\n\n"); err != nil {
					return
				}
			case <-session.done:
				return
			case <-request.Context().Done():
				return
			}
		}
	}()
	return &http.Response{
		StatusCode: http.StatusOK, Status: "200 OK", Request: request, Body: reader, ContentLength: -1,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream; charset=utf-8"}, "Cache-Control": []string{"no-store"},
			"Connection": []string{"keep-alive"}, "X-Accel-Buffering": []string{"no"},
		},
	}
}

func (m *PluginManager) processPageMessage(ctx context.Context, session *pageBridgeSession, message interface{}) (shared.PageMessageResult, error) {
	m.mu.RLock()
	plugins := append([]managedPlugin(nil), m.plugins...)
	m.mu.RUnlock()
	for _, item := range plugins {
		manifest := item.runtime.Manifest()
		if manifest.ID != session.pluginID || !manifest.Permissions.Has("page-bridge") {
			continue
		}
		handler, ok := pageMessageHandler(item.runtime)
		if !ok {
			return shared.PageMessageResult{OK: false, Error: "plugin does not handle page messages"}, nil
		}
		contextValue := shared.PageMessageContext{
			PageSessionID: session.id, ScriptID: session.scriptID, PageURL: session.pageURL, Origin: session.origin,
		}
		var result shared.PageMessageResult
		var handled bool
		err := m.runtimeState(manifest.ID).run(ctx, func(callCtx context.Context) error {
			var callErr error
			result, handled, callErr = handler.HandlePageMessage(callCtx, message, contextValue)
			return callErr
		})
		if err != nil {
			return result, err
		}
		if !handled {
			return shared.PageMessageResult{OK: false, Error: "plugin does not handle page messages"}, nil
		}
		published := m.publishPageMessageResources(manifest, result.Resources)
		result.Resources = nil
		if result.AutoDownload && result.OK {
			if !manifest.Permissions.Has("enqueue-download") {
				result.OK = false
				result.Error = "automatic download permission is unavailable"
			} else if err := m.enqueuePageDownloads(published); err != nil {
				if m.logger != nil {
					m.logger.Esg(err, "plugin "+session.pluginID+" automatic page download")
				}
				result.OK = false
				result.Error = "automatic download could not be started"
			}
		}
		return result, nil
	}
	return shared.PageMessageResult{OK: false, Error: "plugin is not active"}, nil
}

func pageMessageHandler(runtime shared.RuntimePlugin) (shared.PageMessageHandler, bool) {
	if handler, ok := runtime.(shared.PageMessageHandler); ok {
		return handler, true
	}
	if overridden, ok := runtime.(*manifestOverridePlugin); ok {
		handler, exists := overridden.RuntimePlugin.(shared.PageMessageHandler)
		return handler, exists
	}
	return nil, false
}

func (m *PluginManager) publishPageMessageResources(manifest shared.PluginManifest, resources []shared.ResourceCandidate) []shared.ResourceCandidate {
	if !manifest.Permissions.Has("emit-resource") || m.resources == nil {
		return nil
	}
	if len(resources) > maxPluginResources {
		resources = resources[:maxPluginResources]
	}
	validated := make([]shared.ResourceCandidate, 0, len(resources))
	for index := range resources {
		resources[index].Source.PluginID = manifest.ID
		resources[index].Source.PluginVersion = manifest.Version
		m.mu.RLock()
		resources[index].Source.PluginDigest = m.statuses[manifest.ID].Digest
		m.mu.RUnlock()
		resources[index].ParentID = ""
		if validateResourceActions(manifest, resources[index].Actions) != nil || validateCandidate(&resources[index]) != nil {
			continue
		}
		validated = append(validated, resources[index])
	}
	validated = m.resources.FilterSelectedCandidates(validated)
	m.resources.PublishCandidates(validated)
	return validated
}

func (m *PluginManager) enqueuePageDownloads(resources []shared.ResourceCandidate) error {
	if len(resources) == 0 {
		return errors.New("automatic download resource is missing")
	}
	if len(resources) > maxPageAutoDownloads {
		return errors.New("automatic download resource limit exceeded")
	}
	m.mu.RLock()
	handler := m.pageDownload
	m.mu.RUnlock()
	if handler == nil || m.resources == nil {
		return errors.New("automatic download service is unavailable")
	}
	for _, resource := range resources {
		if resource.GroupKey == "" {
			return errors.New("automatic download resource group is missing")
		}
		stored, exists := m.resources.CandidateByGroup(resource.Source.PluginID, resource.GroupKey)
		if !exists {
			return errors.New("automatic download resource was not published")
		}
		if err := handler(stored); err != nil {
			return err
		}
	}
	return nil
}
