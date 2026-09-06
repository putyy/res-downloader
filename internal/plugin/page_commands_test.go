package plugin

import (
	"encoding/json"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
	"time"
)

func TestDispatchPageCommandUsesStoredResourceActionData(t *testing.T) {
	manifest := pageCommandTestManifest()
	hub := newPageBridgeHub(nil)
	session := hub.create(manifest.ID, "controller", "https://www.example.com/watch/42", "www.example.com")
	session.events = 1
	manager := &PluginManager{
		pages: hub,
		statuses: map[string]shared.PluginStatus{
			manifest.ID: {Manifest: manifest, Loaded: true},
		},
	}
	resource := shared.ResourceCandidate{
		ID: "resource-42", GroupKey: "video:42", Source: shared.ResourceSource{PluginID: manifest.ID},
		Actions: []shared.ResourceAction{{ID: "inspect", Data: map[string]interface{}{"assetId": "42"}}},
	}

	dispatch, err := manager.DispatchPageCommand(resource, "inspect")
	if err != nil {
		t.Fatal(err)
	}
	if dispatch.RequestID == "" || dispatch.PageScriptID != "controller" || dispatch.Delivered != 1 {
		t.Fatalf("dispatch = %#v", dispatch)
	}

	select {
	case raw := <-session.messages:
		var message shared.PageCommandMessage
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		if message.Protocol != shared.PageCommandProtocol || message.Type != shared.PageCommandType || message.RequestID != dispatch.RequestID {
			t.Fatalf("message envelope = %#v", message)
		}
		if message.ActionID != "inspect" || message.Resource.ID != resource.ID || message.Resource.GroupKey != resource.GroupKey {
			t.Fatalf("message target = %#v", message)
		}
		if message.Data["assetId"] != "42" {
			t.Fatalf("message data = %#v", message.Data)
		}
	default:
		t.Fatal("page command was not queued")
	}
}

func TestDispatchPageCommandRequiresActiveTargetPage(t *testing.T) {
	manifest := pageCommandTestManifest()
	hub := newPageBridgeHub(nil)
	hub.create(manifest.ID, "another-script", "https://www.example.com/other", "www.example.com")
	manager := &PluginManager{
		pages: hub,
		statuses: map[string]shared.PluginStatus{
			manifest.ID: {Manifest: manifest, Loaded: true},
		},
	}
	resource := shared.ResourceCandidate{
		ID: "resource-42", Source: shared.ResourceSource{PluginID: manifest.ID},
		Actions: []shared.ResourceAction{{ID: "inspect"}},
	}
	if _, err := manager.DispatchPageCommand(resource, "inspect"); err == nil || !strings.Contains(err.Error(), "no active page") {
		t.Fatalf("expected missing page error, got %v", err)
	}
}

func TestValidatePageCommandActionDataLimit(t *testing.T) {
	manifest := pageCommandTestManifest()
	actions := []shared.ResourceAction{{
		ID: "inspect", Data: map[string]interface{}{"payload": strings.Repeat("x", maxPageCommandDataSize)},
	}}
	if err := validateResourceActions(manifest, actions); err == nil {
		t.Fatal("expected oversized page-command data to be rejected")
	}
}

func TestDispatchPageCommandRejectsDisconnectedPages(t *testing.T) {
	for _, expired := range []bool{false, true} {
		manifest := pageCommandTestManifest()
		hub := newPageBridgeHub(nil)
		session := hub.create(manifest.ID, "controller", "https://www.example.com/watch/42", "www.example.com")
		if expired {
			session.lastSeen = time.Now().Add(-pageSessionIdleTimeout - time.Second)
		}
		manager := &PluginManager{pages: hub, statuses: map[string]shared.PluginStatus{
			manifest.ID: {Manifest: manifest, Loaded: true},
		}}
		resource := shared.ResourceCandidate{
			ID: "resource-42", Source: shared.ResourceSource{PluginID: manifest.ID},
			Actions: []shared.ResourceAction{{ID: "inspect"}},
		}
		if _, err := manager.DispatchPageCommand(resource, "inspect"); err == nil || !strings.Contains(err.Error(), "no active page") {
			t.Fatalf("expired=%v: expected missing page error, got %v", expired, err)
		}
		if len(session.messages) != 0 {
			t.Fatal("disconnected page received a command")
		}
		if _, exists := hub.sessions[session.id]; expired && exists {
			t.Fatal("expired page session was not removed")
		}
	}
}

func TestQueuePageCommandFiltersRecipientsAndCountsFullQueues(t *testing.T) {
	hub := newPageBridgeHub(nil)
	connected := hub.create("example.plugin", "controller", "https://example.com/watch/42", "example.com")
	full := hub.create("example.plugin", "controller", "https://example.com/watch/43", "example.com")
	otherScript := hub.create("example.plugin", "other", "https://example.com/", "example.com")
	otherPlugin := hub.create("another.plugin", "controller", "https://example.com/", "example.com")
	for _, session := range []*pageBridgeSession{connected, full, otherScript, otherPlugin} {
		session.events = 1
	}
	for len(full.messages) < cap(full.messages) {
		full.messages <- []byte(`{"existing":true}`)
	}
	raw := []byte(`{"type":"resource-action"}`)
	matched, delivered := hub.queuePageCommand("example.plugin", "controller", raw)
	if matched != 2 || delivered != 1 {
		t.Fatalf("matched=%d delivered=%d, want 2 and 1", matched, delivered)
	}
	if len(connected.messages) != 1 || len(otherScript.messages) != 0 || len(otherPlugin.messages) != 0 {
		t.Fatal("command was not scoped to the available target page")
	}
	connected.events = 0
	matched, delivered = hub.queuePageCommand("example.plugin", "controller", raw)
	if matched != 1 || delivered != 0 {
		t.Fatalf("full queue: matched=%d delivered=%d, want 1 and 0", matched, delivered)
	}
}
