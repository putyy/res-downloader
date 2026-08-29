package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin/native"
)

func (m *PluginManager) loadState() {
	raw, err := os.ReadFile(m.stateFile)
	if err == nil {
		_ = json.Unmarshal(raw, &m.overrides)
	}
}

func (m *PluginManager) loadSettings() {
	raw, err := os.ReadFile(m.settingsFile)
	if err == nil {
		_ = json.Unmarshal(raw, &m.settings)
	}
}

func (m *PluginManager) loadRemoved() {
	raw, err := os.ReadFile(m.removedFile)
	if err == nil {
		_ = json.Unmarshal(raw, &m.removed)
	}
}

func (m *PluginManager) loadSources() {
	raw, err := os.ReadFile(m.sourcesFile)
	if err == nil {
		_ = json.Unmarshal(raw, &m.sources)
	}
}

func (m *PluginManager) saveState() error {
	m.mu.RLock()
	raw, err := json.MarshalIndent(m.overrides, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return writePrivateFile(m.stateFile, raw)
}

func (m *PluginManager) saveSettings() error {
	m.mu.RLock()
	raw, err := json.MarshalIndent(m.settings, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return writePrivateFile(m.settingsFile, raw)
}

func (m *PluginManager) saveRemoved() error {
	m.mu.RLock()
	raw, err := json.MarshalIndent(m.removed, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return writePrivateFile(m.removedFile, raw)
}

func (m *PluginManager) saveSources() error {
	m.mu.RLock()
	raw, err := json.MarshalIndent(m.sources, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return writePrivateFile(m.sourcesFile, raw)
}

func writePrivateFile(fileName string, data []byte) error {
	if err := os.WriteFile(fileName, data, 0600); err != nil {
		return err
	}
	return os.Chmod(fileName, 0600)
}

func (m *PluginManager) pluginSettings(id string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return effectivePluginSettings(m.statuses[id].Manifest.SettingsSchema, m.settings[id])
}

func effectivePluginSettings(schema, configured map[string]interface{}) map[string]interface{} {
	settings := make(map[string]interface{})
	properties, _ := schema["properties"].(map[string]interface{})
	for key, rawProperty := range properties {
		property, _ := rawProperty.(map[string]interface{})
		if defaultValue, exists := property["default"]; exists {
			settings[key] = defaultValue
		}
	}
	for key, value := range configured {
		settings[key] = value
	}
	cloned, _ := shared.CloneJSON(settings)
	return cloned
}

func validatePluginSettings(schema, settings map[string]interface{}) error {
	if len(schema) == 0 {
		if len(settings) > 0 {
			return errors.New("plugin does not declare settingsSchema")
		}
		return nil
	}
	properties, _ := schema["properties"].(map[string]interface{})
	for key, value := range settings {
		rawProperty, exists := properties[key]
		if !exists {
			return fmt.Errorf("unknown plugin setting %q", key)
		}
		property, _ := rawProperty.(map[string]interface{})
		typeName, _ := property["type"].(string)
		valid := typeName == "" ||
			(typeName == "string" && isString(value)) ||
			(typeName == "number" && isNumber(value)) ||
			(typeName == "integer" && isInteger(value)) ||
			(typeName == "boolean" && isBoolean(value)) ||
			(typeName == "object" && isObject(value)) ||
			(typeName == "array" && isArray(value))
		if !valid {
			return fmt.Errorf("plugin setting %q must be %s", key, typeName)
		}
		if enum, ok := property["enum"].([]interface{}); ok && !containsJSON(enum, value) {
			return fmt.Errorf("plugin setting %q is not one of the allowed values", key)
		}
		if format, _ := property["format"].(string); format == "capture-rules" {
			if _, err := native.DecodeCaptureRules(value); err != nil {
				return fmt.Errorf("plugin setting %q: %w", key, err)
			}
		}
	}
	return nil
}

func isString(v interface{}) bool  { _, ok := v.(string); return ok }
func isNumber(v interface{}) bool  { _, ok := v.(float64); return ok }
func isInteger(v interface{}) bool { n, ok := v.(float64); return ok && n == float64(int64(n)) }
func isBoolean(v interface{}) bool { _, ok := v.(bool); return ok }
func isObject(v interface{}) bool {
	if v == nil {
		return false
	}
	return reflect.TypeOf(v).Kind() == reflect.Map
}
func isArray(v interface{}) bool {
	if v == nil {
		return false
	}
	kind := reflect.TypeOf(v).Kind()
	return kind == reflect.Slice || kind == reflect.Array
}

func containsJSON(values []interface{}, target interface{}) bool {
	want, _ := json.Marshal(target)
	for _, value := range values {
		got, _ := json.Marshal(value)
		if string(got) == string(want) {
			return true
		}
	}
	return false
}
