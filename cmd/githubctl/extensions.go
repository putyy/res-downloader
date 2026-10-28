package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	githubGraphQLDefault = "https://api.github.com/graphql"
	jsDelivrDefault      = "https://cdn.jsdelivr.net"
	rawGitHubDefault     = "https://raw.githubusercontent.com"
	officialGitHubOwner  = "putyy"
	githubSearchLimit    = 1000 // GitHub's per-query limit, not a store entry limit.
	maxManifestSize      = int64(512 * 1024)
	maxArchiveSize       = int64(25 * 1024 * 1024)
	indexConcurrency     = 12
)

const repositorySearchQuery = `
query($query: String!, $first: Int!, $after: String) {
  search(query: $query, type: REPOSITORY, first: $first, after: $after) {
    repositoryCount
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Repository {
        name
        nameWithOwner
        description
        stargazerCount
        updatedAt
        licenseInfo { spdxId }
        latestRelease { tagName publishedAt }
      }
    }
  }
}`

type release struct {
	TagName     string `json:"tagName"`
	PublishedAt string `json:"publishedAt"`
}

type repository struct {
	Name          string   `json:"name"`
	FullName      string   `json:"nameWithOwner"`
	Description   string   `json:"description"`
	Stars         int      `json:"stargazerCount"`
	UpdatedAt     string   `json:"updatedAt"`
	LatestRelease *release `json:"latestRelease"`
	License       *struct {
		SPDXID string `json:"spdxId"`
	} `json:"licenseInfo"`
}

