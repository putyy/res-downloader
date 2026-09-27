package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"res-downloader/internal/metadata"
	shared "res-downloader/internal/model"
	"strings"
	"time"
)

const (
	maxPluginStoreIndexSize  int64 = 4 * 1024 * 1024
	maxPluginStoreEntries          = 1000
	officialPluginStoreOwner       = "putyy"
	pluginStoreIndexPath           = "/extensions/index.json"
)

var downloadStorePluginArchive = downloadPluginArchive

func (m *PluginManager) PluginStore(ctx context.Context) (shared.PluginStoreIndex, bool, string, error) {
	var index shared.PluginStoreIndex
	raw, _, err := metadata.Fetch(ctx, pluginStoreIndexPath, func(raw []byte) error {
		_, err := decodePluginStoreIndex(raw)
		return err
	})
	if err == nil {
		index, err = decodePluginStoreIndex(raw)
	}

	if err == nil {
		if cacheErr := writePluginStoreCache(m.pluginStoreCacheFile(), index); cacheErr != nil {
			m.logger.Esg(cacheErr, "cache plugin store index")
		}
		return index, false, "", nil
	}

	cached, cacheErr := readPluginStoreCache(m.pluginStoreCacheFile())
	if cacheErr == nil {
		return cached, true, err.Error(), nil
	}
	return shared.PluginStoreIndex{}, false, "", fmt.Errorf("load plugin store: %w", err)
}

func (m *PluginManager) pluginStoreCacheFile() string {
	return filepath.Join(filepath.Dir(m.pluginDir), "plugin-store-cache.json")
}

func decodePluginStoreIndex(raw []byte) (shared.PluginStoreIndex, error) {
	var index shared.PluginStoreIndex
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(&index); err != nil {
		return index, fmt.Errorf("decode plugin store index: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return index, err
	}
	if err := validatePluginStoreIndex(index); err != nil {
		return index, err
	}
	return index, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("plugin store index contains trailing JSON")
		}
		return fmt.Errorf("decode plugin store index: %w", err)
	}
	return nil
}

func validatePluginStoreIndex(index shared.PluginStoreIndex) error {
	if index.SchemaVersion != shared.PluginStoreSchemaVersion {
		return fmt.Errorf("unsupported plugin store schema version %d", index.SchemaVersion)
	}
	if index.Topic != shared.PluginStoreTopic {
		return fmt.Errorf("unexpected plugin store topic %q", index.Topic)
	}
	if _, err := time.Parse(time.RFC3339, index.GeneratedAt); err != nil {
		return errors.New("plugin store generatedAt must use RFC 3339")
	}
	if len(index.Extensions) > maxPluginStoreEntries {
		return fmt.Errorf("plugin store contains more than %d entries", maxPluginStoreEntries)
	}
	ids := make(map[string]bool)
	for position, entry := range index.Extensions {
		if err := validatePluginStoreEntry(entry); err != nil {
			return fmt.Errorf("plugin store entry %d: %w", position, err)
		}
		if entry.ID != "" {
			if ids[entry.ID] {
				return fmt.Errorf("plugin store contains duplicate plugin id %q", entry.ID)
			}
			ids[entry.ID] = true
		}
	}
	return nil
}

