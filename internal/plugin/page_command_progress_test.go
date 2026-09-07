package plugin

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	shared "res-downloader/internal/model"
)

func trackedCommandFixture(t *testing.T) (*PluginManager, []*pageBridgeSession, string) {
	t.Helper()
	manifest := pageCommandTestManifest()
	action := manifest.Actions["inspect"]
	action.TrackProgress = true
	manifest.Actions["inspect"] = action
	hub := newPageBridgeHub(nil)
	manager := &PluginManager{pages: hub, statuses: map[string]shared.PluginStatus{
		manifest.ID: {Manifest: manifest, Loaded: true},
	}}
	sessions := []*pageBridgeSession{
		hub.create(manifest.ID, "controller", "https://www.example.com/watch/42", "www.example.com"),
		hub.create(manifest.ID, "controller", "https://www.example.com/watch/42", "www.example.com"),
	}
	for _, s := range sessions {
		s.events = 1
	}
	dispatch, err := manager.DispatchPageCommand(shared.ResourceCandidate{
		ID: "resource-42", Source: shared.ResourceSource{PluginID: manifest.ID},
		Actions: []shared.ResourceAction{{ID: "inspect"}},
	}, "inspect")
	if err != nil {
		t.Fatal(err)
	}
	return manager, sessions, dispatch.RequestID
}

