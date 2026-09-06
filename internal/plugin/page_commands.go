package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	shared "res-downloader/internal/model"
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
		return shared.PageCommandDispatch{}, errors.New("page command service is unavailable")
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
		return shared.PageCommandDispatch{}, fmt.Errorf("page command exceeds %d bytes", maxPageBridgeMessageSize)
	}

	matched, delivered := m.pages.queuePageCommand(resource.Source.PluginID, definition.PageScript, raw)
	if matched == 0 {
		return shared.PageCommandDispatch{}, fmt.Errorf("no active page is available for action %q", actionID)
	}
	if delivered == 0 {
		return shared.PageCommandDispatch{}, fmt.Errorf("page command queues are unavailable for action %q", actionID)
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
func (h *pageBridgeHub) queuePageCommand(pluginID, scriptID string, raw []byte) (matched, delivered int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneLocked(time.Now())
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
			default:
			}
		}
		session.eventsMu.Unlock()
	}
	return matched, delivered
}
