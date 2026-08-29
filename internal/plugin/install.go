package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	shared "res-downloader/internal/model"
	"sort"
	"strings"
	"time"
)

const (
	maxPluginArchiveSize    int64 = 25 * 1024 * 1024
	maxPluginExtractedSize  int64 = 64 * 1024 * 1024
	maxPluginArchiveFiles         = 256
	maxPluginArchiveEntries       = maxPluginArchiveFiles * 4
	pluginDownloadTimeout         = 45 * time.Second
)

const MaxPluginArchiveSize = maxPluginArchiveSize

func (m *PluginManager) InstallArchiveApproved(data []byte, replace bool, expectedID, expectedVersion, expectedContentSHA256 string, approvePermissions bool) (shared.PluginManifest, error) {
	return m.installArchiveApproved(data, replace, expectedID, expectedVersion, expectedContentSHA256, approvePermissions)
}

// InstallLocalArchiveApproved installs a package explicitly selected by the
// user. Putyy-owned packages use the official loader; all other local packages
// remain community plugins.
func (m *PluginManager) InstallLocalArchiveApproved(data []byte, replace bool, expectedID, expectedVersion, expectedContentSHA256 string, approvePermissions bool) (shared.PluginManifest, error) {
	manifest, _, err := inspectPluginArchiveDetails(data, true)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	source, err := localPluginSource(manifest)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	return m.installArchiveApprovedForSource(data, replace, expectedID, expectedVersion, expectedContentSHA256, approvePermissions, source)
}

func downloadPluginArchive(ctx context.Context, config NetworkSettings, rawURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, pluginDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "res-downloader-plugin-installer")
	response, err := newPluginHTTPClient(config).Do(req)
	if err != nil {
		return nil, fmt.Errorf("download plugin: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download plugin: unexpected HTTP status %d", response.StatusCode)
	}
	if response.ContentLength > maxPluginArchiveSize {
		return nil, fmt.Errorf("plugin package exceeds %d bytes", maxPluginArchiveSize)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxPluginArchiveSize+1))
	if err != nil {
		return nil, fmt.Errorf("download plugin: %w", err)
	}
	if int64(len(data)) > maxPluginArchiveSize {
		return nil, fmt.Errorf("plugin package exceeds %d bytes", maxPluginArchiveSize)
	}
	return data, nil
}

func newPluginHTTPClient(config NetworkSettings) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 20 * time.Second}
	configuredProxy := ""
	if config.DownloadProxy && !strings.Contains(config.UpstreamProxy, config.Port) {
		configuredProxy = strings.TrimSpace(config.UpstreamProxy)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if configuredProxy != "" {
				return dialer.DialContext(ctx, network, address)
			}
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, address := range addresses {
				if forbiddenPluginDownloadIP(address) {
					return nil, errors.New("plugin download host resolves to a private or local address")
				}
			}
			if len(addresses) == 0 {
				return nil, errors.New("plugin download host has no IP address")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	if configuredProxy != "" {
		transport.Proxy = func(*http.Request) (*url.URL, error) {
			proxyURL, err := url.Parse(configuredProxy)
			if err != nil || proxyURL.Host == "" || proxyURL.User != nil && proxyURL.User.Username() == "" {
				return nil, errors.New("configured download proxy URL is invalid")
			}
			switch proxyURL.Scheme {
			case "http", "https", "socks5", "socks5h":
				return proxyURL, nil
			default:
				return nil, errors.New("configured download proxy must use http, https, socks5, or socks5h")
			}
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   pluginDownloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("plugin download has too many redirects")
			}
			return validatePluginDownloadURL(req.URL.String())
		},
	}
}

func validatePluginDownloadURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || len(rawURL) > 4096 {
		return errors.New("plugin URL must be a valid HTTPS URL without embedded credentials")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && forbiddenPluginDownloadIP(ip) {
		return errors.New("plugin URL cannot use a private or local address")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return errors.New("plugin URL cannot use a private or local host")
	}
	return nil
}

func forbiddenPluginDownloadIP(ip net.IP) bool {
	return ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func validatePluginContentChecksum(expected string) (string, error) {
	expected = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(expected, "sha256:")))
	if expected == "" {
		return "", nil
	}
	if len(expected) != sha256.Size*2 {
		return "", errors.New("content SHA-256 checksum must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return "", errors.New("content SHA-256 checksum is not valid hexadecimal")
	}
	return expected, nil
}