func TestPageCommandHasExactlyOneOwner(t *testing.T) {
	m, sessions, id := trackedCommandFixture(t)
	var wg sync.WaitGroup
	accepted := make(chan *pageBridgeSession, 2)
	for _, s := range sessions {
		wg.Add(1)
		go func(s *pageBridgeSession) {
			defer wg.Done()
			ok, _, err := m.pages.claimCommand(s, id, "")
			if err != nil {
				t.Error(err)
			}
			if ok {
				accepted <- s
			}
		}(s)
	}
	wg.Wait()
	if len(accepted) != 1 {
		t.Fatalf("owners = %d", len(accepted))
	}
	owner := <-accepted
	for _, s := range sessions {
		if s != owner {
			if err := m.pages.reportCommand(s, shared.PageCommandReport{RequestID: id, State: "completed"}); err == nil {
				t.Fatal("non-owner completed command")
			}
			if err := m.pages.reportCommand(s, shared.PageCommandReport{RequestID: id, State: "rejected"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	progress := 42.0
	if err := m.pages.reportCommand(owner, shared.PageCommandReport{RequestID: id, State: "running", Progress: &progress}); err != nil {
		t.Fatal(err)
	}
	view := m.PageCommandStatuses()[0]
	*view.Progress = 0
	if *m.PageCommandStatuses()[0].Progress != 42 {
		t.Fatal("status shared mutable progress")
	}
	if err := m.pages.reportCommand(owner, shared.PageCommandReport{RequestID: id, State: "completed"}); err != nil {
		t.Fatal(err)
	}
	if err := m.pages.reportCommand(owner, shared.PageCommandReport{RequestID: id, State: "running"}); err == nil {
		t.Fatal("terminal state was overwritten")
	}
}

func TestPageCommandRejectsAllMismatchesAndForeignPages(t *testing.T) {
	m, sessions, id := trackedCommandFixture(t)
	foreign := m.pages.create("foreign", "controller", "https://www.example.com/watch/42", "www.example.com")
	late := m.pages.create(sessions[0].pluginID, "controller", sessions[0].pageURL, sessions[0].host)
	for _, s := range []*pageBridgeSession{foreign, late} {
		if _, _, err := m.pages.claimCommand(s, id, ""); err == nil {
			t.Fatal("unaddressed session claimed command")
		}
		if err := m.pages.reportCommand(s, shared.PageCommandReport{RequestID: id, State: "rejected"}); err == nil {
			t.Fatal("unaddressed session rejected command")
		}
	}
	for _, s := range sessions {
		if err := m.pages.reportCommand(s, shared.PageCommandReport{RequestID: id, State: "rejected"}); err != nil {
			t.Fatal(err)
		}
	}
	if m.PageCommandStatuses()[0].State != "failed" || m.PageCommandStatuses()[0].ErrorCode != "page_command_target_unavailable" {
		t.Fatal("all mismatches did not fail")
	}
}

func TestPageCommandResumeRotatesSecretAndRejectsOldOwner(t *testing.T) {
	m, sessions, id := trackedCommandFixture(t)
	_, token, err := m.pages.claimCommand(sessions[0], id, "")
	if err != nil {
		t.Fatal(err)
	}
	resumed := m.pages.create(sessions[0].pluginID, "controller", sessions[0].pageURL, sessions[0].host)
	if _, _, err := m.pages.claimCommand(resumed, id, "wrong"); err == nil {
		t.Fatal("bad resume token accepted")
	}
	ok, next, err := m.pages.claimCommand(resumed, id, token)
	if err != nil || !ok || token == next {
		t.Fatalf("resume: %v %v", ok, err)
	}
	if _, _, err := m.pages.claimCommand(sessions[0], id, token); err == nil {
		t.Fatal("replayed resume token accepted")
	}
	if err := m.pages.reportCommand(sessions[0], shared.PageCommandReport{RequestID: id, State: "running"}); err == nil {
		t.Fatal("old owner retained write access")
	}
	raw, _ := json.Marshal(m.PageCommandStatuses())
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), next) {
		t.Fatal("status leaked resume secret")
	}
}

func TestPageCommandBoundsTimeoutAndReload(t *testing.T) {
	m, sessions, id := trackedCommandFixture(t)
	_, _, _ = m.pages.claimCommand(sessions[0], id, "")
	for _, progress := range []float64{-1, 101, math.NaN(), math.Inf(1)} {
		if err := m.pages.reportCommand(sessions[0], shared.PageCommandReport{RequestID: id, State: "running", Progress: &progress}); err == nil {
			t.Fatal("invalid progress accepted")
		}
	}
	if err := m.pages.reportCommand(sessions[0], shared.PageCommandReport{RequestID: id, State: "running", Message: strings.Repeat("x", 1025)}); err == nil {
		t.Fatal("oversized message accepted")
	}
	m.pages.commands[id].view.UpdatedAt = time.Now().Add(-91 * time.Second).UnixMilli()
	if m.PageCommandStatuses()[0].State != "failed" || m.PageCommandStatuses()[0].ErrorCode != "page_command_timeout" {
		t.Fatal("stale command did not fail")
	}
	m.pages.commands[id].view.UpdatedAt = time.Now().Add(-11 * time.Minute).UnixMilli()
	if len(m.PageCommandStatuses()) != 0 {
		t.Fatal("expired command retained")
	}
	m, _, _ = trackedCommandFixture(t)
	m.pages.closeAll()
	if m.PageCommandStatuses()[0].State != "failed" || m.PageCommandStatuses()[0].ErrorCode != "page_command_reloaded" {
		t.Fatal("reload did not fail command")
	}
}

func TestPageCommandPendingTimeoutDuplicateAndCapacity(t *testing.T) {
	m, sessions, id := trackedCommandFixture(t)
	resource := shared.ResourceCandidate{ID: "resource-42", Source: shared.ResourceSource{PluginID: sessions[0].pluginID}, Actions: []shared.ResourceAction{{ID: "inspect"}}}
	var commandError *PageCommandError
	if _, err := m.DispatchPageCommand(resource, "inspect"); !errors.As(err, &commandError) || commandError.Code != "page_command_already_active" {
		t.Fatalf("expected duplicate error code, got %v", err)
	}
	m.pages.commands[id].view.UpdatedAt = time.Now().Add(-31 * time.Second).UnixMilli()
	if m.PageCommandStatuses()[0].State != "failed" || m.PageCommandStatuses()[0].ErrorCode != "page_command_not_accepted" {
		t.Fatal("pending command did not time out")
	}
	for len(m.pages.commands) < maxTrackedPageCommands {
		key := strings.Repeat("x", len(m.pages.commands)+1)
		m.pages.commands[key] = &trackedPageCommand{view: shared.PageCommandStatus{State: "completed", UpdatedAt: time.Now().UnixMilli()}}
	}
	if _, err := m.DispatchPageCommand(resource, "inspect"); !errors.As(err, &commandError) || commandError.Code != "page_command_limit_reached" {
		t.Fatalf("expected capacity error code, got %v", err)
	}
}

func TestPageCommandEndpointRejectsInvalidInputAndMissingPermission(t *testing.T) {
	m, sessions, id := trackedCommandFixture(t)
	for _, body := range []string{
		`{"requestId":"` + id + `","extra":true}`,
		`{"requestId":"` + id + `","progress":"50"}`,
		`{"requestId":"` + id + `"} {}`,
		strings.Repeat(" ", 4097),
	} {
		response := m.handlePageCommandRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), sessions[0], "command-report")
		if response.StatusCode < 400 {
			t.Fatalf("accepted invalid report: %s", body)
		}
		_ = response.Body.Close()
	}
	status := m.statuses[sessions[0].pluginID]
	status.Manifest.Permissions.Capabilities = nil
	m.statuses[sessions[0].pluginID] = status
	response := m.handlePageCommandRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"requestId":"`+id+`"}`)), sessions[0], "command-claim")
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("missing permission: %d", response.StatusCode)
	}
}
