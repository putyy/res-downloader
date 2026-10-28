package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	m "res-downloader/internal/model"
	"res-downloader/internal/operation"
	"strings"
	"testing"
	"time"
)

func TestOperationReadyOriginThroughBridge(t *testing.T) {
	for _, test := range []struct {
		name       string
		sessionURL string
		pageURL    string
		body       string
		accept     bool
	}{
		{name: "MITM default HTTPS port", sessionURL: "https://example.test:443/watch", pageURL: "https://example.test/watch", accept: true},
		{name: "browser explicit default port", sessionURL: "https://example.test/watch", pageURL: "https://example.test:443/watch", accept: true},
		{name: "HTTP default port", sessionURL: "http://example.test:80/watch", pageURL: "http://example.test/watch", accept: true},
		{name: "scheme and host case", sessionURL: "https://EXAMPLE.TEST:443/watch", pageURL: "HTTPS://example.test/watch", accept: true},
		{name: "numeric default port", sessionURL: "https://example.test:00443/watch", pageURL: "https://example.test/watch", accept: true},
		{name: "matching nondefault port", sessionURL: "https://example.test:8443/watch", pageURL: "https://example.test:08443/watch", accept: true},
		{name: "IPv6 normalized", sessionURL: "https://[2001:db8::1]:443/watch", pageURL: "https://[2001:0db8:0:0:0:0:0:1]/watch", accept: true},
		{name: "different host", pageURL: "https://other.test/watch"},
		{name: "different scheme", pageURL: "http://example.test/watch"},
		{name: "different port", pageURL: "https://example.test:8443/watch"},
		{name: "credentials", pageURL: "https://user@example.test/watch"},
		{name: "empty page URL"},
		{name: "relative page URL", pageURL: "/watch"},
		{name: "empty host", pageURL: "https:///watch"},
		{name: "invalid port", pageURL: "https://example.test:abc/watch"},
		{name: "out of range port", pageURL: "https://example.test:65536/watch"},
		{name: "empty explicit port", pageURL: "https://example.test:/watch"},
		{name: "invalid IPv6", pageURL: "https://[example.test]/watch"},
		{name: "unbracketed IPv6", sessionURL: "https://[::1]:443/watch", pageURL: "https://::1:443/watch"},
		{name: "IPv6 zone", sessionURL: "https://[fe80::1]/watch", pageURL: "https://[fe80::1%25en0]/watch"},
		{name: "invalid session origin", sessionURL: "/watch", pageURL: "https://example.test/watch"},
		{name: "malformed JSON", body: `{"pageUrl":`},
		{name: "unknown JSON field", body: `{"pageUrl":"https://example.test/watch","unexpected":true}`},
		{name: "wrong JSON field type", body: `{"pageUrl":"https://example.test/watch","ready":"true"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			sessionURL := test.sessionURL
			if sessionURL == "" {
				sessionURL = "https://example.test:443/watch"
			}
			hub := newPageBridgeHub(nil)
			// The inner HTTP Host need not retain the CONNECT default port.
			p := hub.create("example.operation", "controller", sessionURL, "example.test")
			if p == nil {
				t.Fatal("create page session")
			}
			manager := &PluginManager{pages: hub, operations: &operation.Service{}}
			body := test.body
			if body == "" {
				raw, err := json.Marshal(map[string]interface{}{
					"title": "Example", "pageUrl": test.pageURL, "ready": true,
					"login": "unknown", "context": "video123456",
				})
				if err != nil {
					t.Fatal(err)
				}
				body = string(raw)
			}
			request := httptest.NewRequest(http.MethodPost, "https://example.test"+pageBridgePrefix+p.id+"/"+p.token+"/operation-ready", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://example.test")
			response, handled := manager.HandlePageBridge(request)
			if !handled || response == nil {
				t.Fatal("ready request not handled")
			}
			defer response.Body.Close()
			var reply struct {
				OK   bool `json:"ok"`
				Data struct {
					Revision string `json:"revision"`
				} `json:"data"`
			}
			if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
				t.Fatal(err)
			}
			if !test.accept {
				if response.StatusCode != http.StatusBadRequest || reply.OK || p.ready || p.revision != "" || p.pageURL != sessionURL || p.title != "" || p.context != "" {
					t.Fatalf("invalid readiness accepted or changed session: status=%d", response.StatusCode)
				}
				return
			}
			if response.StatusCode != http.StatusOK || !reply.OK || reply.Data.Revision == "" || reply.Data.Revision != p.revision {
				t.Fatalf("valid readiness rejected or missing revision: status=%d", response.StatusCode)
			}
			sessions := manager.Sessions()
			if len(sessions) != 1 || !sessions[0].Ready || sessions[0].Revision != reply.Data.Revision || sessions[0].PageURL != test.pageURL {
				t.Fatal("ready session not exposed by discovery")
			}
		})
	}
}

func TestOperationDiscoveryHidesClosedDocumentWithCapture(t *testing.T) {
	hub := newPageBridgeHub(nil)
	p := hub.create("example.operation", "controller", "https://www.example.com/", "www.example.com")
	p.events = 1
	p.ready = true
	p.revision = "r"
	if !hub.startCapture(p, "pending") {
		t.Fatal("cannot start capture")
	}
	manager := &PluginManager{pages: hub}
	if len(manager.Sessions()) != 1 {
		t.Fatal("connected page missing")
	}
	removed, deferred := hub.closeSession(p)
	if removed || !deferred {
		t.Fatal("pending capture cleanup should be deferred")
	}
	if len(manager.Sessions()) != 0 {
		t.Fatal("closed document still consumes operation lease")
	}
}
func TestOperationDispatchTargetsOneLiveRevision(t *testing.T) {
	hub := newPageBridgeHub(nil)
	target := hub.create("test.plugin", "page", "https://example.com/", "example.com")
	other := hub.create("test.plugin", "page", "https://example.com/", "example.com")
	target.events = 1
	other.events = 1
	target.revision = "r1"
	manager := &PluginManager{pages: hub}
	execution := m.OperationExecution{ExecutionID: "execution", PluginID: "test.plugin", PageSessionID: target.id, Revision: "wrong"}
	if manager.Send(execution, m.OperationRequest{}, "invoke") == nil {
		t.Fatal("stale revision accepted")
	}
	execution.Revision = "r1"
	if err := manager.Send(execution, m.OperationRequest{}, "invoke"); err != nil {
		t.Fatal(err)
	}
	if len(target.messages) != 1 || len(other.messages) != 0 {
		t.Fatal("operation was broadcast or lost")
	}
	raw := string(<-target.messages)
	if strings.Contains(raw, target.token) || strings.Contains(raw, other.token) {
		t.Fatal("bridge token leaked into operation message")
	}
}
func TestOperationManifestPermissionAndSchemaValidation(t *testing.T) {
	for _, change := range []func(*m.PluginManifest){func(v *m.PluginManifest) {
		o := v.Operations["inspect"]
		o.InputSchema["pattern"] = ".*"
		v.Operations["inspect"] = o
	}, func(v *m.PluginManifest) {
		o := v.Operations["inspect"]
		o.Effects = []string{"publish"}
		v.Operations["inspect"] = o
	}, func(v *m.PluginManifest) {
		o := v.Operations["inspect"]
		o.SafeRetry = true
		o.Effects = []string{"page"}
		v.Operations["inspect"] = o
	}} {
		manifest := operationTestManifest()
		change(&manifest)
		if validateManifest(manifest) == nil {
			t.Fatal("invalid operation declaration accepted")
		}
	}
}

func TestOperationReloadRequiresPageEffect(t *testing.T) {
	manifest := operationTestManifest()
	op := manifest.Operations["inspect"]
	op.AllowReload = true
	manifest.Operations["inspect"] = op
	if validateManifest(manifest) == nil {
		t.Fatal("read-only operation allowed a page reload")
	}
	op.Effects = []string{"read", "page"}
	manifest.Operations["inspect"] = op
	if err := validateManifest(manifest); err != nil {
		t.Fatal(err)
	}
}

func TestOperationReloadIdentityRejectsOldPluginGeneration(t *testing.T) {
	manifest := operationTestManifest()
	hub := newPageBridgeHub(nil)
	manager := &PluginManager{pages: hub, operationGeneration: 7, plugins: []managedPlugin{{runtime: &handledTestPlugin{manifest: manifest}}}}
	page := hub.create(manifest.ID, "controller", "https://www.example.com/watch", "www.example.com", 7)
	page.login, page.context = "unknown", "content-one"
	identity, ok := manager.ReloadIdentity(page.id)
	if !ok || identity.PluginID != manifest.ID || identity.ScriptID != "controller" || identity.PageURL != page.pageURL || identity.Login != page.login || identity.Context != page.context || identity.RuntimeID != "7" {
		t.Fatal("reload identity lost a binding")
	}
	// A same-version plugin reload must invalidate even a late page injection
	// created from the old snapshot after closeAll already ran.
	manager.operationGeneration = 8
	if _, ok := manager.ReloadIdentity(page.id); ok {
		t.Fatal("old-generation page retained its reload identity")
	}
	if sessions := manager.Sessions(); len(sessions) != 1 || len(sessions[0].Operations) != 0 {
		t.Fatal("old-generation page exposed current operations")
	}
	page.operationGeneration = 8
	page.documentClosed = true
	if _, ok := manager.ReloadIdentity(page.id); ok {
		t.Fatal("closed document retained its reload identity")
	}
	raw, err := json.Marshal(manager.Operations())
	if err != nil || strings.Contains(string(raw), "RuntimeID") || strings.Contains(string(raw), "runtimeId") {
		t.Fatal("internal runtime generation leaked through discovery")
	}
}

func TestOperationReloadThroughAuthenticatedBridge(t *testing.T) {
	manifest := operationTestManifest()
	op := manifest.Operations["inspect"]
	op.AllowReload, op.Cancellable = true, true
	op.Effects = []string{"read", "page"}
	manifest.Operations["inspect"] = op
	hub := newPageBridgeHub(nil)
	manager := &PluginManager{pages: hub, operationGeneration: 1, plugins: []managedPlugin{{runtime: &handledTestPlugin{manifest: manifest}}}}
	service, err := operation.New(filepath.Join(t.TempDir(), "operations.db"), manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	manager.SetOperations(service)
	request := func(page *pageBridgeSession, action string, input interface{}, status int) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "https://www.example.com"+pageBridgePrefix+page.id+"/"+page.token+"/operation-"+action, strings.NewReader(string(raw)))
		req.Header.Set("Origin", "https://www.example.com")
		req.Header.Set("Content-Type", "application/json")
		response, handled := manager.HandlePageBridge(req)
		if !handled || response == nil {
			t.Fatal("operation reload route missing")
		}
		defer response.Body.Close()
		var body struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil || response.StatusCode != status || body.OK != (status == http.StatusOK) {
			t.Fatalf("unexpected %s response: status %d", action, response.StatusCode)
		}
		return body.Data
	}
	newPage := func() *pageBridgeSession {
		t.Helper()
		page := hub.create(manifest.ID, "controller", "https://www.example.com:443/watch", "www.example.com", 1)
		if page == nil {
			t.Fatal("create page")
		}
		page.eventsMu.Lock()
		page.events = 1
		page.eventsMu.Unlock()
		request(page, "ready", map[string]interface{}{"pageUrl": "https://www.example.com/watch", "ready": true, "login": "unknown", "context": "content-one"}, http.StatusOK)
		return page
	}
	readInvoke := func(page *pageBridgeSession) map[string]interface{} {
		t.Helper()
		select {
		case raw := <-page.messages:
			var message map[string]interface{}
			if err := json.Unmarshal(raw, &message); err != nil {
				t.Fatal(err)
			}
			return message
		case <-time.After(3 * time.Second):
			t.Fatal("operation was not delivered")
			return nil
		}
	}
	old := newPage()
	execution, err := service.Submit(m.OperationRequest{PluginID: manifest.ID, OperationID: "inspect", PageSessionID: old.id, Input: map[string]interface{}{}}, "desktop")
	if err != nil {
		t.Fatal(err)
	}
	readInvoke(old)
	input := map[string]interface{}{"executionId": execution.ExecutionID, "revision": old.revision}
	request(old, "claim", input, http.StatusOK)
	var ticket m.OperationReloadTicket
	if err := json.Unmarshal(request(old, "reload", input, http.StatusOK), &ticket); err != nil || len(ticket.Token) != 64 {
		t.Fatal("prepare did not return a bounded reload ticket")
	}
	hub.closeSession(old)
	fresh := newPage()
	resume := map[string]interface{}{"executionId": execution.ExecutionID, "revision": fresh.revision, "token": ticket.Token}
	request(fresh, "resume", resume, http.StatusOK)
	message := readInvoke(fresh)
	if message["executionId"] != execution.ExecutionID || message["reloadCount"] != float64(1) || message["revision"] != fresh.revision {
		t.Fatal("resume did not invoke the same execution on its new owner")
	}
	request(fresh, "resume", resume, http.StatusConflict)
	request(fresh, "claim", map[string]interface{}{"executionId": execution.ExecutionID, "revision": fresh.revision}, http.StatusOK)
	current, err := service.Get(execution.ExecutionID)
	if err != nil || current.State != "running" || current.PageSessionID != fresh.id || current.ReloadCount != 1 {
		t.Fatal("resumed execution did not claim its new owner")
	}
}

func TestOperationReloadWaitsOnlyForOldConnection(t *testing.T) {
	attempts := 0
	if err := waitForOperationReload(context.Background(), func() error {
		attempts++
		if attempts == 1 {
			return operation.ErrReloadPageConnected
		}
		return nil
	}); err != nil || attempts != 2 {
		t.Fatal("asynchronous old connection close prevented continuation")
	}
	rejected := errors.New("page identity changed")
	attempts = 0
	if err := waitForOperationReload(context.Background(), func() error {
		attempts++
		return rejected
	}); !errors.Is(err, rejected) || attempts != 1 {
		t.Fatal("non-connection rejection was retried")
	}
	ctx, cancel := context.WithCancel(context.Background())
	attempts = 0
	if err := waitForOperationReload(ctx, func() error {
		attempts++
		cancel()
		return operation.ErrReloadPageConnected
	}); !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatal("cancelled bridge request continued waiting")
	}
}

func TestOperationReportUsesInvokedRevisionAfterPageChange(t *testing.T) {
	for _, terminal := range []string{"cancelled", "succeeded"} {
		t.Run(terminal, func(t *testing.T) {
			manifest := operationTestManifest()
			hub := newPageBridgeHub(nil)
			manager := &PluginManager{pages: hub, plugins: []managedPlugin{{runtime: &handledTestPlugin{manifest: manifest}}}}
			service, err := operation.New(filepath.Join(t.TempDir(), "operations.db"), manager)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = service.Close() })
			manager.SetOperations(service)
			page := hub.create(manifest.ID, "controller", "https://www.example.com/watch", "www.example.com")
			page.eventsMu.Lock()
			page.events = 1
			page.eventsMu.Unlock()
			request := func(action string, input interface{}, status int) {
				t.Helper()
				raw, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "https://www.example.com"+pageBridgePrefix+page.id+"/"+page.token+"/operation-"+action, strings.NewReader(string(raw)))
				req.Header.Set("Origin", "https://www.example.com")
				response, handled := manager.HandlePageBridge(req)
				if !handled || response == nil {
					t.Fatal("operation bridge request was not handled")
				}
				defer response.Body.Close()
				if response.StatusCode != status {
					t.Fatalf("%s status = %d, want %d", action, response.StatusCode, status)
				}
			}
			ready := func(context string) {
				request("ready", map[string]interface{}{"pageUrl": "https://www.example.com/watch", "ready": true, "login": "unknown", "context": context}, http.StatusOK)
			}
			ready("before")
			req := m.OperationRequest{PluginID: manifest.ID, OperationID: "inspect", Input: map[string]interface{}{}}
			execution, err := service.Submit(req, "desktop")
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-page.messages:
			case <-time.After(3 * time.Second):
				t.Fatal("operation was not delivered")
			}
			request("claim", map[string]string{"executionId": execution.ExecutionID, "revision": execution.Revision}, http.StatusOK)
			ready("after")
			request("report", map[string]string{"executionId": execution.ExecutionID, "state": terminal}, http.StatusConflict)
			request("report", m.OperationReport{ExecutionID: execution.ExecutionID, Revision: "wrong", State: terminal}, http.StatusConflict)
			request("report", m.OperationReport{ExecutionID: execution.ExecutionID, Revision: execution.Revision, State: terminal, Data: json.RawMessage(`{}`)}, http.StatusOK)
			ended, err := service.Get(execution.ExecutionID)
			if err != nil || ended.State != "interrupted" || len(ended.Result) != 0 || ended.CancelConfirmed != (terminal == "cancelled") {
				t.Fatalf("stale report changed the outcome: %#v %v", ended, err)
			}
			if _, err := service.Submit(req, "desktop"); err != nil {
				t.Fatal("stopped handler retained its page lease", err)
			}
		})
	}
}
