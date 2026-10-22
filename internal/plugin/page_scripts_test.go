package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
	"time"
)

func TestPageBridgeProbeUsesExistingSessionAndOriginChecks(t *testing.T) {
	manager := &PluginManager{pages: newPageBridgeHub(nil)}
	session := manager.pages.create("example.page", "hook", "https://www.example.com/watch", "www.example.com")
	endpoint := "https://www.example.com" + pageBridgePrefix + session.id + "/" + session.token + "/probe"
	for _, scenario := range []struct {
		name, method, origin, address string
		status                        int
	}{
		{"valid", http.MethodGet, "https://www.example.com", endpoint, http.StatusOK},
		{"no origin", http.MethodGet, "", endpoint, http.StatusOK},
		{"wrong method", http.MethodPost, "https://www.example.com", endpoint, http.StatusMethodNotAllowed},
		{"wrong origin", http.MethodGet, "https://other.example.com", endpoint, http.StatusForbidden},
		{"wrong token", http.MethodGet, "https://www.example.com", "https://www.example.com" + pageBridgePrefix + session.id + "/invalid/probe", http.StatusForbidden},
		{"wrong host", http.MethodGet, "", "https://other.example.com" + pageBridgePrefix + session.id + "/" + session.token + "/probe", http.StatusForbidden},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := httptest.NewRequest(scenario.method, scenario.address, nil)
			request.Header.Set("Origin", scenario.origin)
			response, handled := manager.HandlePageBridge(request)
			if !handled || response.StatusCode != scenario.status {
				t.Fatalf("handled=%v, response=%#v", handled, response)
			}
			defer response.Body.Close()
			if scenario.status == http.StatusOK {
				var result struct {
					OK         bool   `json:"ok"`
					Bridge     string `json:"bridge"`
					APIVersion int    `json:"apiVersion"`
				}
				if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
					t.Fatal(err)
				}
				if !result.OK || result.Bridge != "res-downloader" || result.APIVersion != shared.PluginAPIVersion {
					t.Fatalf("unexpected probe response: %#v", result)
				}
			}
		})
	}
	manager.pages.closeSession(session)
	response, handled := manager.HandlePageBridge(httptest.NewRequest(http.MethodGet, endpoint, nil))
	if !handled || response.StatusCode != http.StatusForbidden {
		t.Fatal("closed session should reject probes locally")
	}
	response.Body.Close()
}

