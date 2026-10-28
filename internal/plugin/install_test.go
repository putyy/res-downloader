package plugin

import (
	"archive/zip"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
	"testing"
)

func TestInstallAndUninstallPluginArchive(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	archive := pluginTestArchive(t, map[string]string{
		"sample/plugin.json": `{
  "id":"test.online","name":"Online Test","author":{"name":"Test Author","url":"https://example.com"},
  "version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js",
  "permissions":{"domains":["example.com"],"capabilities":["observe-response"]},"match":[]
}`,
		"sample/main.js": "function onObservation() { return {decision: 'continue'} }",
	})
	manifest, err := manager.InstallArchive(archive)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "test.online" || manifest.Author.Name != "Test Author" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := os.Stat(filepath.Join(manager.pluginDir, manifest.ID, "plugin.json")); err != nil {
		t.Fatalf("installed manifest: %v", err)
	}
	if err := manager.Uninstall(manifest.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(manager.pluginDir, manifest.ID)); !os.IsNotExist(err) {
		t.Fatalf("plugin directory still exists: %v", err)
	}
	for _, status := range manager.Statuses() {
		if status.Manifest.ID == manifest.ID {
			t.Fatal("uninstalled plugin remains in status list")
		}
	}
}

func TestInstallLocalOfficialPluginArchive(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	archive := pluginTestArchive(t, map[string]string{
		"plugin.json": `{
  "id":"official.bilibili","name":"Bilibili","author":{"name":"putyy","url":"https://github.com/putyy/resd-plugin-bilibili"},
  "version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js",
  "permissions":{"domains":["api.bilibili.com"],"capabilities":["observe-response"]},"match":[]
}`,
		"main.js": "function onObservation() { return {decision: 'continue'} }",
	})
	manifest, digest, err := InspectLocalPluginArchiveDetails(archive)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := manager.InstallLocalArchiveApproved(archive, false, manifest.ID, manifest.Version, digest, false)
	if err != nil {
		t.Fatal(err)
	}
	status, exists := manager.Status(installed.ID)
	if !exists || status.Source != shared.PluginSourceOfficial {
		t.Fatalf("local official plugin status = %#v", status)
	}
}

func TestLocalPluginCannotImpersonateOfficialPublisher(t *testing.T) {
	archive := pluginTestArchive(t, map[string]string{
		"plugin.json": `{
  "id":"official.fake","name":"Fake Official","author":{"name":"Someone","url":"https://github.com/someone/resd-plugin-fake"},
  "version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js",
  "permissions":{"domains":["example.com"],"capabilities":[]},"match":[]
}`,
		"main.js": "function onObservation() {}",
	})
	_, _, err := InspectLocalPluginArchiveDetails(archive)
	if err == nil || !strings.Contains(err.Error(), "github.com/putyy") {
		t.Fatalf("expected official publisher error, got %v", err)
	}
}

func TestInstallPluginArchiveRejectsTraversal(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	archive := pluginTestArchive(t, map[string]string{"../escaped": "nope"})
	_, err := manager.InstallArchive(archive)
	if err == nil || !strings.Contains(err.Error(), "escapes the package") {
		t.Fatalf("expected traversal error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(manager.pluginDir), "escaped")); !os.IsNotExist(err) {
		t.Fatalf("archive wrote outside staging directory: %v", err)
	}
}

func TestInstallPluginArchiveIgnoresMacOSMetadata(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	archive := pluginTestArchive(t, map[string]string{
		"youtube/plugin.json": `{
  "id":"test.macos-archive","name":"macOS Archive Test","version":"1.0.0","apiVersion":1,
  "runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":[]},"match":[]
}`,
		"youtube/main.js":                         "function onObservation() {}",
		"youtube/.DS_Store":                       "finder metadata",
		"youtube/._main.js":                       "appledouble metadata",
		"__MACOSX/._youtube":                      "appledouble directory metadata",
		"__MACOSX/youtube/._plugin.json":          "appledouble file metadata",
		"__MACOSX/youtube/fixtures/._sample.json": "appledouble nested metadata",
	})

	manifest, err := manager.InstallArchive(archive)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "test.macos-archive" {
		t.Fatalf("manifest id = %q", manifest.ID)
	}
	installed := filepath.Join(manager.pluginDir, manifest.ID)
	for _, relative := range []string{".DS_Store", "._main.js", "__MACOSX"} {
		if _, err := os.Stat(filepath.Join(installed, relative)); !os.IsNotExist(err) {
			t.Fatalf("macOS metadata %q was installed: %v", relative, err)
		}
	}
}

func TestPluginContentChecksumIgnoresGeneratedWrapperDirectory(t *testing.T) {
	files := map[string]string{
		"plugin.json": `{"id":"test.digest","name":"Digest","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":[]},"match":[]}`,
		"main.js":     "function onObservation() {}",
	}
	rootArchive := pluginTestArchive(t, files)
	wrapperArchive := pluginTestArchive(t, map[string]string{
		"repository-generated/plugin.json": files["plugin.json"],
		"repository-generated/main.js":     files["main.js"],
	})
	_, first, err := InspectPluginArchiveDetails(rootArchive)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := InspectPluginArchiveDetails(wrapperArchive)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("content digests differ: %q, %q", first, second)
	}
	manager := newInstallTestPluginManager(t)
	if _, err := manager.installArchive(rootArchive, false, "test.digest", "1.0.0", strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected content checksum mismatch, got %v", err)
	}
}

