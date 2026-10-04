package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// These are transport contracts only. Plugin-specific schemas and permissions
// are always revalidated by the operation service at submission time.
type argumentField struct {
	kind         string
	required     bool
	min, max     int
	defaultValue any
	enum         []string
}

func operationRequestFields() map[string]argumentField {
	return map[string]argumentField{
		"pluginId":       {kind: "string", required: true, min: 1, max: 512},
		"operationId":    {kind: "string", required: true, min: 1, max: 512},
		"input":          {kind: "object", required: true},
		"pageSessionId":  {kind: "string", min: 1, max: 512},
		"cursor":         {kind: "string", min: 1, max: 128},
		"limit":          {kind: "integer", min: 1, max: 100},
		"idempotencyKey": {kind: "string", min: 1, max: 128},
		"retryOf":        {kind: "string", min: 1, max: 64},
		"resourceId":     {kind: "string", min: 1, max: 512},
		"actionId":       {kind: "string", min: 1, max: 512},
	}
}

func (op operation) fields() map[string]argumentField {
	fields := map[string]argumentField{}
	id := argumentField{kind: "string", required: true, min: 1, max: 512}
	switch op.name {
	case "list_plugin_operations":
		fields["pluginId"] = argumentField{kind: "string", min: 1, max: 512}
	case "get_plugin_operation":
		fields["pluginId"], fields["operationId"] = id, id
	case "invoke_plugin_operation":
		return operationRequestFields()
	case "invoke_plugin_batch":
		fields["items"] = argumentField{kind: "array", required: true, min: 1, max: 32}
		fields["idempotencyKey"] = argumentField{kind: "string", min: 1, max: 96}
	case "get_plugin_execution", "cancel_plugin_execution", "cancel_plugin_batch", "get_artifact":
		fields["id"] = id
	case "get_plugin_batch", "list_plugin_executions", "read_text_artifact":
		fields["offset"] = argumentField{kind: "integer", min: 0, max: 10000, defaultValue: 0}
		fields["limit"] = argumentField{kind: "integer", min: 1, max: 100, defaultValue: 50}
		if op.name == "list_plugin_executions" {
			fields["pluginId"] = argumentField{kind: "string", min: 1, max: 512}
			fields["state"] = argumentField{kind: "string", min: 1, max: 32, enum: []string{"queued", "running", "succeeded", "failed", "cancelled", "timed_out", "interrupted"}}
		} else {
			fields["id"] = id
		}
		if op.name == "get_plugin_batch" {
			fields["offset"] = argumentField{kind: "integer", min: 0, max: 32, defaultValue: 0}
			fields["limit"] = argumentField{kind: "integer", min: 1, max: 32, defaultValue: 32}
		}
		if op.name == "read_text_artifact" {
			fields["offset"] = argumentField{kind: "integer", min: 0, max: 1024*1024 - 1, defaultValue: 0}
			fields["limit"] = argumentField{kind: "integer", min: 4, max: 32768, defaultValue: 32768}
		}
	}
	return fields
}

func fieldsSchema(fields map[string]argumentField) map[string]any {
	properties := map[string]any{}
	required := []string{}
	for name, field := range fields {
		property := map[string]any{"type": field.kind}
		switch field.kind {
		case "string":
			property["minLength"], property["maxLength"] = field.min, field.max
			property["description"] = "Non-blank value; maximum length also applies in UTF-8 bytes."
		case "integer":
			property["minimum"], property["maximum"] = field.min, field.max
		case "object":
			property["additionalProperties"] = true
			property["description"] = "Plugin operation input; validated against its current inputSchema by the host. Maximum serialized input is 32 KiB."
		case "array":
			property["minItems"], property["maxItems"] = field.min, field.max
			property["items"] = fieldsSchema(operationRequestFields())
		}
		if field.defaultValue != nil {
			property["default"] = field.defaultValue
		}
		if field.enum != nil {
			property["enum"] = field.enum
		}
		properties[name] = property
		if field.required {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func validateFields(raw json.RawMessage, fields map[string]argumentField) (map[string]any, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if len(raw) > 2*1024*1024 {
		return nil, fail("invalid_arguments", "arguments exceed 2 MiB")
	}
	var values map[string]json.RawMessage
	d := json.NewDecoder(bytes.NewReader(raw))
	if d.Decode(&values) != nil || values == nil {
		return nil, fail("invalid_arguments", "arguments must be a JSON object")
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fail("invalid_arguments", "unexpected data after arguments")
	}
	for name := range values {
		if _, exists := fields[name]; !exists {
			return nil, fail("invalid_arguments", fmt.Sprintf("unknown argument %q", name))
		}
	}
	result := map[string]any{}
	for name, field := range fields {
		raw, exists := values[name]
		if !exists {
			if field.required {
				return nil, fail("invalid_arguments", name+" is required")
			}
			if field.defaultValue != nil {
				result[name] = field.defaultValue
			}
			continue
		}
		invalid := func() (map[string]any, error) {
			return nil, fail("invalid_arguments", name+" has an invalid type, value or size")
		}
		if string(raw) == "null" {
			return invalid()
		}
		switch field.kind {
		case "string":
			var value string
			if json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value) == "" || len(value) < field.min || len(value) > field.max {
				return invalid()
			}
			if field.enum != nil {
				found := false
				for _, allowed := range field.enum {
					if value == allowed {
						found = true
					}
				}
				if !found {
					return invalid()
				}
			}
			result[name] = value
		case "integer":
			var value int
			if json.Unmarshal(raw, &value) != nil || value < field.min || value > field.max {
				return invalid()
			}
			result[name] = value
		case "object":
			var value map[string]json.RawMessage
			if len(raw) > 32*1024 || json.Unmarshal(raw, &value) != nil || value == nil {
				return invalid()
			}
			// Preserve nested JSON numbers exactly until the host schema validates them.
			result[name] = value
		case "array":
			var items []json.RawMessage
			if json.Unmarshal(raw, &items) != nil || len(items) < field.min || len(items) > field.max {
				return invalid()
			}
			validated := make([]map[string]any, len(items))
			for i, item := range items {
				value, err := validateFields(item, operationRequestFields())
				if err != nil {
					return nil, fail("invalid_arguments", fmt.Sprintf("items[%d]: %s", i, err))
				}
				validated[i] = value
			}
			result[name] = validated
		}
	}
	return result, nil
}
