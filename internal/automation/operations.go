// Package automation implements CLI and MCP adapters for the running desktop app.
package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type operation struct {
	name, group, command, path, description string
	argument                                string
	readOnly                                bool
}

var operations = []operation{
	{"list_resources", "resources", "list", "/api/resources", "List the captured resource catalog with pagination, including collection children. Open/play content through the capture proxy first; this does not browse or resolve webpage URLs.", "page", true},
	{"create_download", "downloads", "create", "/api/download/create", "Queue a captured resource for download using desktop download settings. Returns a task immediately; use list_downloads or get_download to check progress. An existing active task for the resource is reused.", "resourceId", false},
	{"list_downloads", "downloads", "list", "/api/download/tasks", "List download tasks and progress, including completed and failed tasks.", "", true},
	{"get_download", "downloads", "get", "/api/download/tasks", "Get one download task by task ID, including state, progress and output information.", "id", true},
	{"pause_download", "downloads", "pause", "/api/download/pause", "Pause a task when its executor supports pausing.", "id", false},
	{"resume_download", "downloads", "resume", "/api/download/resume", "Resume a paused or interrupted task when supported.", "id", false},
	{"cancel_download", "downloads", "cancel", "/api/download/cancel", "Cancel a download task. This does not delete completed output files.", "id", false},
	{"retry_download", "downloads", "retry", "/api/download/retry", "Retry a failed or cancelled download task.", "id", false},
}

type commandError struct {
	kind    string
	message string
}

func (e *commandError) Error() string { return e.message }

func fail(kind, message string) error { return &commandError{kind: kind, message: message} }

func (op operation) schema() map[string]any {
	properties := map[string]any{}
	required := []string{}
	if op.argument == "page" {
		properties["offset"] = map[string]any{"type": "integer", "minimum": 0, "default": 0}
		properties["limit"] = map[string]any{"type": "integer", "minimum": 1, "maximum": 5000, "default": 100}
	} else if op.argument != "" {
		properties[op.argument] = map[string]any{"type": "string", "minLength": 1, "maxLength": 512}
		required = append(required, op.argument)
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

// Validate in one place so CLI and MCP accept exactly the same arguments.
func (op operation) arguments(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var values map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, fail("invalid_arguments", "arguments must be a JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fail("invalid_arguments", "unexpected data after arguments")
	}
	for name := range values {
		if (op.argument == "page" && (name == "offset" || name == "limit")) || (op.argument != "" && op.argument != "page" && name == op.argument) {
			continue
		}
		return nil, fail("invalid_arguments", fmt.Sprintf("unknown argument %q", name))
	}
	result := map[string]any{}
	if op.argument == "page" {
		for _, name := range []string{"offset", "limit"} {
			value := 0
			if name == "limit" {
				value = 100
			}
			if raw, exists := values[name]; exists {
				if string(raw) == "null" || json.Unmarshal(raw, &value) != nil {
					return nil, fail("invalid_arguments", name+" must be an integer")
				}
			}
			if value < 0 || (name == "limit" && (value < 1 || value > 5000)) {
				return nil, fail("invalid_arguments", "offset must be >= 0 and limit must be between 1 and 5000")
			}
			result[name] = value
		}
	} else if op.argument != "" {
		var id string
		if json.Unmarshal(values[op.argument], &id) != nil || strings.TrimSpace(id) == "" || len(id) > 512 {
			return nil, fail("invalid_arguments", op.argument+" must be a non-empty ID of at most 512 bytes")
		}
		result["id"] = id
	}
	return result, nil
}