func TestUpdatePluginArchivePreservesSettings(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	archiveForVersion := func(version string) []byte {
		return pluginTestArchive(t, map[string]string{
			"plugin.json": `{
  "id":"test.update","name":"Update Test","version":"` + version + `","apiVersion":1,
  "runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":[]},
  "match":[],"settingsSchema":{"type":"object","properties":{"quality":{"type":"string","default":"auto"}}}
}`,
			"main.js": "function onObservation() { return {decision: 'continue'} }",
		})
	}
	if _, err := manager.InstallArchive(archiveForVersion("1.0.0")); err != nil {
		t.Fatal(err)
	}
	manager.settings["test.update"] = map[string]interface{}{"quality": "high"}
	manifest, err := manager.installArchive(archiveForVersion("1.1.0"), true, "test.update", "1.1.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.1.0" {
		t.Fatalf("version = %q", manifest.Version)
	}
	if manager.settings["test.update"]["quality"] != "high" {
		t.Fatal("plugin settings were not preserved")
	}
	statuses := manager.Statuses()
	if len(statuses) != 1 || statuses[0].Manifest.Version != "1.1.0" {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}

func TestPluginDownloadURLRejectsInsecureAndLocalHosts(t *testing.T) {
	for _, rawURL := range []string{
		"http://example.com/plugin.zip",
		"https://127.0.0.1/plugin.zip",
		"https://localhost/plugin.zip",
		"https://user:password@example.com/plugin.zip",
	} {
		if err := validatePluginDownloadURL(rawURL); err == nil {
			t.Fatalf("expected URL %q to be rejected", rawURL)
		}
	}
	if err := validatePluginDownloadURL("https://example.com/plugin.zip"); err != nil {
		t.Fatalf("expected public HTTPS URL to be accepted: %v", err)
	}
}

func TestPluginHTTPClientReusesEnabledDownloadProxy(t *testing.T) {
	config := NetworkSettings{DownloadProxy: true, UpstreamProxy: "http://127.0.0.1:7890", Port: "8899"}
	client := newPluginHTTPClient(config)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("plugin HTTP client did not configure the download proxy")
	}
	request, _ := http.NewRequest(http.MethodGet, "https://github.com/owner/repo", nil)
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL.String() != "http://127.0.0.1:7890" {
		t.Fatalf("proxy URL = %q", proxyURL)
	}
}

func TestUninstalledBundledPluginIsNotRestored(t *testing.T) {
	manager := newInstallTestPluginManager(t)
	if err := installBundledPlugins(manager.pluginDir); err != nil {
		t.Fatal(err)
	}
	if err := manager.Reload(); err != nil {
		t.Fatal(err)
	}
	statusFound := false
	for _, status := range manager.Statuses() {
		if status.Manifest.ID == "official.wechat" {
			statusFound = status.Bundled
		}
	}
	if !statusFound {
		t.Fatal("bundled plugin was not identified")
	}
	if err := manager.Uninstall("official.wechat"); err != nil {
		t.Fatal(err)
	}
	if !manager.removed["official.wechat"] {
		t.Fatal("bundled plugin uninstall was not persisted")
	}
	if err := installBundledPluginsExcept(manager.pluginDir, manager.removed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(manager.pluginDir, "official.wechat")); !os.IsNotExist(err) {
		t.Fatalf("removed bundled plugin was restored: %v", err)
	}
}

func TestOfficialStoreVersionOverridesBundledAndUninstallRemovesLogicalPlugin(t *testing.T) {
	userDirectory := t.TempDir()
	manager := NewManager(userDirectory, func() NetworkSettings { return NetworkSettings{} }, nil, testResourceSink{}, NewLogger(false, ""))
	archive := pluginTestArchive(t, map[string]string{
		"plugin.json": `{"id":"official.wechat","name":"WeChat Store","version":"2.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["*.weixin.qq.com"]}}`,
		"main.js":     `function onObservation() { return {decision: "continue"} }`,
	})
	manifest, err := manager.installArchiveApprovedForSource(
		archive, true, "official.wechat", "2.0.0", "", true, shared.PluginSourceOfficial,
	)
	if err != nil {
		t.Fatal(err)
	}
	status, exists := manager.Status("official.wechat")
	if !exists || manifest.Version != "2.0.0" || status.Source != shared.PluginSourceOfficial || status.Bundled {
		t.Fatalf("unexpected official override status: %#v", status)
	}
	if err := manager.Uninstall("official.wechat"); err != nil {
		t.Fatal(err)
	}
	if _, exists := manager.Status("official.wechat"); exists {
		t.Fatal("uninstalled logical plugin is still active")
	}
	restarted := NewManager(userDirectory, func() NetworkSettings { return NetworkSettings{} }, nil, testResourceSink{}, NewLogger(false, ""))
	if _, exists := restarted.Status("official.wechat"); exists {
		t.Fatal("bundled plugin was restored after logical uninstall")
	}
}

func newInstallTestPluginManager(t *testing.T) *PluginManager {
	t.Helper()
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins")
	if err := os.Mkdir(pluginDir, 0750); err != nil {
		t.Fatal(err)
	}
	return &PluginManager{
		resources:    testResourceSink{},
		config:       func() NetworkSettings { return NetworkSettings{} },
		logger:       NewLogger(false, ""),
		statuses:     make(map[string]shared.PluginStatus),
		overrides:    make(map[string]bool),
		settings:     make(map[string]map[string]interface{}),
		removed:      make(map[string]bool),
		sources:      make(map[string]string),
		pluginDir:    pluginDir,
		stateFile:    filepath.Join(root, "plugin-state.json"),
		settingsFile: filepath.Join(root, "plugin-settings.json"),
		removedFile:  filepath.Join(root, "plugin-removed.json"),
		sourcesFile:  filepath.Join(root, "plugin-sources.json"),
	}
}

func pluginTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