func TestPageBridgeMessageDoesNotRequireProbe(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(`function onPageMessage(message) { return {ok:true,data:message}; }`), 0600); err != nil {
		t.Fatal(err)
	}
	runtime, err := newJavaScriptPlugin(directory, shared.PluginManifest{
		ID: "example.page", Entry: "main.js", APIVersion: shared.PluginAPIVersion,
		Permissions: shared.PluginPermissions{Capabilities: []string{"page-bridge"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := &PluginManager{pages: newPageBridgeHub(nil), plugins: []managedPlugin{{runtime: runtime}}}
	session := manager.pages.create("example.page", "hook", "https://www.example.com/watch", "www.example.com")
	endpoint := "https://www.example.com" + pageBridgePrefix + session.id + "/" + session.token + "/message"
	response, handled := manager.HandlePageBridge(httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{"type":"sample-message"}`)))
	if !handled || response.StatusCode != http.StatusOK {
		t.Fatal("message requests should be handled without probing")
	}
	defer response.Body.Close()
	var result shared.PageMessageResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("message was rejected without a probe: %#v", result)
	}
}

func TestValidateManifestPageScripts(t *testing.T) {
	manifest := shared.PluginManifest{
		ID: "example.page", Name: "Page", Version: "1.0.0", APIVersion: shared.PluginAPIVersion, Runtime: "javascript", Entry: "main.js",
		Permissions: shared.PluginPermissions{Domains: []string{"*.example.com"}, Capabilities: []string{"inject-page-script", "page-bridge"}},
		PageScripts: []shared.PluginPageScript{{
			ID: "hook", Entry: "page/hook.js", RunAt: "document-start", Frames: "top", Bridge: true,
			Match: []shared.PluginPageScriptMatch{{Host: "www.example.com", Path: "/watch*"}},
		}},
	}
	if err := validateManifest(manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Permissions.Capabilities = []string{"inject-page-script"}
	if err := validateManifest(manifest); err == nil {
		t.Fatal("bridge without page-bridge permission should be rejected")
	}
	manifest.Permissions.Capabilities = []string{"inject-page-script", "page-bridge", "emit-resource", "enqueue-download"}
	if err := validateManifest(manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Permissions.Capabilities = []string{"inject-page-script", "page-bridge", "enqueue-download"}
	if err := validateManifest(manifest); err == nil {
		t.Fatal("enqueue-download without emit-resource permission should be rejected")
	}
}

func TestValidatePageScriptFilesRejectsEscapingEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.js"), []byte("function onObservation(){}"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := shared.PluginManifest{PageScripts: []shared.PluginPageScript{{ID: "hook", Entry: "../outside.js"}}}
	if err := validatePageScriptFiles(root, manifest); err == nil {
		t.Fatal("escaping page script entry should be rejected")
	}
}

func TestPageBridgeSessionDoesNotExpireDuringCapture(t *testing.T) {
	hub := newPageBridgeHub(nil)
	session := hub.create("example.page", "hook", "https://www.example.com/watch?v=1", "www.example.com")
	if session == nil {
		t.Fatal("expected page bridge session")
	}
	if !hub.startCapture(session, "capture-key") {
		t.Fatal("expected capture to start")
	}
	hub.mu.Lock()
	session.lastSeen = time.Now().Add(-pageSessionIdleTimeout - time.Minute)
	hub.mu.Unlock()

	if current, ok := hub.session(session.id, session.token); !ok || current != session {
		t.Fatal("active capture session should not expire")
	}
}

func TestPageBridgeSessionDoesNotExpireWithEventConnection(t *testing.T) {
	hub := newPageBridgeHub(nil)
	session := hub.create("example.page", "hook", "https://www.example.com/watch?v=1", "www.example.com")
	if session == nil {
		t.Fatal("expected page bridge session")
	}
	session.eventsMu.Lock()
	session.events = 1
	session.eventsMu.Unlock()
	hub.mu.Lock()
	session.lastSeen = time.Now().Add(-pageSessionIdleTimeout - time.Minute)
	hub.mu.Unlock()

	if current, ok := hub.session(session.id, session.token); !ok || current != session {
		t.Fatal("session with an event connection should not expire")
	}
}

func TestPageBridgeIdleSessionExpires(t *testing.T) {
	hub := newPageBridgeHub(nil)
	session := hub.create("example.page", "hook", "https://www.example.com/watch?v=1", "www.example.com")
	if session == nil {
		t.Fatal("expected page bridge session")
	}
	hub.mu.Lock()
	session.lastSeen = time.Now().Add(-pageSessionIdleTimeout - time.Minute)
	hub.mu.Unlock()

	if _, ok := hub.session(session.id, session.token); ok {
		t.Fatal("idle session should expire")
	}
}

func TestPageBridgeCapacityDoesNotEvictActiveCapture(t *testing.T) {
	hub := newPageBridgeHub(nil)
	active := hub.create("example.page", "hook", "https://www.example.com/watch?v=active", "www.example.com")
	if active == nil || !hub.startCapture(active, "capture-key") {
		t.Fatal("expected active capture session")
	}

	for index := 0; index < maxPageSessionsPerPlugin; index++ {
		if session := hub.create("example.page", "hook", "https://www.example.com/watch?v=prefetch", "www.example.com"); session == nil {
			t.Fatal("idle page session should be evicted before the active capture")
		}
	}
	if current, ok := hub.session(active.id, active.token); !ok || current != active {
		t.Fatal("capacity pruning evicted the active capture session")
	}
}

func TestPageBridgeCapacityRejectsNewSessionWhenAllAreBusy(t *testing.T) {
	hub := newPageBridgeHub(nil)
	for index := 0; index < maxPageSessionsPerPlugin; index++ {
		session := hub.create("example.page", "hook", "https://www.example.com/watch?v=active", "www.example.com")
		if session == nil || !hub.startCapture(session, "capture-key") {
			t.Fatal("expected active capture session")
		}
	}

	if session := hub.create("example.page", "hook", "https://www.example.com/watch?v=prefetch", "www.example.com"); session != nil {
		t.Fatal("new session should be rejected instead of evicting a busy session")
	}
}

func TestPageBridgeCloseIsDeferredDuringCapture(t *testing.T) {
	hub := newPageBridgeHub(nil)
	session := hub.create("example.page", "hook", "https://www.example.com/watch?v=active", "www.example.com")
	if session == nil || !hub.startCapture(session, "capture-key") {
		t.Fatal("expected active capture session")
	}

	if closed, deferred := hub.closeSession(session); closed || !deferred {
		t.Fatal("page close should be deferred during capture")
	}
	if current, ok := hub.session(session.id, session.token); !ok || current != session {
		t.Fatal("deferred close removed the active capture session")
	}

	hub.finishCapture(session, "capture-key")
	if closed, deferred := hub.closeSession(session); !closed || deferred {
		t.Fatal("idle page session should close normally")
	}
	if _, ok := hub.session(session.id, session.token); ok {
		t.Fatal("closed page session is still registered")
	}
}

func TestPageBridgeRequestAllowedUsesCapturedHostForRelativePageURL(t *testing.T) {
	hub := newPageBridgeHub(nil)
	session := hub.create("example.page", "hook", "/watch?v=1", "www.example.com")
	if session == nil {
		t.Fatal("expected page bridge session")
	}

	request := &http.Request{Host: "www.example.com", Header: http.Header{"Origin": []string{"https://www.example.com"}}}
	if !pageBridgeRequestAllowed(session, request) {
		t.Fatal("same-host bridge request should be accepted when the intercepted page URL is relative")
	}
	request.Header.Del("Origin")
	if !pageBridgeRequestAllowed(session, request) {
		t.Fatal("same-host bridge request without Origin should be accepted")
	}
	request.Header.Set("Origin", "https://evil.example")
	if pageBridgeRequestAllowed(session, request) {
		t.Fatal("cross-origin bridge request should be rejected")
	}
	request.Header.Set("Origin", "https://www.example.com")
	request.Host = "other.example.com"
	if pageBridgeRequestAllowed(session, request) {
		t.Fatal("bridge request for another host should be rejected")
	}
}

func TestPageBridgeAutomaticDownloadUsesPublishedGroup(t *testing.T) {
	resource := shared.ResourceCandidate{ID: "resource-id", GroupKey: "video:1", Source: shared.ResourceSource{PluginID: "example.page"}}
	sink := &collectingResourceSink{resources: []shared.ResourceCandidate{resource}}
	var queued shared.ResourceCandidate
	manager := &PluginManager{resources: sink, pageDownload: func(candidate shared.ResourceCandidate) error {
		queued = candidate
		return nil
	}}

	if err := manager.enqueuePageDownloads([]shared.ResourceCandidate{{GroupKey: resource.GroupKey, Source: resource.Source}}); err != nil {
		t.Fatal(err)
	}
	if queued.ID != resource.ID {
		t.Fatalf("queued resource is %#v", queued)
	}
}