// InspectPluginArchiveDetails validates a package and returns a digest of its
// normalized extracted file tree. ZIP compression and the single generated
// GitHub top-level directory therefore do not affect the digest.
func InspectPluginArchiveDetails(data []byte) (shared.PluginManifest, string, error) {
	return inspectPluginArchiveDetails(data, false)
}

// InspectLocalPluginArchiveDetails validates a package selected from disk.
// The official prefix remains restricted to packages identifying a Putyy
// GitHub repository, while ordinary local packages use community rules.
func InspectLocalPluginArchiveDetails(data []byte) (shared.PluginManifest, string, error) {
	manifest, digest, err := inspectPluginArchiveDetails(data, true)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	if _, err := localPluginSource(manifest); err != nil {
		return shared.PluginManifest{}, "", err
	}
	return manifest, digest, nil
}

func localPluginSource(manifest shared.PluginManifest) (string, error) {
	authorURL, err := url.Parse(strings.TrimSpace(manifest.Author.URL))
	if err == nil && strings.EqualFold(authorURL.Hostname(), "github.com") {
		parts := strings.Split(strings.Trim(authorURL.Path, "/"), "/")
		if len(parts) > 0 && strings.EqualFold(parts[0], officialPluginStoreOwner) {
			return shared.PluginSourceOfficial, nil
		}
	}
	if reservedPluginIDPrefix(manifest.ID) == "official." {
		return "", fmt.Errorf("plugin id %q is official but author URL is not a github.com/%s repository", manifest.ID, officialPluginStoreOwner)
	}
	return shared.PluginSourceCommunity, nil
}

func inspectPluginArchiveDetails(data []byte, official bool) (shared.PluginManifest, string, error) {
	if int64(len(data)) > maxPluginArchiveSize {
		return shared.PluginManifest{}, "", fmt.Errorf("plugin package exceeds %d bytes", maxPluginArchiveSize)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return shared.PluginManifest{}, "", fmt.Errorf("open plugin package: %w", err)
	}
	if err := validatePluginArchiveEntries(archive); err != nil {
		return shared.PluginManifest{}, "", err
	}
	stage, err := os.MkdirTemp("", ".res-downloader-plugin-inspect-")
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	defer os.RemoveAll(stage)
	if err := extractPluginArchive(archive, stage); err != nil {
		return shared.PluginManifest{}, "", err
	}
	pluginRoot, err := locatePackagedPlugin(stage)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	var runtimePlugin shared.RuntimePlugin
	if official {
		runtimePlugin, _, err = LoadOfficialPlugin(pluginRoot)
	} else {
		runtimePlugin, _, err = LoadExternalPlugin(pluginRoot)
	}
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	digest, err := hashPluginDirectory(pluginRoot)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	return runtimePlugin.Manifest(), digest, nil
}

func (m *PluginManager) InstallArchive(data []byte) (shared.PluginManifest, error) {
	return m.installArchive(data, false, "", "", "")
}

func (m *PluginManager) installArchive(data []byte, replace bool, expectedID, expectedVersion, expectedContentSHA256 string) (shared.PluginManifest, error) {
	return m.installArchiveApproved(data, replace, expectedID, expectedVersion, expectedContentSHA256, false)
}

func (m *PluginManager) installArchiveApproved(data []byte, replace bool, expectedID, expectedVersion, expectedContentSHA256 string, approvePermissions bool) (shared.PluginManifest, error) {
	return m.installArchiveApprovedForSource(data, replace, expectedID, expectedVersion, expectedContentSHA256, approvePermissions, shared.PluginSourceCommunity)
}