type graphQLResponse struct {
	Data struct {
		Search struct {
			RepositoryCount int          `json:"repositoryCount"`
			Nodes           []repository `json:"nodes"`
			PageInfo        struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"search"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type githubClient struct {
	client      *http.Client
	graphqlURL  string
	jsDelivrURL string
	rawURL      string
	token       string
}

type options struct {
	output string
	topic  string
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(data)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("githubctl extensions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "dist/extensions/index.json", "output index JSON path")
	topic := flags.String("topic", shared.PluginStoreTopic, "GitHub repository topic")
	graphqlURL := flags.String("github-graphql", githubGraphQLDefault, "GitHub GraphQL API URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *topic != shared.PluginStoreTopic {
		return fmt.Errorf("topic must be %q for schema v%d", shared.PluginStoreTopic, shared.PluginStoreSchemaVersion)
	}
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		return errors.New("GITHUB_TOKEN is required for the GitHub GraphQL API")
	}
	github := &githubClient{
		client:      &http.Client{Timeout: 30 * time.Second},
		graphqlURL:  strings.TrimRight(*graphqlURL, "/"),
		jsDelivrURL: jsDelivrDefault,
		rawURL:      rawGitHubDefault,
		token:       token,
	}
	configuration := options{output: *output, topic: *topic}
	index, err := buildIndex(ctx, github, configuration, stderr)
	if err != nil {
		return err
	}
	if err := writeIndex(configuration.output, index); err != nil {
		return err
	}
	available := 0
	accelerated := 0
	for _, extension := range index.Extensions {
		if extension.Status == shared.PluginStoreAvailable {
			available++
			if extension.Release != nil && extension.Release.Accelerated {
				accelerated++
			}
		}
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s: %d repositories, %d installable, %d accelerated\n", configuration.output, len(index.Extensions), available, accelerated)
	return nil
}

func buildIndex(ctx context.Context, github *githubClient, configuration options, stderr io.Writer) (shared.PluginStoreIndex, error) {
	repositories, err := github.searchRepositories(ctx, configuration.topic)
	if err != nil {
		return shared.PluginStoreIndex{}, err
	}
	entries := make([]shared.PluginStoreEntry, len(repositories))
	safeStderr := &lockedWriter{writer: stderr}
	semaphore := make(chan struct{}, indexConcurrency)
	var wait sync.WaitGroup
	for position, repo := range repositories {
		position, repo := position, repo
		semaphore <- struct{}{}
		wait.Add(1)
		go func() {
			defer wait.Done()
			defer func() { <-semaphore }()
			entries[position] = buildRepositoryEntry(ctx, github, repo, safeStderr)
		}()
	}
	wait.Wait()

	markDuplicatePluginIDs(entries)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Status != entries[j].Status {
			return entries[i].Status == shared.PluginStoreAvailable
		}
		if entries[i].Source != entries[j].Source {
			return entries[i].Source == shared.PluginSourceOfficial
		}
		if entries[i].Stars != entries[j].Stars {
			return entries[i].Stars > entries[j].Stars
		}
		return entries[i].Repository < entries[j].Repository
	})
	return shared.PluginStoreIndex{
		SchemaVersion: shared.PluginStoreSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Topic:         configuration.topic,
		Extensions:    entries,
	}, nil
}

func buildRepositoryEntry(ctx context.Context, github *githubClient, repo repository, stderr io.Writer) shared.PluginStoreEntry {
	source := shared.PluginSourceCommunity
	owner, _, _ := strings.Cut(repo.FullName, "/")
	if strings.EqualFold(owner, officialGitHubOwner) {
		source = shared.PluginSourceOfficial
	}
	entry := shared.PluginStoreEntry{
		Name: repo.Name, Description: repo.Description, Repository: repo.FullName,
		Stars: repo.Stars, UpdatedAt: repo.UpdatedAt, Source: source,
		Status: shared.PluginStoreUnavailable, StatusMessage: "No installable GitHub Release was found",
	}
	if repo.License != nil && repo.License.SPDXID != "NOASSERTION" {
		entry.License = repo.License.SPDXID
	}
	latest := repo.LatestRelease
	if latest == nil || latest.TagName == "" {
		return entry
	}

	rawManifest, err := github.downloadManifest(ctx, repo.FullName, latest.TagName)
	if err != nil {
		entry.StatusMessage = conciseError(err)
		_, _ = fmt.Fprintf(stderr, "skip package %s: %v\n", repo.FullName, err)
		return entry
	}
	manifest, err := plugin.ParseStoreManifestJSON(rawManifest, source == shared.PluginSourceOfficial)
	if err != nil {
		entry.StatusMessage = "Release plugin.json is not a valid res-downloader manifest"
		_, _ = fmt.Fprintf(stderr, "skip package %s: %v\n", repo.FullName, err)
		return entry
	}
	if strings.TrimPrefix(latest.TagName, "v") != manifest.Version {
		entry.StatusMessage = fmt.Sprintf("Release tag %q does not match plugin version %q", latest.TagName, manifest.Version)
		return entry
	}

	entry.ID = manifest.ID
	entry.Name = manifest.Name
	entry.Author = manifest.Author.Name
	entry.Locales = manifest.Locales
	entry.Capabilities = manifest.Permissions.Capabilities
	entry.Release = &shared.PluginStoreRelease{
		Tag: latest.TagName, PublishedAt: latest.PublishedAt,
	}
	if ok, headErr := github.jsDelivrArchiveExists(ctx, repo.FullName, latest.TagName); headErr != nil {
		_, _ = fmt.Fprintf(stderr, "jsDelivr unavailable for %s@%s: %v\n", repo.FullName, latest.TagName, headErr)
	} else if ok {
		entry.Release.Accelerated = true
	}
	entry.Status = shared.PluginStoreAvailable
	entry.StatusMessage = ""
	return entry
}

func (g *githubClient) searchRepositories(ctx context.Context, topic string) ([]repository, error) {
	repositories := make([]repository, 0)
	seen := make(map[string]bool)
	err := g.searchRepositoryRange(ctx, topic, time.Unix(0, 0).UTC(), time.Now().UTC().Truncate(time.Second), &repositories, seen)
	return repositories, err
}

// Split crowded creation-time ranges before paginating: GitHub only exposes
// the first 1,000 matches for a single search, even when more matches exist.
func (g *githubClient) searchRepositoryRange(ctx context.Context, topic string, start, end time.Time, repositories *[]repository, seen map[string]bool) error {
	query := fmt.Sprintf("topic:%s archived:false fork:false sort:updated-desc created:%s..%s", topic, start.Format(time.RFC3339), end.Format(time.RFC3339))
	after := ""
	for {
		variables := map[string]interface{}{
			"query": query,
			"first": 100,
			"after": nil,
		}
		if after != "" {
			variables["after"] = after
		}
		var response graphQLResponse
		if err := g.graphQL(ctx, repositorySearchQuery, variables, &response); err != nil {
			return fmt.Errorf("search GitHub repositories: %w", err)
		}
		if len(response.Errors) > 0 {
			return fmt.Errorf("search GitHub repositories: %s", response.Errors[0].Message)
		}
		if response.Data.Search.RepositoryCount > githubSearchLimit {
			if !start.Before(end) {
				return fmt.Errorf("GitHub search has more than %d repositories created at %s; cannot retrieve a complete index", githubSearchLimit, start.Format(time.RFC3339))
			}
			middle := start.Add(time.Duration(int64(end.Sub(start)/time.Second)/2) * time.Second)
			if err := g.searchRepositoryRange(ctx, topic, start, middle, repositories, seen); err != nil {
				return err
			}
			return g.searchRepositoryRange(ctx, topic, middle.Add(time.Second), end, repositories, seen)
		}
		for _, repo := range response.Data.Search.Nodes {
			key := strings.ToLower(repo.FullName)
			if key != "" && !seen[key] {
				seen[key] = true
				*repositories = append(*repositories, repo)
			}
		}
		page := response.Data.Search.PageInfo
		if !page.HasNextPage {
			return nil
		}
		if page.EndCursor == "" || page.EndCursor == after || len(response.Data.Search.Nodes) == 0 {
			return errors.New("GitHub search pagination did not advance; cannot retrieve a complete index")
		}
		after = page.EndCursor
	}
}

func (g *githubClient) graphQL(ctx context.Context, query string, variables map[string]interface{}, destination interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.graphqlURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "res-downloader-extension-index")
	request.Header.Set("Authorization", "Bearer "+g.token)
	response, err := g.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("GitHub HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(response.Body, 8*1024*1024)).Decode(destination)
}

func (g *githubClient) downloadManifest(ctx context.Context, repository, tag string) ([]byte, error) {
	primary := joinURL(g.jsDelivrURL, "/gh/"+repository+"@"+tag+"/plugin.json")
	raw, err := g.downloadSmallFile(ctx, primary, maxManifestSize)
	if err == nil {
		return raw, nil
	}
	fallback := joinURL(g.rawURL, "/"+repository+"/"+tag+"/plugin.json")
	raw, fallbackErr := g.downloadSmallFile(ctx, fallback, maxManifestSize)
	if fallbackErr != nil {
		return nil, fmt.Errorf("download tagged plugin.json: jsDelivr: %v; GitHub: %w", err, fallbackErr)
	}
	return raw, nil
}

func (g *githubClient) downloadSmallFile(ctx context.Context, rawURL string, maximum int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "res-downloader-extension-index")
	response, err := g.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return raw, nil
}

func (g *githubClient) jsDelivrArchiveExists(ctx context.Context, repository, tag string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rawURL := joinURL(g.jsDelivrURL, "/gh/"+repository+"@"+tag+"/dist/plugin.zip")
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("User-Agent", "res-downloader-extension-index")
	response, err := g.client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if response.ContentLength == 0 || response.ContentLength > maxArchiveSize {
		return false, fmt.Errorf("invalid content length %d", response.ContentLength)
	}
	return true, nil
}

func joinURL(base, suffix string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/") + suffix
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + suffix
	return parsed.String()
}

func markDuplicatePluginIDs(entries []shared.PluginStoreEntry) {
	positions := make(map[string][]int)
	for index, entry := range entries {
		if entry.Status == shared.PluginStoreAvailable {
			positions[entry.ID] = append(positions[entry.ID], index)
		}
	}
	for id, duplicates := range positions {
		if len(duplicates) < 2 {
			continue
		}
		for _, position := range duplicates {
			entries[position].ID = ""
			entries[position].Release = nil
			entries[position].Status = shared.PluginStoreUnavailable
			entries[position].StatusMessage = fmt.Sprintf("Plugin id %q is claimed by multiple repositories", id)
		}
	}
}

func writeIndex(fileName string, index shared.PluginStoreIndex) error {
	if err := plugin.ValidatePluginStoreIndex(index); err != nil {
		return fmt.Errorf("validate plugin store index: %w", err)
	}
	raw, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if len(raw) > shared.PluginStoreMaxIndexSize {
		return errors.New("plugin store index exceeds 4 MiB")
	}
	if err := os.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(fileName), ".index-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), fileName)
}

func conciseError(err error) string {
	message := err.Error()
	if len(message) > 300 {
		return message[:300]
	}
	return message
}
