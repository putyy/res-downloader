package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	shared "res-downloader/internal/model"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMarkDuplicatePluginIDs(t *testing.T) {
	manifestA := &shared.PluginManifest{ID: "com.example.video"}
	manifestB := &shared.PluginManifest{ID: "com.example.video"}
	entries := []shared.PluginStoreEntry{
		{ID: manifestA.ID, Repository: "one/repo", Status: shared.PluginStoreAvailable, Manifest: manifestA, Release: &shared.PluginStoreRelease{}},
		{ID: manifestB.ID, Repository: "two/repo", Status: shared.PluginStoreAvailable, Manifest: manifestB, Release: &shared.PluginStoreRelease{}},
	}
	markDuplicatePluginIDs(entries)
	for _, entry := range entries {
		if entry.Status != shared.PluginStoreUnavailable || entry.Manifest != nil || entry.Release != nil || entry.ID != "" {
			t.Fatalf("duplicate entry was not disabled: %#v", entry)
		}
		if !strings.Contains(entry.StatusMessage, "multiple repositories") {
			t.Fatalf("unexpected status message: %q", entry.StatusMessage)
		}
	}
}

func TestSecureHTTPSURLRejectsLocalAddresses(t *testing.T) {
	for _, rawURL := range []string{
		"http://example.com",
		"https://localhost/project",
		"https://plugin.local/project",
		"https://127.0.0.1/project",
		"https://user@example.com/project",
	} {
		if secureHTTPSURL(rawURL) {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
	if !secureHTTPSURL("https://example.com/project") {
		t.Fatal("expected public HTTPS URL to be accepted")
	}
}

func TestBuildIndexUsesTaggedManifestAndHeadCheckedJSDelivrArchive(t *testing.T) {
	var archiveGETs atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/graphql":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("unexpected GraphQL request: %s, %q", request.Method, request.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"data":{"search":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[{"name":"repo","nameWithOwner":"putyy/repo","description":"Demo","url":"https://github.com/putyy/repo","stargazerCount":7,"forkCount":2,"updatedAt":"2026-08-16T00:00:00Z","owner":{"login":"Putyy","avatarUrl":"https://avatars.githubusercontent.com/u/1"},"latestRelease":{"tagName":"v1.0.0","url":"https://github.com/putyy/repo/releases/tag/v1.0.0","publishedAt":"2026-08-16T00:00:00Z"}}]}}}`)
		case request.URL.Path == "/gh/putyy/repo@v1.0.0/plugin.json":
			if request.Method != http.MethodGet {
				t.Errorf("manifest method = %s", request.Method)
			}
			_, _ = fmt.Fprint(w, extensionTestManifest)
		case request.URL.Path == "/gh/putyy/repo@v1.0.0/dist/plugin.zip":
			if request.Method == http.MethodGet {
				archiveGETs.Add(1)
			}
			if request.Method != http.MethodHead {
				t.Errorf("archive method = %s", request.Method)
			}
			w.Header().Set("Content-Length", "1234")
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	index, err := buildIndex(context.Background(), &githubClient{
		client: &http.Client{Timeout: time.Second}, graphqlURL: server.URL + "/graphql",
		jsDelivrURL: server.URL, rawURL: server.URL + "/raw", token: "test-token",
	}, options{topic: shared.PluginStoreTopic, maxRepos: 10}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Extensions) != 1 || index.Extensions[0].Status != shared.PluginStoreAvailable {
		t.Fatalf("unexpected index: %#v", index)
	}
	entry := index.Extensions[0]
	if entry.Source != shared.PluginSourceOfficial {
		t.Fatalf("source = %q", entry.Source)
	}
	release := entry.Release
	if release.Tag != "v1.0.0" || release.ArchiveURL != "https://github.com/putyy/repo/archive/refs/tags/v1.0.0.zip" {
		t.Fatalf("unexpected release: %#v", release)
	}
	if release.AcceleratedURL != "https://cdn.jsdelivr.net/gh/putyy/repo@v1.0.0/dist/plugin.zip" {
		t.Fatalf("accelerated URL = %q", release.AcceleratedURL)
	}
	if archiveGETs.Load() != 0 {
		t.Fatalf("downloaded %d archives while generating index", archiveGETs.Load())
	}
}

func TestMissingJSDelivrArchiveKeepsGitHubFallbackAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/gh/owner/repo@v1.0.0/plugin.json" {
			_, _ = fmt.Fprint(w, extensionTestManifest)
			return
		}
		http.NotFound(w, request)
	}))
	defer server.Close()
	repo := repository{Name: "repo", FullName: "owner/repo", HTMLURL: "https://github.com/owner/repo", LatestRelease: &release{TagName: "v1.0.0"}}
	repo.Owner.Login = "owner"
	entry := buildRepositoryEntry(context.Background(), &githubClient{
		client: &http.Client{Timeout: time.Second}, jsDelivrURL: server.URL, rawURL: server.URL,
	}, repo, &bytes.Buffer{})
	if entry.Status != shared.PluginStoreAvailable || entry.Release == nil || entry.Release.AcceleratedURL != "" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
	if entry.Source != shared.PluginSourceCommunity {
		t.Fatalf("source = %q", entry.Source)
	}
}

const extensionTestManifest = `{"id":"com.example.demo","name":"Demo","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":["observe-response"]},"match":[]}`
