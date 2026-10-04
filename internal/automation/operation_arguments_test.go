package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func namedOperation(t *testing.T, name string) operation {
	t.Helper()
	for _, op := range operations {
		if op.name == name {
			return op
		}
	}
	t.Fatalf("missing operation %s", name)
	return operation{}
}

func TestOperationArgumentsRejectInvalidRequestsBeforeDiscovery(t *testing.T) {
	cases := []struct{ tool, raw string }{
		{"list_page_sessions", `{"pluginId":"not-a-supported-filter"}`},
		{"get_plugin_operation", `{"pluginId":"p"}`},
		{"invoke_plugin_operation", `{"pluginId":"p","operationId":"op","input":null}`},
		{"invoke_plugin_operation", `{"pluginId":"p","operationId":"op","input":{},"source":"desktop"}`},
		{"invoke_plugin_operation", `{"pluginId":"p","operationId":"op","input":{},"limit":101}`},
		{"invoke_plugin_operation", `{"pluginId":"p","operationId":"op","input":{},"idempotencyKey":" "}`},
		{"invoke_plugin_batch", `{"items":[]}`},
		{"invoke_plugin_batch", `{"items":[{"pluginId":"p","operationId":"op","input":{},"path":"/tmp/private"}]}`},
		{"get_plugin_batch", `{"id":"b","limit":33}`},
		{"list_plugin_executions", `{"state":"complete"}`},
		{"read_text_artifact", `{"id":"a","offset":1048576}`},
		{"read_text_artifact", `{"id":"a","limit":3}`},
		{"get_artifact", `{"path":"/tmp/private"}`},
		{"get_artifact", `{"id":"a"} {}`},
	}
	c := newClient(filepath.Join(t.TempDir(), "missing"), time.Second)
	for _, tc := range cases {
		t.Run(tc.tool+tc.raw, func(t *testing.T) {
			_, err := c.call(context.Background(), namedOperation(t, tc.tool), json.RawMessage(tc.raw))
			var ce *commandError
			if !errors.As(err, &ce) || ce.kind != "invalid_arguments" {
				t.Fatalf("validation did not precede session discovery: %v", err)
			}
		})
	}
}

func TestOperationInputPreservesJSONNumbersAndBoundsBatch(t *testing.T) {
	op := namedOperation(t, "invoke_plugin_operation")
	args, err := op.arguments(json.RawMessage(`{"pluginId":"p","operationId":"op","input":{"id":9007199254740993}}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(args)
	if err != nil || !strings.Contains(string(raw), "9007199254740993") {
		t.Fatalf("rounded operation input: %s, %v", raw, err)
	}
	items := make([]map[string]any, 33)
	for i := range items {
		items[i] = args
	}
	raw, _ = json.Marshal(map[string]any{"items": items})
	if _, err := namedOperation(t, "invoke_plugin_batch").arguments(raw); err == nil {
		t.Fatal("accepted oversized batch")
	}
	args["input"] = map[string]string{"text": strings.Repeat("x", 32*1024)}
	raw, _ = json.Marshal(args)
	if _, err := op.arguments(raw); err == nil {
		t.Fatal("accepted oversized input")
	}
}

func TestCLIInvokeReturnsAcceptedExecutionWithoutPolling(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/operations/invoke" {
			t.Errorf("unexpected polling route: %s", r.URL.Path)
		}
		var args map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&args) != nil || string(args["idempotencyKey"]) != `"request-1"` {
			t.Error("lost idempotency key")
		}
		io.WriteString(w, `{"code":1,"message":"ok","data":{"executionId":"e1","state":"queued"}}`)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"cli", "operations", "invoke", "--args", `{"pluginId":"p","operationId":"op","input":{},"idempotencyKey":"request-1"}`, "--session-file", clientSession(t, server.URL)}, io.NopCloser(strings.NewReader("")), &stdout, &stderr, "test")
	if code != 0 || requests != 1 || !strings.Contains(stdout.String(), `"executionId":"e1"`) {
		t.Fatalf("exit=%d requests=%d stdout=%s stderr=%s", code, requests, &stdout, &stderr)
	}
}

func TestOperationClientPreservesHostErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":0,"message":"multiple pages match","data":{"errorCode":"ambiguous_session"}}`)
	}))
	defer server.Close()
	_, err := newClient(clientSession(t, server.URL), time.Second).call(context.Background(), namedOperation(t, "invoke_plugin_operation"), json.RawMessage(`{"pluginId":"p","operationId":"op","input":{}}`))
	var ce *commandError
	if !errors.As(err, &ce) || ce.kind != "ambiguous_session" {
		t.Fatalf("lost host error code: %v", err)
	}
}

func TestOperationToolSchemasAreStableAndBounded(t *testing.T) {
	names := map[string]bool{}
	for _, op := range operations {
		if names[op.name] {
			t.Fatalf("duplicate tool %s", op.name)
		}
		names[op.name] = true
		schema := op.schema()
		if schema["additionalProperties"] != false {
			t.Fatalf("unbounded outer arguments: %s", op.name)
		}
		if op.argument == "json" && !strings.HasPrefix(op.path, "/api/operations/") {
			t.Fatalf("unexpected control route: %s", op.path)
		}
	}
	if len(names) != 20 {
		t.Fatalf("expected 8 existing and 12 operation tools, got %d", len(names))
	}
	args, err := namedOperation(t, "read_text_artifact").arguments(json.RawMessage(`{"id":"a"}`))
	if err != nil || args["offset"] != 0 || args["limit"] != 32768 {
		t.Fatalf("unexpected text defaults: %v %v", args, err)
	}
}
