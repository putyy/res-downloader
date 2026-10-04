package operation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStrictSchemaRejectsUnsupportedContracts(t *testing.T) {
	for _, raw := range []string{`{"type":"string","pattern":".*"}`, `{"type":"object"}`, `{"type":"array"}`, `{"type":"string","minimum":1}`, `{"type":"integer","minimum":5,"maximum":1}`, `{"type":"object","additionalProperties":false,"required":["missing"]}`, `{"type":"string","enum":[5]}`} {
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &schema); err != nil {
			t.Fatal(err)
		}
		if ValidateSchema(schema) == nil {
			t.Fatalf("unsupported schema accepted %s", raw)
		}
	}
}
func TestPersistedProjectionCannotOptInArbitraryNestedValues(t *testing.T) {
	var schema map[string]interface{}
	_ = json.Unmarshal([]byte(`{"type":"object","additionalProperties":false,"x-persist":true,"properties":{"title":{"type":"string","x-persist":true},"auth":{"type":"string","x-persist":true,"x-sensitive":true},"Cookie":{"type":"string","x-persist":true},"nested":{"type":"object","additionalProperties":false,"x-persist":true,"properties":{"private":{"type":"string"}}}}}`), &schema)
	if err := ValidateSchema(schema); err != nil {
		t.Fatal(err)
	}
	projected := Persisted(schema, map[string]interface{}{"title": "public", "auth": "secret-one", "Cookie": "secret-two", "nested": map[string]interface{}{"private": "secret-three"}})
	raw, _ := json.Marshal(projected)
	if string(raw) != `{"title":"public"}` || strings.Contains(string(raw), "secret") {
		t.Fatalf("unsafe projection %s", raw)
	}
}
func TestUTF8LengthAndUnknownInputValidation(t *testing.T) {
	schema := map[string]interface{}{"type": "string", "maxLength": float64(2)}
	if err := ValidateValue(schema, "中文"); err != nil {
		t.Fatal(err)
	}
	if ValidateValue(schema, "中文三") == nil {
		t.Fatal("maximum length not enforced")
	}
	object := map[string]interface{}{"type": "object", "additionalProperties": false}
	if ValidateValue(object, map[string]interface{}{"extra": true}) == nil {
		t.Fatal("extra input accepted")
	}
}