func (m *PluginManager) installArchiveApprovedForSource(data []byte, replace bool, expectedID, expectedVersion, expectedContentSHA256 string, approvePermissions bool, source string) (shared.PluginManifest, error) {
	m.installMu.Lock()
	defer m.installMu.Unlock()
	if source != shared.PluginSourceOfficial && source != shared.PluginSourceCommunity {
		return shared.PluginManifest{}, errors.New("invalid plugin installation source")
	}
	if int64(len(data)) > maxPluginArchiveSize {
		return shared.PluginManifest{}, fmt.Errorf("plugin package exceeds %d bytes", maxPluginArchiveSize)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return shared.PluginManifest{}, fmt.Errorf("open plugin package: %w", err)
	}
	if err := validatePluginArchiveEntries(archive); err != nil {
		return shared.PluginManifest{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(m.pluginDir), ".plugin-install-")
	if err != nil {
		return shared.PluginManifest{}, err
	}
	defer os.RemoveAll(stage)
	packageRoot := filepath.Join(stage, "package")
	if err := os.Mkdir(packageRoot, 0750); err != nil {
		return shared.PluginManifest{}, err
	}
	if err := extractPluginArchive(archive, packageRoot); err != nil {
		return shared.PluginManifest{}, err
	}
	pluginRoot, err := locatePackagedPlugin(packageRoot)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	var runtimePlugin shared.RuntimePlugin
	if source == shared.PluginSourceOfficial {
		runtimePlugin, _, err = m.loadOfficialPlugin(pluginRoot)
	} else {
		runtimePlugin, _, err = m.loadExternalPlugin(pluginRoot)
	}
	if err != nil {
		return shared.PluginManifest{}, fmt.Errorf("validate plugin: %w", err)
	}
	manifest := runtimePlugin.Manifest()
	expectedDigest, err := validatePluginContentChecksum(expectedContentSHA256)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	if expectedDigest != "" {
		actualDigest, err := hashPluginDirectory(pluginRoot)
		if err != nil {
			return shared.PluginManifest{}, err
		}
		if actualDigest != expectedDigest {
			return shared.PluginManifest{}, errors.New("plugin extracted content SHA-256 checksum does not match")
		}
	}
	if expectedID != "" && manifest.ID != expectedID {
		return shared.PluginManifest{}, fmt.Errorf("plugin package id %q does not match store entry %q", manifest.ID, expectedID)
	}
	if expectedVersion != "" && manifest.Version != expectedVersion {
		return shared.PluginManifest{}, fmt.Errorf("plugin package version %q does not match store entry %q", manifest.Version, expectedVersion)
	}
	target := filepath.Join(m.pluginDir, manifest.ID)
	_, targetErr := os.Stat(target)
	targetExists := targetErr == nil
	if targetExists && !replace {
		return shared.PluginManifest{}, fmt.Errorf("plugin %q is already installed", manifest.ID)
	} else if targetErr != nil && !os.IsNotExist(targetErr) {
		return shared.PluginManifest{}, targetErr
	}
	if replace {
		m.mu.RLock()
		status, exists := m.statuses[manifest.ID]
		m.mu.RUnlock()
		if !targetExists || !exists {
			return shared.PluginManifest{}, fmt.Errorf("plugin %q is not installed", manifest.ID)
		}
		if status.Builtin {
			return shared.PluginManifest{}, errors.New("built-in plugins cannot be updated")
		}
		if status.Bundled && source != shared.PluginSourceOfficial {
			return shared.PluginManifest{}, errors.New("preinstalled plugins are updated with the application")
		}
		comparison, err := compareSemanticVersions(manifest.Version, status.Manifest.Version)
		if err != nil {
			return shared.PluginManifest{}, err
		}
		if comparison < 0 {
			return shared.PluginManifest{}, fmt.Errorf("plugin downgrade from %s to %s requires rollback", status.Manifest.Version, manifest.Version)
		}
		added := pluginPermissionIncrease(status.Manifest.Permissions, manifest.Permissions)
		if len(added) > 0 && !approvePermissions {
			return shared.PluginManifest{}, fmt.Errorf("plugin update requires permission approval: %s", strings.Join(added, ", "))
		}
	}
	backup := ""
	if targetExists {
		backup = filepath.Join(filepath.Dir(m.pluginDir), fmt.Sprintf(".plugin-update-%s-%d", manifest.ID, time.Now().UnixNano()))
		if err := os.Rename(target, backup); err != nil {
			return shared.PluginManifest{}, fmt.Errorf("prepare plugin update: %w", err)
		}
	}
	if err := os.Rename(pluginRoot, target); err != nil {
		if backup != "" {
			_ = os.Rename(backup, target)
		}
		return shared.PluginManifest{}, fmt.Errorf("install plugin: %w", err)
	}
	rollbackFiles := func() {
		_ = os.RemoveAll(target)
		if backup != "" {
			_ = os.Rename(backup, target)
		}
	}
	m.mu.Lock()
	if m.sources == nil {
		m.sources = make(map[string]string)
	}
	oldSource, hadSource := m.sources[manifest.ID]
	wasRemoved := m.removed[manifest.ID]
	m.sources[manifest.ID] = source
	delete(m.removed, manifest.ID)
	m.mu.Unlock()
	rollbackState := func() {
		m.mu.Lock()
		if hadSource {
			m.sources[manifest.ID] = oldSource
		} else {
			delete(m.sources, manifest.ID)
		}
		if wasRemoved {
			m.removed[manifest.ID] = true
		} else {
			delete(m.removed, manifest.ID)
		}
		m.mu.Unlock()
		_ = m.saveSources()
		_ = m.saveRemoved()
	}
	if err := m.saveSources(); err != nil {
		rollbackState()
		rollbackFiles()
		return shared.PluginManifest{}, err
	}
	if err := m.saveRemoved(); err != nil {
		rollbackState()
		rollbackFiles()
		return shared.PluginManifest{}, err
	}
	if replace {
		if err := m.Reload(); err != nil {
			rollbackState()
			rollbackFiles()
			_ = m.Reload()
			return shared.PluginManifest{}, err
		}
		if backup != "" {
			backupRoot := m.pluginBackupRoot()
			if err := os.MkdirAll(backupRoot, 0750); err != nil {
				return shared.PluginManifest{}, err
			}
			retained := filepath.Join(backupRoot, manifest.ID)
			if err := os.RemoveAll(retained); err != nil {
				m.logger.Esg(err, "remove older plugin rollback")
			}
			if err := os.Rename(backup, retained); err != nil {
				m.logger.Esg(err, "retain previous plugin version")
			}
		}
		return manifest, nil
	}
	rollback := func() {
		rollbackState()
		rollbackFiles()
	}
	if err := m.Reload(); err != nil {
		rollback()
		return shared.PluginManifest{}, err
	}
	return manifest, nil
}

func (m *PluginManager) Rollback(id string) (shared.PluginManifest, error) {
	m.installMu.Lock()
	defer m.installMu.Unlock()
	if !validIdentifier(id) {
		return shared.PluginManifest{}, errors.New("invalid plugin id")
	}
	target := filepath.Join(m.pluginDir, id)
	backupRoot := m.pluginBackupRoot()
	backup := filepath.Join(backupRoot, id)
	if _, err := os.Stat(backup); err != nil {
		return shared.PluginManifest{}, errors.New("plugin rollback is unavailable")
	}
	rollbackSource := shared.PluginSourceCommunity
	var runtimePlugin shared.RuntimePlugin
	var err error
	if isBundledPluginID(id) {
		runtimePlugin, _, err = LoadBundledPlugin(backup)
		if err == nil {
			rollbackSource = shared.PluginSourceBuiltin
		} else {
			runtimePlugin, _, err = m.loadOfficialPlugin(backup)
			rollbackSource = shared.PluginSourceOfficial
		}
	} else {
		m.mu.RLock()
		rollbackSource = m.sources[id]
		m.mu.RUnlock()
		if rollbackSource == shared.PluginSourceOfficial {
			runtimePlugin, _, err = m.loadOfficialPlugin(backup)
		} else {
			rollbackSource = shared.PluginSourceCommunity
			runtimePlugin, _, err = m.loadExternalPlugin(backup)
		}
	}
	if err != nil {
		return shared.PluginManifest{}, fmt.Errorf("validate rollback: %w", err)
	}
	if runtimePlugin.Manifest().ID != id {
		return shared.PluginManifest{}, errors.New("rollback plugin id does not match")
	}
	stage := filepath.Join(backupRoot, fmt.Sprintf(".%s-current-%d", id, time.Now().UnixNano()))
	if err := os.Rename(target, stage); err != nil {
		return shared.PluginManifest{}, err
	}
	if err := os.Rename(backup, target); err != nil {
		_ = os.Rename(stage, target)
		return shared.PluginManifest{}, err
	}
	if err := os.Rename(stage, backup); err != nil {
		_ = os.Rename(target, stage)
		_ = os.Rename(backup, target)
		_ = os.Rename(stage, backup)
		return shared.PluginManifest{}, err
	}
	m.mu.Lock()
	oldSource, hadSource := m.sources[id]
	if rollbackSource == shared.PluginSourceBuiltin {
		delete(m.sources, id)
	} else {
		m.sources[id] = rollbackSource
	}
	m.mu.Unlock()
	if err := m.saveSources(); err != nil {
		m.mu.Lock()
		if hadSource {
			m.sources[id] = oldSource
		} else {
			delete(m.sources, id)
		}
		m.mu.Unlock()
		_ = os.Rename(backup, stage)
		_ = os.Rename(target, backup)
		_ = os.Rename(stage, target)
		return shared.PluginManifest{}, err
	}
	if err := m.Reload(); err != nil {
		failed := filepath.Join(backupRoot, fmt.Sprintf(".%s-failed-%d", id, time.Now().UnixNano()))
		_ = os.Rename(target, failed)
		_ = os.Rename(backup, target)
		_ = os.Rename(failed, backup)
		m.mu.Lock()
		if hadSource {
			m.sources[id] = oldSource
		} else {
			delete(m.sources, id)
		}
		m.mu.Unlock()
		_ = m.saveSources()
		_ = m.Reload()
		return shared.PluginManifest{}, err
	}
	return runtimePlugin.Manifest(), nil
}

func hashPluginDirectory(directory string) (string, error) {
	return hashPluginFS(os.DirFS(directory), ".")
}

func hashPluginFS(fileSystem fs.FS, root string) (string, error) {
	root = path.Clean(root)
	prefix := ""
	if root != "." {
		prefix = root + "/"
	}
	paths := make([]string, 0)
	if err := fs.WalkDir(fileSystem, root, func(fileName string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("plugin contains unsupported file %q", fileName)
		}
		relative := strings.TrimPrefix(fileName, prefix)
		if relative == fileName && prefix != "" {
			return fmt.Errorf("plugin file %q is outside %q", fileName, root)
		}
		paths = append(paths, relative)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("res-downloader-plugin-content-v1\x00"))
	for _, relative := range paths {
		content, err := fs.ReadFile(fileSystem, path.Join(root, relative))
		if err != nil {
			return "", err
		}
		if err := binary.Write(hasher, binary.BigEndian, uint64(len(relative))); err != nil {
			return "", err
		}
		_, _ = hasher.Write([]byte(relative))
		if err := binary.Write(hasher, binary.BigEndian, uint64(len(content))); err != nil {
			return "", err
		}
		_, _ = hasher.Write(content)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func extractPluginArchive(archive *zip.Reader, destination string) error {
	var extracted int64
	for _, entry := range archive.File {
		if strings.Contains(entry.Name, "\\") {
			return fmt.Errorf("plugin package entry %q has an invalid path", entry.Name)
		}
		clean := path.Clean(entry.Name)
		if clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("plugin package entry %q escapes the package", entry.Name)
		}
		if isIgnoredPluginArchiveEntry(clean) {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 || (!entry.Mode().IsRegular() && !entry.FileInfo().IsDir()) {
			return fmt.Errorf("plugin package contains unsupported entry %q", entry.Name)
		}
		localPath := filepath.FromSlash(clean)
		if filepath.IsAbs(localPath) || filepath.VolumeName(localPath) != "" {
			return fmt.Errorf("plugin package entry %q has an invalid path", entry.Name)
		}
		target := filepath.Join(destination, localPath)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("plugin package entry %q escapes the package", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0750); err != nil {
				return err
			}
			continue
		}
		if entry.UncompressedSize64 > uint64(maxPluginExtractedSize-extracted) {
			return fmt.Errorf("plugin package expands beyond %d bytes", maxPluginExtractedSize)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
		if err != nil {
			input.Close()
			return err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, maxPluginExtractedSize-extracted+1))
		closeOutputErr := output.Close()
		closeInputErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
		extracted += written
		if extracted > maxPluginExtractedSize {
			return fmt.Errorf("plugin package expands beyond %d bytes", maxPluginExtractedSize)
		}
	}
	return nil
}

func validatePluginArchiveEntries(archive *zip.Reader) error {
	if len(archive.File) == 0 || len(archive.File) > maxPluginArchiveEntries {
		return fmt.Errorf("plugin package must contain between 1 and %d ZIP entries", maxPluginArchiveEntries)
	}
	meaningful := 0
	for _, entry := range archive.File {
		if !isIgnoredPluginArchiveEntry(path.Clean(entry.Name)) {
			meaningful++
		}
	}
	if meaningful == 0 || meaningful > maxPluginArchiveFiles {
		return fmt.Errorf("plugin package must contain between 1 and %d plugin entries", maxPluginArchiveFiles)
	}
	return nil
}

// isIgnoredPluginArchiveEntry filters metadata emitted by macOS Finder when a
// directory is compressed. These entries are not part of the plugin and must
// not affect package-root detection or the canonical content digest.
func isIgnoredPluginArchiveEntry(clean string) bool {
	if clean == "__MACOSX" || strings.HasPrefix(clean, "__MACOSX/") {
		return true
	}
	base := path.Base(clean)
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

func locatePackagedPlugin(packageRoot string) (string, error) {
	if info, err := os.Stat(filepath.Join(packageRoot, "plugin.json")); err == nil && info.Mode().IsRegular() {
		return packageRoot, nil
	}
	entries, err := os.ReadDir(packageRoot)
	if err != nil {
		return "", err
	}
	var directory string
	for _, entry := range entries {
		if !entry.IsDir() || directory != "" {
			return "", errors.New("plugin ZIP must contain plugin.json at its root or inside one top-level directory")
		}
		directory = filepath.Join(packageRoot, entry.Name())
	}
	if directory == "" {
		return "", errors.New("plugin ZIP does not contain plugin.json")
	}
	if info, err := os.Stat(filepath.Join(directory, "plugin.json")); err != nil || !info.Mode().IsRegular() {
		return "", errors.New("plugin ZIP does not contain plugin.json")
	}
	return directory, nil
}

func (m *PluginManager) Uninstall(id string) error {
	m.installMu.Lock()
	defer m.installMu.Unlock()
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || filepath.VolumeName(id) != "" {
		return errors.New("invalid plugin id")
	}
	m.mu.RLock()
	status, exists := m.statuses[id]
	m.mu.RUnlock()
	if !exists {
		return errors.New("plugin not found")
	}
	if status.Builtin {
		return errors.New("built-in plugins cannot be uninstalled")
	}
	target := filepath.Join(m.pluginDir, id)
	trash := filepath.Join(filepath.Dir(m.pluginDir), fmt.Sprintf(".plugin-uninstall-%s-%d", id, time.Now().UnixNano()))
	if err := os.Rename(target, trash); err != nil {
		return fmt.Errorf("uninstall plugin: %w", err)
	}
	m.mu.Lock()
	oldRemoved := m.removed[id]
	oldSource, hadSource := m.sources[id]
	oldOverride, hadOverride := m.overrides[id]
	oldSettings, hadSettings := m.settings[id]
	if isBundledPluginID(id) {
		m.removed[id] = true
	}
	delete(m.sources, id)
	delete(m.overrides, id)
	delete(m.settings, id)
	m.mu.Unlock()
	rollback := func() {
		m.mu.Lock()
		if oldRemoved {
			m.removed[id] = true
		} else {
			delete(m.removed, id)
		}
		if hadSource {
			m.sources[id] = oldSource
		} else {
			delete(m.sources, id)
		}
		if hadOverride {
			m.overrides[id] = oldOverride
		} else {
			delete(m.overrides, id)
		}
		if hadSettings {
			m.settings[id] = oldSettings
		} else {
			delete(m.settings, id)
		}
		m.mu.Unlock()
		_ = m.saveRemoved()
		_ = m.saveSources()
		_ = m.saveState()
		_ = m.saveSettings()
		_ = os.Rename(trash, target)
	}
	if err := m.saveRemoved(); err != nil {
		rollback()
		return err
	}
	if err := m.saveSources(); err != nil {
		rollback()
		return err
	}
	if err := m.saveState(); err != nil {
		rollback()
		return err
	}
	if err := m.saveSettings(); err != nil {
		rollback()
		return err
	}
	if err := m.Reload(); err != nil {
		rollback()
		return err
	}
	if err := os.RemoveAll(trash); err != nil {
		m.logger.Esg(err, "remove uninstalled plugin files")
	}
	if err := os.RemoveAll(filepath.Join(m.pluginBackupRoot(), id)); err != nil {
		m.logger.Esg(err, "remove plugin rollback files")
	}
	return nil
}