func validatePluginStoreEntry(entry shared.PluginStoreEntry) error {
	if entry.Name == "" || len(entry.Name) > 200 || entry.Repository == "" || len(entry.Repository) > 200 {
		return errors.New("name and repository are required")
	}
	if len(entry.Description) > 2000 || len(entry.StatusMessage) > 1000 {
		return errors.New("entry text is too long")
	}
	if err := validateGitHubRepositoryURL(entry.Repository, entry.RepositoryURL); err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}
	if entry.Homepage != "" {
		if err := validatePublicWebURL(entry.Homepage); err != nil {
			return fmt.Errorf("invalid homepage: %w", err)
		}
	}
	if entry.Source != shared.PluginSourceOfficial && entry.Source != shared.PluginSourceCommunity {
		return fmt.Errorf("invalid plugin source %q", entry.Source)
	}
	officialOwner := strings.EqualFold(entry.Owner, officialPluginStoreOwner)
	if officialOwner != (entry.Source == shared.PluginSourceOfficial) {
		return errors.New("plugin source does not match repository owner")
	}
	if entry.Status != shared.PluginStoreAvailable && entry.Status != shared.PluginStoreUnavailable {
		return fmt.Errorf("invalid status %q", entry.Status)
	}
	if entry.Status == shared.PluginStoreUnavailable {
		return nil
	}
	if entry.Manifest == nil || entry.Release == nil {
		return errors.New("available entry requires manifest and release")
	}
	if !validIdentifier(entry.ID) || entry.Manifest.ID != entry.ID {
		return errors.New("entry id does not match a valid manifest id")
	}
	if err := validateManifestForSource(*entry.Manifest, entry.Source == shared.PluginSourceOfficial); err != nil {
		return fmt.Errorf("invalid store manifest: %w", err)
	}
	if entry.Manifest.Version == "" || entry.Release.Version != entry.Manifest.Version {
		return errors.New("release version does not match manifest")
	}
	release := entry.Release
	if release.Tag == "" || len(release.Tag) > 200 {
		return errors.New("release tag is required")
	}
	if strings.TrimPrefix(release.Tag, "v") != release.Version {
		return errors.New("release tag must match manifest version with an optional v prefix")
	}
	if err := validateGitHubTagArchiveURL(entry.Repository, release.Tag, release.ArchiveURL); err != nil {
		return fmt.Errorf("invalid release archive URL: %w", err)
	}
	if release.AcceleratedURL != "" {
		if err := validateJSDelivrArchiveURL(entry.Repository, release.Tag, release.AcceleratedURL); err != nil {
			return fmt.Errorf("invalid accelerated archive URL: %w", err)
		}
	}
	return nil
}

func validatePublicWebURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || len(rawURL) > 4096 {
		return errors.New("URL must be public HTTPS")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return errors.New("URL cannot use a local host")
	}
	if ip := net.ParseIP(host); ip != nil && forbiddenPluginDownloadIP(ip) {
		return errors.New("URL cannot use a private or local address")
	}
	return nil
}

func validateGitHubRepositoryURL(repository, rawURL string) error {
	if err := validatePublicWebURL(rawURL); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	if !strings.EqualFold(parsed.Hostname(), "github.com") || strings.Trim(parsed.EscapedPath(), "/") != repository {
		return errors.New("repository URL must match its github.com owner/repository path")
	}
	return nil
}

func validateGitHubTagArchiveURL(repository, tag, rawURL string) error {
	if err := validatePublicWebURL(rawURL); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	decodedPath, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return errors.New("archive URL path is invalid")
	}
	expectedPath := "/" + repository + "/archive/refs/tags/" + tag + ".zip"
	if !strings.EqualFold(parsed.Hostname(), "github.com") || decodedPath != expectedPath || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("archive URL must match its github.com repository and tag")
	}
	return nil
}

func validateJSDelivrArchiveURL(repository, tag, rawURL string) error {
	if err := validatePublicWebURL(rawURL); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	decodedPath, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return errors.New("accelerated archive URL path is invalid")
	}
	expectedPath := "/gh/" + repository + "@" + tag + "/dist/plugin.zip"
	if !strings.EqualFold(parsed.Hostname(), "cdn.jsdelivr.net") || decodedPath != expectedPath || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("accelerated archive URL must match its jsDelivr repository and tag")
	}
	return nil
}

func writePluginStoreCache(fileName string, index shared.PluginStoreIndex) error {
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(fileName), ".plugin-store-cache-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
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
	return os.Rename(temporaryName, fileName)
}

