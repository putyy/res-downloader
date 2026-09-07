package plugin

import (
	"encoding/json"
	"fmt"
	"time"

	shared "res-downloader/internal/model"
)

// PageCommandError carries a stable desktop translation key; Error remains useful in logs.
type PageCommandError struct {
	Code    string
	Message string
}

func (e *PageCommandError) Error() string { return e.Message }

const (
	queueCommandDuplicate = -1
	queueCommandCapacity  = -2
)

func (m *PluginManager) DispatchPageCommand(resource shared.ResourceCandidate, actionID string) (shared.PageCommandDispatch, error) {
	definition, action, err := m.ResolveResourceAction(resource, actionID)
	if err != nil {
		return shared.PageCommandDispatch{}, err
	}
	if definition.Kind != shared.PluginActionPageCommand {
		return shared.PageCommandDispatch{}, fmt.Errorf("resource action %q is not a page command", actionID)
	}
	if m.pages == nil {
		return shared.PageCommandDispatch{}, &PageCommandError{Code: "page_command_unavailable", Message: "page command service is unavailable"}
	}

	requestID, err := randomPageBridgeValue(16)
	if err != nil {
		return shared.PageCommandDispatch{}, fmt.Errorf("create page command request ID: %w", err)
	}
	message := shared.PageCommandMessage{
		Protocol:  shared.PageCommandProtocol,
		Type:      shared.PageCommandType,
		RequestID: requestID,
		ActionID:  actionID,
		Resource: shared.PageCommandResource{
			ID:       resource.ID,
			GroupKey: resource.GroupKey,
		},
		Data: action.Data,
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return shared.PageCommandDispatch{}, fmt.Errorf("encode page command: %w", err)
	}
	if len(raw) > maxPageBridgeMessageSize {
		return shared.PageCommandDispatch{}, &PageCommandError{Code: "page_command_too_large", Message: fmt.Sprintf("page command exceeds %d bytes", maxPageBridgeMessageSize)}
	}

	var tracked *trackedPageCommand
	if definition.TrackProgress {
		token, err := randomPageBridgeValue(24)
		if err != nil {
			return shared.PageCommandDispatch{}, err
		}
		now := time.Now().UnixMilli()
		tracked = &trackedPageCommand{
			view: shared.PageCommandStatus{RequestID: requestID, PluginID: resource.Source.PluginID,
				ActionID: actionID, ResourceID: resource.ID, State: "pending", CreatedAt: now, UpdatedAt: now},
			scriptID: definition.PageScript, resumeToken: token, recipients: make(map[string]bool),
		}
	}
	matched, delivered := m.pages.queuePageCommand(resource.Source.PluginID, definition.PageScript, raw, tracked)
	if matched == queueCommandDuplicate {
		return shared.PageCommandDispatch{}, &PageCommandError{Code: "page_command_already_active", Message: "a page command is already active"}
	}
	if matched == queueCommandCapacity {
		return shared.PageCommandDispatch{}, &PageCommandError{Code: "page_command_limit_reached", Message: "the page command limit was reached"}
	}
	if matched == 0 {
		return shared.PageCommandDispatch{}, &PageCommandError{Code: "page_command_no_page", Message: fmt.Sprintf("no active page is available for action %q", actionID)}
	}
	if delivered == 0 {
		return shared.PageCommandDispatch{}, &PageCommandError{Code: "page_command_queue_full", Message: fmt.Sprintf("page command queues are unavailable for action %q", actionID)}
	}
	return shared.PageCommandDispatch{
		RequestID:    requestID,
		PageScriptID: definition.PageScript,
		Delivered:    delivered,
	}, nil
}

// queuePageCommand only queues user actions for currently connected pages.
// Keep session removal and connection teardown locked through the enqueue so
// a known disconnected session cannot be counted as a recipient. Enqueueing
// still does not acknowledge page execution or guarantee future connectivity.
func (h *pageBridgeHub) queuePageCommand(pluginID, scriptID string, raw []byte, tracking ...*trackedPageCommand) (matched, delivered int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked(time.Now())
	h.pruneCommandsLocked(time.Now())
	var command *trackedPageCommand
	if len(tracking) > 0 {
		command = tracking[0]
	}
	if command != nil {
		for _, existing := range h.commands {
			if !commandTerminal(existing.view.State) && existing.view.PluginID == pluginID &&
				existing.view.ResourceID == command.view.ResourceID && existing.view.ActionID == command.view.ActionID {
				return queueCommandDuplicate, 0
			}
		}
		if len(h.commands) >= maxTrackedPageCommands {
			return queueCommandCapacity, 0
		}
	}
	for _, session := range h.sessions {
		if session.pluginID != pluginID || session.scriptID != scriptID {
			continue
		}
		session.eventsMu.Lock()
		if session.events > 0 {
			matched++
			select {
			case session.messages <- raw:
				delivered++
				if command != nil {
					command.recipients[session.id] = true
				}
			default:
			}
		}
		session.eventsMu.Unlock()
	}
	if command != nil && delivered > 0 {
		if h.commands == nil {
			h.commands = make(map[string]*trackedPageCommand)
		}
		h.commands[command.view.RequestID] = command
	}
	return matched, delivered
}
