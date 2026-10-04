package operation

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"
)

const MaxSchemaDepth = 12

// ValidateSchema rejects unsupported keywords rather than pretending to enforce them.
func ValidateSchema(schema map[string]interface{}) error { return checkSchema(schema, 0) }
func checkSchema(s map[string]interface{}, depth int) error {
	if depth > MaxSchemaDepth || len(s) == 0 {
		return fmt.Errorf("invalid schema depth or empty schema")
	}
	typ, ok := s["type"].(string)
	if !ok || !oneOf(typ, "object", "array", "string", "number", "integer", "boolean", "null") {
		return fmt.Errorf("schema requires one supported type")
	}
	for k, v := range s {
		switch k {
		case "type":
		case "title", "description":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s must be a string", k)
			}
		case "x-persist", "x-sensitive":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%s must be boolean", k)
			}
		case "properties":
			props, ok := v.(map[string]interface{})
			if !ok || typ != "object" || len(props) > 64 {
				return fmt.Errorf("invalid properties")
			}
			for _, p := range props {
				child, ok := p.(map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid property schema")
				}
				if err := checkSchema(child, depth+1); err != nil {
					return err
				}
			}
		case "additionalProperties":
			if typ != "object" || v != false {
				return fmt.Errorf("additionalProperties must be false")
			}
		case "required":
			list, ok := v.([]interface{})
			if !ok || typ != "object" {
				return fmt.Errorf("invalid required")
			}
			props, _ := s["properties"].(map[string]interface{})
			seen := map[string]bool{}
			for _, n := range list {
				name, ok := n.(string)
				if !ok || props[name] == nil || seen[name] {
					return fmt.Errorf("invalid required property")
				}
				seen[name] = true
			}
		case "items":
			child, ok := v.(map[string]interface{})
			if !ok || typ != "array" {
				return fmt.Errorf("invalid items")
			}
			if err := checkSchema(child, depth+1); err != nil {
				return err
			}
		case "enum":
			list, ok := v.([]interface{})
			if !ok || len(list) == 0 || len(list) > 64 {
				return fmt.Errorf("invalid enum")
			}
			for _, item := range list {
				if typ == "object" || typ == "array" {
					return fmt.Errorf("enum supports scalar values only")
				}
				if err := validateValue(map[string]interface{}{"type": typ}, item, depth); err != nil {
					return err
				}
			}
		case "minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems":
			n, ok := number(v)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
				return fmt.Errorf("invalid numeric constraint")
			}
			if strings.Contains(k, "Length") || strings.Contains(k, "Items") {
				if n < 0 || n != math.Trunc(n) || (strings.Contains(k, "Length") && typ != "string") || (strings.Contains(k, "Items") && typ != "array") {
					return fmt.Errorf("invalid size constraint")
				}
			} else if typ != "number" && typ != "integer" {
				return fmt.Errorf("numeric bound on non-number")
			}
		default:
			return fmt.Errorf("unsupported schema keyword %q", k)
		}
	}
	if typ == "object" && s["additionalProperties"] != false {
		return fmt.Errorf("object schemas require additionalProperties: false")
	}
	if typ == "array" && s["items"] == nil {
		return fmt.Errorf("array schema requires items")
	}
	for _, pair := range [][2]string{{"minimum", "maximum"}, {"minLength", "maxLength"}, {"minItems", "maxItems"}} {
		lo, lok := number(s[pair[0]])
		hi, hok := number(s[pair[1]])
		if lok && hok && lo > hi {
			return fmt.Errorf("inverted schema bounds")
		}
	}
	return nil
}
func ValidateValue(s map[string]interface{}, v interface{}) error { return validateValue(s, v, 0) }
func validateValue(s map[string]interface{}, v interface{}, depth int) error {
	if depth > MaxSchemaDepth {
		return fmt.Errorf("value is too deeply nested")
	}
	typ, _ := s["type"].(string)
	switch typ {
	case "object":
		obj, ok := v.(map[string]interface{})
		if !ok || obj == nil {
			return fmt.Errorf("expected object")
		}
		props, _ := s["properties"].(map[string]interface{})
		required, _ := s["required"].([]interface{})
		for _, r := range required {
			if _, ok := obj[r.(string)]; !ok {
				return fmt.Errorf("required field %s", r)
			}
		}
		for k, value := range obj {
			child, ok := props[k].(map[string]interface{})
			if !ok {
				return fmt.Errorf("unknown field %q", k)
			}
			if err := validateValue(child, value, depth+1); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	case "array":
		list, ok := v.([]interface{})
		if !ok {
			return fmt.Errorf("expected array")
		}
		if !bounds(s, float64(len(list)), "minItems", "maxItems") {
			return fmt.Errorf("array length out of bounds")
		}
		child, _ := s["items"].(map[string]interface{})
		for _, value := range list {
			if err := validateValue(child, value, depth+1); err != nil {
				return err
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok || !utf8.ValidString(str) || !bounds(s, float64(utf8.RuneCountInString(str)), "minLength", "maxLength") {
			return fmt.Errorf("invalid string or length")
		}
	case "number", "integer":
		n, ok := number(v)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || (typ == "integer" && math.Trunc(n) != n) || !bounds(s, n, "minimum", "maximum") {
			return fmt.Errorf("invalid numeric value")
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected boolean")
		}
	case "null":
		if v != nil {
			return fmt.Errorf("expected null")
		}
	default:
		return fmt.Errorf("unsupported schema type")
	}
	if enum, ok := s["enum"].([]interface{}); ok {
		found := false
		for _, e := range enum {
			if reflect.DeepEqual(e, v) {
				found = true
			}
			a, aok := number(e)
			b, bok := number(v)
			if aok && bok && a == b {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("value not in enum")
		}
	}
	return nil
}
func number(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	}
	return 0, false
}
func bounds(s map[string]interface{}, n float64, lo, hi string) bool {
	if x, ok := number(s[lo]); ok && n < x {
		return false
	}
	if x, ok := number(s[hi]); ok && n > x {
		return false
	}
	return true
}
func oneOf(v string, choices ...string) bool {
	for _, c := range choices {
		if c == v {
			return true
		}
	}
	return false
}

// Persisted projects only individually opted-in scalar leaves. Opting in an
// object never grants permission to persist arbitrary nested JSON.
func Persisted(s map[string]interface{}, v interface{}) interface{} {
	if s["x-sensitive"] == true {
		return nil
	}
	switch s["type"] {
	case "object":
		obj, _ := v.(map[string]interface{})
		props, _ := s["properties"].(map[string]interface{})
		out := map[string]interface{}{}
		for key, value := range obj {
			lower := strings.ToLower(strings.ReplaceAll(key, "-", ""))
			if strings.Contains(lower, "cookie") || strings.Contains(lower, "authorization") || strings.Contains(lower, "token") || strings.Contains(lower, "password") {
				continue
			}
			child, _ := props[key].(map[string]interface{})
			if p := Persisted(child, value); p != nil {
				out[key] = p
			}
		}
		if len(out) > 0 {
			return out
		}
	case "array":
		list, _ := v.([]interface{})
		child, _ := s["items"].(map[string]interface{})
		out := []interface{}{}
		for _, value := range list {
			if p := Persisted(child, value); p != nil {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	default:
		if s["x-persist"] == true {
			return v
		}
	}
	return nil
}