func readPluginStoreCache(fileName string) (shared.PluginStoreIndex, error) {
	raw, err := os.ReadFile(fileName)
	if err != nil {
		return shared.PluginStoreIndex{}, err
	}
	if int64(len(raw)) > maxPluginStoreIndexSize {
		return shared.PluginStoreIndex{}, errors.New("cached plugin store index is too large")
	}
	return decodePluginStoreIndex(raw)
}

func (m *PluginManager) localArchiveStoreMatch(manifest shared.PluginManifest) string {
	index, err := readPluginStoreCache(m.pluginStoreCacheFile())
	if err != nil {
		return "cache-unavailable"
	}
	for _, entry := range index.Extensions {
		if entry.ID != manifest.ID {
			continue
		}
		if entry.Release != nil && entry.Release.Version == manifest.Version {
			return "same-version"
		}
		return "different"
	}
	return "not-listed"
}

func (m *PluginManager) LocalArchiveStoreMatch(manifest shared.PluginManifest) string {
	return m.localArchiveStoreMatch(manifest)
}

// InstallFromStore resolves download URLs from the trusted cached index. The
// accelerated package is preferred, while the GitHub tag archive remains a
// validation-aware fallback.
func (m *PluginManager) InstallFromStore(ctx context.Context, id, version string, approvePermissions bool) (shared.PluginManifest, string, error) {
	index, err := readPluginStoreCache(m.pluginStoreCacheFile())
	if err != nil {
		return shared.PluginManifest{}, "", fmt.Errorf("load plugin store cache: %w", err)
	}
	var entry *shared.PluginStoreEntry
	for position := range index.Extensions {
		candidate := &index.Extensions[position]
		if candidate.ID == id && candidate.Status == shared.PluginStoreAvailable && candidate.Release != nil &&
			candidate.Release.Version == version {
			entry = candidate
			break
		}
	}
	if entry == nil || entry.Manifest == nil || entry.Release == nil {
		return shared.PluginManifest{}, "", errors.New("plugin version is not available in the cached store index")
	}

	replace := false
	if installed, exists := m.Status(id); exists {
		comparison, compareErr := compareSemanticVersions(version, installed.Manifest.Version)
		if compareErr != nil {
			return shared.PluginManifest{}, "", compareErr
		}
		if comparison <= 0 {
			return shared.PluginManifest{}, "", fmt.Errorf("plugin %q version %s is already installed or newer", id, installed.Manifest.Version)
		}
		replace = true
	}

	urls := make([]string, 0, 2)
	if entry.Release.AcceleratedURL != "" {
		urls = append(urls, entry.Release.AcceleratedURL)
	}
	urls = append(urls, entry.Release.ArchiveURL)
	official := entry.Source == shared.PluginSourceOfficial
	failures := make([]string, 0, len(urls))
	for _, rawURL := range urls {
		downloadContext := ctx
		cancel := func() {}
		if rawURL == entry.Release.AcceleratedURL {
			downloadContext, cancel = context.WithTimeout(ctx, 12*time.Second)
		}
		data, downloadErr := downloadStorePluginArchive(downloadContext, m.networkSettings(), rawURL)
		cancel()
		if downloadErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", rawURL, downloadErr))
			continue
		}
		manifest, _, inspectErr := inspectPluginArchiveDetails(data, official)
		if inspectErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", rawURL, inspectErr))
			continue
		}
		if manifest.ID != entry.ID || manifest.Version != entry.Release.Version || !reflect.DeepEqual(manifest, *entry.Manifest) {
			failures = append(failures, fmt.Sprintf("%s: package manifest does not match the store index", rawURL))
			continue
		}
		installed, installErr := m.installArchiveApprovedForSource(
			data, replace, entry.ID, entry.Release.Version, "", approvePermissions, entry.Source,
		)
		if installErr != nil {
			return shared.PluginManifest{}, rawURL, installErr
		}
		return installed, rawURL, nil
	}
	return shared.PluginManifest{}, "", fmt.Errorf("download plugin from all store sources: %s", strings.Join(failures, "; "))
}
