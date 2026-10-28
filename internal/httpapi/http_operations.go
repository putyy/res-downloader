package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	m "res-downloader/internal/model"
	"res-downloader/internal/operation"
	"strings"
)

type operationSourceKey struct{}

func operationSource(r *http.Request) string {
	if value, _ := r.Context().Value(operationSourceKey{}).(string); value == "automation" {
		return value
	}
	return "desktop"
}
func operationPath(path string) bool {
	switch strings.TrimPrefix(path, "/api/operations/") {
	case "list", "get", "sessions", "invoke", "batch", "execution", "batch-get", "history", "resources", "cancel", "batch-cancel", "artifact", "text", "status", "settings", "clean":
		return strings.HasPrefix(path, "/api/operations/")
	}
	return false
}
func (h *Server) operationError(w http.ResponseWriter, err error) {
	code := "operation_failed"
	var known *operation.Error
	if errors.As(err, &known) {
		code = known.Code
	}
	h.error(w, err.Error(), respData{"errorCode": code})
}
func (h *Server) operationsAPI(w http.ResponseWriter, r *http.Request) {
	service := h.plugins.OperationService()
	if service == nil {
		h.operationError(w, &operation.Error{Code: "storage_unavailable", Message: "operation storage is unavailable"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024+1))
	if err != nil || len(raw) > 2*1024*1024 {
		h.operationError(w, &operation.Error{Code: "input_too_large", Message: "operation request exceeds 2 MiB"})
		return
	}
	decode := func(target interface{}) error {
		if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
			return &operation.Error{Code: "invalid_input", Message: "operation request must be a JSON object"}
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if e := d.Decode(target); e != nil {
			return &operation.Error{Code: "invalid_input", Message: e.Error()}
		}
		if d.Decode(new(interface{})) != io.EOF {
			return &operation.Error{Code: "invalid_input", Message: "unexpected trailing input"}
		}
		return nil
	}
	source := operationSource(r)
	var result interface{}
	switch strings.TrimPrefix(r.URL.Path, "/api/operations/") {
	case "resources":
		var req struct {
			ResourceIDs []string `json:"resourceIds"`
		}
		if err = decode(&req); err == nil {
			result, err = service.ResourceExecutions(req.ResourceIDs)
		}
	case "invoke":
		var req m.OperationRequest
		if err = decode(&req); err == nil {
			result, err = service.Submit(req, source)
		}
	case "batch":
		var req struct {
			Items          []m.OperationRequest `json:"items"`
			IdempotencyKey string               `json:"idempotencyKey,omitempty"`
		}
		if err = decode(&req); err == nil {
			result, err = service.SubmitBatch(req.Items, req.IdempotencyKey, source)
		}
	case "settings":
		var settings operation.Settings
		if err = decode(&settings); err == nil {
			err = service.SetSettings(settings)
			result = service.Status()
		}
	case "clean":
		var req struct {
			History bool `json:"history"`
			Results bool `json:"results"`
		}
		if err = decode(&req); err == nil {
			err = service.Clean(req.History, req.Results)
		}
	default:
		// Each query has its own contract: never silently ignore a filter or
		// accept a field belonging to another operation endpoint.
		path := strings.TrimPrefix(r.URL.Path, "/api/operations/")
		allowed := map[string]string{
			"status": "", "sessions": "", "list": "pluginId",
			"get": "pluginId operationId", "execution": "id",
			"batch-get": "id offset limit", "history": "pluginId state offset limit",
			"cancel": "id", "batch-cancel": "id", "artifact": "id", "text": "id offset limit",
		}
		var fields map[string]json.RawMessage
		if err = decode(&fields); err != nil {
			break
		}
		for name := range fields {
			if !strings.Contains(" "+allowed[path]+" ", " "+name+" ") || bytes.Equal(bytes.TrimSpace(fields[name]), []byte("null")) {
				err = &operation.Error{Code: "invalid_input", Message: "invalid query field: " + name}
				break
			}
		}
		if err != nil {
			break
		}
		var req struct {
			ID          string `json:"id,omitempty"`
			PluginID    string `json:"pluginId,omitempty"`
			OperationID string `json:"operationId,omitempty"`
			State       string `json:"state,omitempty"`
			Offset      int    `json:"offset,omitempty"`
			Limit       int    `json:"limit,omitempty"`
		}
		if err = decode(&req); err != nil {
			break
		}
		if req.Offset < 0 || req.Limit < 0 {
			err = &operation.Error{Code: "invalid_input", Message: "offset and limit must be nonnegative"}
			break
		}
		if req.Limit == 0 {
			req.Limit = 50
		}
		switch strings.TrimPrefix(r.URL.Path, "/api/operations/") {
		case "status":
			result = service.Status()
		case "list":
			out := []m.OperationInfo{}
			for _, info := range service.Discover(source) {
				if req.PluginID == "" || req.PluginID == info.PluginID {
					out = append(out, info)
				}
			}
			result = out
		case "get":
			for _, info := range service.Discover(source) {
				if info.PluginID == req.PluginID && info.OperationID == req.OperationID {
					result = info
				}
			}
			if result == nil {
				err = &operation.Error{Code: "not_found", Message: "operation not found"}
			}
		case "sessions":
			result = service.Sessions(source)
		case "execution":
			result, err = service.Get(req.ID)
		case "batch-get":
			result, err = service.Batch(req.ID, req.Offset, req.Limit)
		case "history":
			result, err = service.List(req.PluginID, req.State, req.Offset, req.Limit)
		case "cancel":
			err = service.Cancel(req.ID)
		case "batch-cancel":
			err = service.CancelBatch(req.ID)
		case "artifact":
			result, err = service.Artifact(req.ID)
		case "text":
			result, err = service.ReadText(req.ID, int64(req.Offset), int64(req.Limit))
		}
	}
	if err != nil {
		h.operationError(w, err)
		return
	}
	h.success(w, result)
}
func automationRequest(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), operationSourceKey{}, "automation"))
}
