package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"sort"
	"time"

	shared "res-downloader/internal/model"
)

const maxTrackedPageCommands = 256

type trackedPageCommand struct {
	view        shared.PageCommandStatus
	scriptID    string
	owner       string
	resumeToken string
	recipients  map[string]bool // true until the recipient rejects the target
}

func commandTerminal(state string) bool {
	return state == "completed" || state == "failed" || state == "cancelled"
}

func (c *trackedPageCommand) finish(state, message string, now time.Time) {
	c.view.State, c.view.Message, c.view.UpdatedAt = state, message, now.UnixMilli()
	c.view.ErrorCode = ""
}

func (c *trackedPageCommand) fail(code, message string, now time.Time) {
	c.finish("failed", message, now)
	c.view.ErrorCode = code
}

func (h *pageBridgeHub) pruneCommandsLocked(now time.Time) {
	for id, c := range h.commands {
		age := now.Sub(time.UnixMilli(c.view.UpdatedAt))
		if commandTerminal(c.view.State) {
			if age > 10*time.Minute {
				delete(h.commands, id)
			}
			continue
		}
		if c.view.State == "pending" && age > 30*time.Second {
			c.fail("page_command_not_accepted", "No matching page accepted the command; open the target page and retry", now)
		} else if age > 90*time.Second || now.Sub(time.UnixMilli(c.view.CreatedAt)) > 6*time.Hour {
			c.fail("page_command_timeout", "Page command timed out or the page disconnected", now)
		}
	}
}

func (m *PluginManager) PageCommandStatuses() []shared.PageCommandStatus {
	out := make([]shared.PageCommandStatus, 0)
	if m.pages == nil {
		return out
	}
	h := m.pages
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneCommandsLocked(time.Now())
	for _, c := range h.commands {
		view := c.view
		if view.Progress != nil {
			p := *view.Progress
			view.Progress = &p
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (h *pageBridgeHub) claimCommand(session *pageBridgeSession, requestID, resumeToken string) (bool, string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneCommandsLocked(time.Now())
	c := h.commands[requestID]
	if c == nil || h.sessions[session.id] != session || c.view.PluginID != session.pluginID || c.scriptID != session.scriptID || commandTerminal(c.view.State) {
		return false, "", errors.New("page command is unavailable")
	}
	if resumeToken != "" {
		if resumeToken != c.resumeToken || c.owner == "" {
			return false, "", errors.New("invalid command resume token")
		}
		// A same-tab reload may resume once using the secret returned only to its owner.
		// Rotate it so concurrent/late resume requests cannot take ownership back.
		next, err := randomPageBridgeValue(24)
		if err != nil {
			return false, "", err
		}
		c.resumeToken = next
	} else {
		if !c.recipients[session.id] {
			return false, "", errors.New("page was not a command recipient")
		}
		if c.owner != "" {
			return false, "", nil
		}
	}
	c.owner = session.id
	c.finish("running", "", time.Now())
	return true, c.resumeToken, nil
}

func (h *pageBridgeHub) reportCommand(session *pageBridgeSession, report shared.PageCommandReport) error {
	if len(report.Message) > 1024 || (report.Progress != nil && (math.IsNaN(*report.Progress) || math.IsInf(*report.Progress, 0) || *report.Progress < 0 || *report.Progress > 100)) {
		return errors.New("invalid page command progress or message")
	}
	if report.State != "running" && report.State != "rejected" && !commandTerminal(report.State) {
		return errors.New("invalid page command state")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneCommandsLocked(time.Now())
	c := h.commands[report.RequestID]
	if c == nil || h.sessions[session.id] != session || c.view.PluginID != session.pluginID || c.scriptID != session.scriptID || commandTerminal(c.view.State) {
		return errors.New("page command is unavailable")
	}
	if report.State == "rejected" {
		if _, exists := c.recipients[session.id]; !exists {
			return errors.New("page was not a command recipient")
		}
		if c.owner != "" {
			return nil
		} // Non-target tabs cannot override the executing page.
		c.recipients[session.id] = false
		for _, pending := range c.recipients {
			if pending {
				return nil
			}
		}
		c.fail("page_command_target_unavailable", "No matching page is ready; open the target page and retry", time.Now())
		return nil
	}
	if c.owner != session.id {
		return errors.New("page does not own this command")
	}
	c.finish(report.State, report.Message, time.Now())
	c.view.Progress = nil
	if report.Progress != nil {
		p := *report.Progress
		c.view.Progress = &p
	}
	return nil
}

func (m *PluginManager) handlePageCommandRequest(request *http.Request, session *pageBridgeSession, action string) *http.Response {
	fail := func(status int, message string) *http.Response {
		return pageBridgeJSONResponse(request, status, map[string]interface{}{"ok": false, "error": message})
	}
	if !m.pages.allowMessage(session) {
		return fail(http.StatusTooManyRequests, "page message rate limit exceeded")
	}
	// Use the same authenticated bridge and strict plugin/script membership as dispatch.
	if !m.pageCommandAllowed(session) {
		return fail(http.StatusForbidden, "page command permission is unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 4097))
	if err != nil || len(body) > 4096 {
		return fail(http.StatusRequestEntityTooLarge, "page command report exceeds 4096 bytes")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var input struct {
		RequestID   string   `json:"requestId"`
		ResumeToken string   `json:"resumeToken,omitempty"`
		State       string   `json:"state,omitempty"`
		Progress    *float64 `json:"progress,omitempty"`
		Message     string   `json:"message,omitempty"`
	}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF || len(input.RequestID) != 32 || len(input.ResumeToken) > 48 {
		return fail(http.StatusBadRequest, "invalid page command request")
	}
	if action == "command-claim" {
		accepted, token, err := m.pages.claimCommand(session, input.RequestID, input.ResumeToken)
		if err != nil {
			return fail(http.StatusConflict, err.Error())
		}
		return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true, "accepted": accepted, "resumeToken": token})
	}
	if err := m.pages.reportCommand(session, shared.PageCommandReport{RequestID: input.RequestID, State: input.State, Progress: input.Progress, Message: input.Message}); err != nil {
		return fail(http.StatusConflict, err.Error())
	}
	return pageBridgeJSONResponse(request, http.StatusOK, map[string]interface{}{"ok": true})
}

func (m *PluginManager) pageCommandAllowed(session *pageBridgeSession) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, exists := m.statuses[session.pluginID]
	if !exists || !status.Loaded || !status.Manifest.IsEnabled() || !status.Manifest.Permissions.Has("page-bridge") {
		return false
	}
	for _, action := range status.Manifest.Actions {
		if action.Kind == shared.PluginActionPageCommand && action.TrackProgress && action.PageScript == session.scriptID {
			return true
		}
	}
	return false
}
