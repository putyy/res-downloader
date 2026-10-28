package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMarkDuplicatePluginIDs(t *testing.T) {
	entries := []shared.PluginStoreEntry{
		{ID: "com.example.video", Repository: "one/repo", Status: shared.PluginStoreAvailable, Release: &shared.PluginStoreRelease{}},
		{ID: "com.example.video", Repository: "two/repo", Status: shared.PluginStoreAvailable, Release: &shared.PluginStoreRelease{}},
	}
	markDuplicatePluginIDs(entries)
	for _, entry := range entries {
		if entry.Status != shared.PluginStoreUnavailable || entry.Release != nil || entry.ID != "" {
			t.Fatalf("duplicate entry was not disabled: %#v", entry)
		}
		if !strings.Contains(entry.StatusMessage, "multiple repositories") {
			t.Fatalf("unexpected status message: %q", entry.StatusMessage)
		}
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
	}, options{topic: shared.PluginStoreTopic}, &bytes.Buffer{})
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
	if release.Tag != "v1.0.0" || release.Version() != "1.0.0" {
		t.Fatalf("unexpected release: %#v", release)
	}
	if !release.Accelerated {
		t.Fatal("expected an accelerated package")
	}
	raw, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"manifest", "repositoryUrl", "homepage", "owner", "ownerAvatarUrl", "forks", "notesUrl", "archiveUrl", "acceleratedUrl", "version", "match", "entry"} {
		if bytes.Contains(raw, []byte(`"`+field+`":`)) {
			t.Fatalf("index contains redundant field %q: %s", field, raw)
		}
	}
	if len(entry.Capabilities) != 1 || entry.Capabilities[0] != "observe-response" {
		t.Fatalf("capabilities = %v", entry.Capabilities)
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
	repo := repository{Name: "repo", FullName: "owner/repo", LatestRelease: &release{TagName: "v1.0.0"}}
	entry := buildRepositoryEntry(context.Background(), &githubClient{
		client: &http.Client{Timeout: time.Second}, jsDelivrURL: server.URL, rawURL: server.URL,
	}, repo, &bytes.Buffer{})
	if entry.Status != shared.PluginStoreAvailable || entry.Release == nil || entry.Release.Accelerated {
		t.Fatalf("unexpected entry: %#v", entry)
	}
	if entry.Source != shared.PluginSourceCommunity {
		t.Fatalf("source = %q", entry.Source)
	}
}

const extensionTestManifest = `{"id":"com.example.demo","name":"Demo","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":["observe-response"]},"match":[]}`

func TestRepositorySearchSplitsCrowdedRangesAndDeduplicatesPages(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Second)
	var splitQueries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var body struct {
			Variables struct {
				Query string  `json:"query"`
				First int     `json:"first"`
				After *string `json:"after"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Variables.First != 100 {
			t.Errorf("page size = %d", body.Variables.First)
		}
		if !strings.Contains(body.Variables.Query, " sort:updated-desc ") {
			t.Errorf("unexpected repository sort: %s", body.Variables.Query)
		}
		var response graphQLResponse
		search := &response.Data.Search
		if strings.HasSuffix(body.Variables.Query, "created:"+start.Format(time.RFC3339)+".."+end.Format(time.RFC3339)) {
			search.RepositoryCount = 1200
		} else {
			offset := 0
			if body.Variables.After != nil {
				var err error
				offset, err = strconv.Atoi(*body.Variables.After)
				if err != nil {
					t.Error(err)
					return
				}
			}
			base := 0
			if strings.HasSuffix(body.Variables.Query, "created:"+end.Format(time.RFC3339)+".."+end.Format(time.RFC3339)) {
				base = 600
			} else if !strings.HasSuffix(body.Variables.Query, "created:"+start.Format(time.RFC3339)+".."+start.Format(time.RFC3339)) {
				t.Errorf("unexpected creation range: %s", body.Variables.Query)
			}
			if body.Variables.After == nil {
				splitQueries.Add(1)
			}
			search.RepositoryCount = 600
			for position := offset; position < offset+100 && position < 600; position++ {
				id := base + position
				if id == 600 {
					id = 0 // An overlapping result must not create a duplicate entry.
				}
				search.Nodes = append(search.Nodes, repository{FullName: fmt.Sprintf("owner/repo-%d", id)})
			}
			search.PageInfo.HasNextPage = offset+100 < 600
			search.PageInfo.EndCursor = strconv.Itoa(offset + 100)
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	github := &githubClient{client: server.Client(), graphqlURL: server.URL}
	repositories := make([]repository, 0)
	if err := github.searchRepositoryRange(context.Background(), shared.PluginStoreTopic, start, end, &repositories, make(map[string]bool)); err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1199 || splitQueries.Load() != 2 {
		t.Fatalf("repositories = %d, split ranges = %d", len(repositories), splitQueries.Load())
	}
}

func TestRepositorySearchRejectsIncompleteResults(t *testing.T) {
	for _, test := range []struct {
		name, response string
	}{
		{"crowded-second", `{"data":{"search":{"repositoryCount":1001}}}`},
		{"stalled-pagination", `{"data":{"search":{"repositoryCount":1,"pageInfo":{"hasNextPage":true,"endCursor":""},"nodes":[{"nameWithOwner":"owner/repo"}]}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				_, _ = fmt.Fprint(w, test.response)
			}))
			defer server.Close()
			github := &githubClient{client: server.Client(), graphqlURL: server.URL}
			repositories := make([]repository, 0)
			start := time.Unix(0, 0).UTC()
			if err := github.searchRepositoryRange(context.Background(), shared.PluginStoreTopic, start, start, &repositories, make(map[string]bool)); err == nil {
				t.Fatal("accepted incomplete search results")
			}
		})
	}
}

func TestWriteIndexRejectsInvalidEntriesWithoutReplacingExistingFile(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*shared.PluginStoreEntry)
	}{
		{"long-name", func(entry *shared.PluginStoreEntry) { entry.Name = strings.Repeat("x", 201) }},
		{"invalid-repository", func(entry *shared.PluginStoreEntry) { entry.Repository = "owner/../repo" }},
		{"missing-release", func(entry *shared.PluginStoreEntry) { entry.Release = nil }},
		{"invalid-tag", func(entry *shared.PluginStoreEntry) { entry.Release.Tag = "not-a-version" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			index := shared.PluginStoreIndex{
				SchemaVersion: shared.PluginStoreSchemaVersion,
				GeneratedAt:   "2026-01-01T00:00:00Z",
				Topic:         shared.PluginStoreTopic,
				Extensions: []shared.PluginStoreEntry{{
					ID: "com.example.demo", Name: "Demo", Repository: "owner/repo",
					Source: shared.PluginSourceCommunity, Status: shared.PluginStoreAvailable,
					Release: &shared.PluginStoreRelease{Tag: "v1.0.0"},
				}},
			}
			fileName := filepath.Join(t.TempDir(), "index.json")
			if err := writeIndex(fileName, index); err != nil {
				t.Fatal(err)
			}
			previous, err := os.ReadFile(fileName)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&index.Extensions[0])
			if err := writeIndex(fileName, index); err == nil || !strings.Contains(err.Error(), "validate plugin store index") {
				t.Fatalf("expected index validation error, got %v", err)
			}
			current, err := os.ReadFile(fileName)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(current, previous) {
				t.Fatal("invalid index replaced the existing file")
			}
		})
	}
}
