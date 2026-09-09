package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"res-downloader/internal/control"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func clientSession(t *testing.T, endpoint string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	raw, err := json.Marshal(control.Session{Version: 1, URL: endpoint, Token: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCLIUsesResourceIDAndReturnsTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/download/create" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("a", 64) {
			t.Error("unexpected request route or missing credential")
		}
		var input map[string]string
		if json.NewDecoder(r.Body).Decode(&input) != nil || input["id"] != "resource-1" {
			t.Error("resource ID was not mapped to the desktop API")
		}
		io.WriteString(w, `{"code":1,"message":"ok","data":{"id":"task-1","state":"pending"}}`)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"cli", "downloads", "create", "--resource-id", "resource-1", "--session-file", clientSession(t, server.URL)}, io.NopCloser(strings.NewReader("")), &stdout, &stderr, "test")
	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"id":"task-1"`) {
		t.Fatalf("unexpected result: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
	}
}

func TestCLIValidatesBeforeDiscovery(t *testing.T) {
	for _, args := range [][]string{
		{"cli", "downloads", "pause"},
		{"cli", "resources", "list", "--limit", "5001"},
		{"cli", "downloads", "list", "--id", "wrong"},
		{"mcp", "--stdio=false"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), append(args, "--session-file", filepath.Join(t.TempDir(), "missing")), io.NopCloser(strings.NewReader("")), &stdout, &stderr, "test")
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid_arguments") {
			t.Fatalf("args=%v: exit=%d stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
	}
}

func TestClientDoesNotFollowRedirectsOrRetryMutations(t *testing.T) {
	requests := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("followed credential-bearing redirect") }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	c := newClient(clientSession(t, source.URL), time.Second)
	_, err := c.call(context.Background(), operations[1], json.RawMessage(`{"resourceId":"r1"}`))
	if err == nil || requests != 1 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
}

func TestMCPDiscoversToolsWithoutDesktopAndReportsToolErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := newClient(filepath.Join(t.TempDir(), "missing"), time.Second)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := newMCPServer(c, "test").Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 8 {
		t.Fatalf("tools=%v err=%v", list, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_resources", Arguments: map[string]any{"limit": 10}})
	if err != nil || !result.IsError {
		t.Fatalf("result=%v err=%v", result, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "pause_download", Arguments: map[string]any{"id": ""}})
	if err != nil || !result.IsError {
		t.Fatalf("result=%v err=%v", result, err)
	}
}

func TestClientFollowsNewSessionAndFindsTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":1,"message":"ok","data":[{"id":"other"},{"id":"wanted","state":"completed"}]}`)
	}))
	defer server.Close()
	path := clientSession(t, server.URL)
	c := newClient(path, time.Second)
	result, err := c.call(context.Background(), operations[3], json.RawMessage(`{"id":"wanted"}`))
	if err != nil || !strings.Contains(string(result.Data), "completed") {
		t.Fatalf("result=%v err=%v", result, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := c.call(context.Background(), operations[3], json.RawMessage(`{"id":"wanted"}`)); err == nil {
		t.Fatal("client cached an expired session")
	}
	newPath := clientSession(t, server.URL)
	raw, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.call(context.Background(), operations[3], json.RawMessage(`{"id":"wanted"}`)); err != nil {
		t.Fatal(err)
	}
}
