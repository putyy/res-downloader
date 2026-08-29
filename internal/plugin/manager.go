package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin/native"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultPluginBodyLimit   int64 = 4 * 1024 * 1024
	maxPluginResources             = 2000
	maxSyntheticResponseSize       = 1024 * 1024
)

type managedPlugin struct {
	runtime     shared.RuntimePlugin
	path        string
	builtin     bool
	pageScripts []loadedPageScript
}

type ResourceSink interface {
	FilterSelectedCandidates([]shared.ResourceCandidate) []shared.ResourceCandidate
	PublishCandidates([]shared.ResourceCandidate)
	CandidateByGroup(string, string) (shared.ResourceCandidate, bool)
	RegisterTypes([]string)
}

type PluginManager struct {
	resources     ResourceSink
	captures      PageCaptureStore
	pageDownload  func(shared.ResourceCandidate) error
	logger        *Logger
	config        NetworkSettingsProvider
	media         *mediaEngine
	correlations  *pluginCorrelationStore
	pages         *pageBridgeHub
	mu            sync.RWMutex
	reloadMu      sync.Mutex
	installMu     sync.Mutex
	plugins       []managedPlugin
	statuses      map[string]shared.PluginStatus
	overrides     map[string]bool
	settings      map[string]map[string]interface{}
	removed       map[string]bool
	sources       map[string]string
	runtimeStates map[string]*pluginRuntimeState
	pluginDir     string
	stateFile     string
	settingsFile  string
	removedFile   string
	sourcesFile   string
	backupDir     string
}

func (m *PluginManager) SetCaptureStore(captures PageCaptureStore) {
	if m != nil {
		m.captures = captures
	}
}

func (m *PluginManager) SetPageDownloadHandler(handler func(shared.ResourceCandidate) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.pageDownload = handler
	m.mu.Unlock()
}

func NewManager(userDir string, config NetworkSettingsProvider, media *mediaEngine, resources ResourceSink, logger *Logger) *PluginManager {
	directory := filepath.Join(userDir, "plugins")
	manager := &PluginManager{
		resources:     resources,
		logger:        logger,
		config:        config,
		media:         media,
		correlations:  newPluginCorrelationStore(),
		pages:         newPageBridgeHub(logger),
		statuses:      make(map[string]shared.PluginStatus),
		overrides:     make(map[string]bool),
		settings:      make(map[string]map[string]interface{}),
		removed:       make(map[string]bool),
		sources:       make(map[string]string),
		runtimeStates: make(map[string]*pluginRuntimeState),
		pluginDir:     directory,
		stateFile:     filepath.Join(userDir, "plugin-state.json"),
		settingsFile:  filepath.Join(userDir, "plugin-settings.json"),
		removedFile:   filepath.Join(userDir, "plugin-removed.json"),
		sourcesFile:   filepath.Join(userDir, "plugin-sources.json"),
		backupDir:     filepath.Join(userDir, "plugin-backups"),
	}
	_ = os.MkdirAll(directory, 0750)
	_ = os.MkdirAll(manager.backupDir, 0750)
	manager.loadState()
	manager.loadSettings()
	manager.loadRemoved()
	manager.loadSources()
	manager.registerBuiltin(&native.DefaultPlugin{})
	manager.registerCaptureRuleTypes(manager.pluginSettings("builtin.generic-detector"))
	replacedSources, err := installBundledPluginsForManager(directory, manager.removed, manager.sources)
	if err != nil {
		logger.Esg(err, "install bundled plugins")
	}
	for _, id := range replacedSources {
		delete(manager.sources, id)
	}
	if len(replacedSources) > 0 {
		_ = manager.saveSources()
	}
	if err := manager.Reload(); err != nil {
		logger.Esg(err, "load external plugins")
	}
	return manager
}

func (m *PluginManager) networkSettings() NetworkSettings {
	if m == nil || m.config == nil {
		return NetworkSettings{}
	}
	return m.config()
}

func (m *PluginManager) loadExternalPlugin(directory string) (shared.RuntimePlugin, string, error) {
	return LoadExternalPlugin(directory, pluginRuntimeServices{logger: m.logger, correlations: m.correlations, pages: m.pages})
}

func (m *PluginManager) loadOfficialPlugin(directory string) (shared.RuntimePlugin, string, error) {
	return LoadOfficialPlugin(directory, pluginRuntimeServices{logger: m.logger, correlations: m.correlations, pages: m.pages})
}

func (m *PluginManager) loadInstalledPlugin(directory, source string) (shared.RuntimePlugin, string, error) {
	services := pluginRuntimeServices{logger: m.logger, correlations: m.correlations, pages: m.pages}
	if source == shared.PluginSourceOfficial {
		return LoadOfficialPlugin(directory, services)
	}
	if isBundledPluginID(filepath.Base(directory)) {
		return LoadBundledPlugin(directory, services)
	}
	return LoadExternalPlugin(directory, services)
}

func (m *PluginManager) registerBuiltin(runtime shared.RuntimePlugin) {
	manifest := runtime.Manifest()
	m.plugins = append(m.plugins, managedPlugin{runtime: runtime, builtin: true})
	m.statuses[manifest.ID] = shared.PluginStatus{
		Manifest: manifest, Source: shared.PluginSourceBuiltin, Loaded: true, Builtin: true, Reloaded: time.Now().Unix(), Digest: "builtin:" + manifest.Version,
	}
}

func (m *PluginManager) Reload() error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return err
	}

	m.mu.RLock()
	overrides := make(map[string]bool, len(m.overrides))
	for id, enabled := range m.overrides {
		overrides[id] = enabled
	}
	sources := make(map[string]string, len(m.sources))
	for id, source := range m.sources {
		sources[id] = source
	}
	m.mu.RUnlock()

	loaded := make([]managedPlugin, 0)
	statuses := make(map[string]shared.PluginStatus)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(m.pluginDir, entry.Name())
		source := sources[entry.Name()]
		runtime, manifestPath, loadErr := m.loadInstalledPlugin(directory, source)
		if loadErr == nil && runtime.Manifest().ID != entry.Name() {
			loadErr = fmt.Errorf("directory name %q must match plugin id %q", entry.Name(), runtime.Manifest().ID)
		}
		if loadErr != nil {
			id := entry.Name()
			invalidSource := sources[id]
			if isBundledPluginID(id) && invalidSource != shared.PluginSourceOfficial {
				invalidSource = shared.PluginSourceBuiltin
			} else if invalidSource == "" {
				invalidSource = shared.PluginSourceCommunity
			}
			statuses[id] = shared.PluginStatus{
				Manifest: shared.PluginManifest{ID: id, Name: id, Runtime: "invalid"},
				Path:     directory, Source: invalidSource, Error: loadErr.Error(), Loaded: false,
			}
			continue
		}
		manifest := runtime.Manifest()
		if enabled, ok := overrides[manifest.ID]; ok {
			manifest.Enabled = &enabled
			runtime = &manifestOverridePlugin{RuntimePlugin: runtime, manifest: manifest}
		}
		bundled := isBundledPluginID(manifest.ID) && source != shared.PluginSourceOfficial
		if bundled {
			source = shared.PluginSourceBuiltin
		} else if source == "" {
			source = shared.PluginSourceCommunity
		}
		status := shared.PluginStatus{
			Manifest: manifest, Path: manifestPath, Source: source, Loaded: manifest.IsEnabled(), Bundled: bundled, Reloaded: time.Now().Unix(),
		}
		pageScripts, pageScriptErr := loadManagedPageScripts(directory, manifest)
		if pageScriptErr != nil {
			status.Error = pageScriptErr.Error()
			status.Loaded = false
			statuses[manifest.ID] = status
			continue
		}
		if _, err := os.Stat(filepath.Join(m.pluginBackupRoot(), manifest.ID)); err == nil {
			status.RollbackAvailable = true
		}
		if digest, digestErr := hashPluginDirectory(directory); digestErr == nil {
			status.Digest = digest
		} else {
			status.Error = digestErr.Error()
			status.Loaded = false
		}
		statuses[manifest.ID] = status
		if manifest.IsEnabled() {
			loaded = append(loaded, managedPlugin{runtime: runtime, path: manifestPath, pageScripts: pageScripts})
		}
	}

	m.mu.Lock()
	builtins := make([]managedPlugin, 0)
	for _, item := range m.plugins {
		if item.builtin {
			builtins = append(builtins, item)
		}
	}
	for id, status := range m.statuses {
		if status.Builtin {
			statuses[id] = status
		}
	}
	m.plugins = append(builtins, loaded...)
	sort.SliceStable(m.plugins, func(i, j int) bool {
		return m.plugins[i].runtime.Manifest().Priority > m.plugins[j].runtime.Manifest().Priority
	})
	m.statuses = statuses
	m.mu.Unlock()
	m.pages.closeAll()

	for _, status := range statuses {
		m.resources.RegisterTypes(resourceTypeNames(status.Manifest.ResourceKinds))
	}
	return nil
}

func (m *PluginManager) pluginBackupRoot() string {
	if m.backupDir != "" {
		return m.backupDir
	}
	return filepath.Join(filepath.Dir(m.pluginDir), "plugin-backups")
}

func (m *PluginManager) Statuses() []shared.PluginStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	statuses := make([]shared.PluginStatus, 0, len(m.statuses))
	for id, status := range m.statuses {
		if id == "builtin.generic-detector" {
			settings := effectivePluginSettings(status.Manifest.SettingsSchema, m.settings[id])
			status.Manifest.ResourceKinds = mergeResourceKindDefinitions(
				status.Manifest.ResourceKinds, captureRuleKindDefinitions(settings["rules"]),
			)
		}
		if state := m.runtimeStates[id]; state != nil {
			status.Health = state.snapshot()
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		leftRank := pluginStatusSortRank(statuses[i])
		rightRank := pluginStatusSortRank(statuses[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return statuses[i].Manifest.ID < statuses[j].Manifest.ID
	})
	return statuses
}

func pluginStatusSortRank(status shared.PluginStatus) int {
	if status.Source == shared.PluginSourceBuiltin {
		return 0
	}
	if status.Source == shared.PluginSourceOfficial {
		return 1
	}
	return 2
}

func (m *PluginManager) Status(id string) (shared.PluginStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, exists := m.statuses[id]
	return status, exists
}

func (m *PluginManager) SetEnabled(id string, enabled bool) error {
	m.mu.Lock()
	status, ok := m.statuses[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("plugin not found")
	}
	if status.Builtin {
		m.mu.Unlock()
		return errors.New("built-in plugins cannot be disabled")
	}
	m.overrides[id] = enabled
	m.mu.Unlock()
	if err := m.saveState(); err != nil {
		return err
	}
	return m.Reload()
}

func (m *PluginManager) Settings() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]map[string]interface{}, len(m.statuses))
	for id, status := range m.statuses {
		out[id] = effectivePluginSettings(status.Manifest.SettingsSchema, m.settings[id])
	}
	return out
}

func (m *PluginManager) SetSettings(id string, settings map[string]interface{}) error {
	m.mu.Lock()
	status, ok := m.statuses[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("plugin not found")
	}
	if err := validatePluginSettings(status.Manifest.SettingsSchema, settings); err != nil {
		m.mu.Unlock()
		return err
	}
	m.settings[id] = effectivePluginSettings(status.Manifest.SettingsSchema, settings)
	settings = m.settings[id]
	m.mu.Unlock()
	if id == "builtin.generic-detector" {
		m.registerCaptureRuleTypes(settings)
	}
	return m.saveSettings()
}

func (m *PluginManager) Validate(id string) error {
	if id == "" || filepath.Base(id) != id {
		return errors.New("invalid plugin id")
	}
	m.mu.RLock()
	source := m.sources[id]
	m.mu.RUnlock()
	var runtime shared.RuntimePlugin
	var err error
	if source == shared.PluginSourceOfficial {
		runtime, _, err = m.loadOfficialPlugin(filepath.Join(m.pluginDir, id))
	} else {
		runtime, _, err = m.loadExternalPlugin(filepath.Join(m.pluginDir, id))
	}
	if err != nil {
		return err
	}
	if runtime.Manifest().ID != id {
		return fmt.Errorf("directory name %q must match plugin id %q", id, runtime.Manifest().ID)
	}
	return nil
}

func resourceKindLeaf(kind string) string {
	if index := strings.LastIndex(kind, "."); index >= 0 && index+1 < len(kind) {
		return kind[index+1:]
	}
	return kind
}

func mergeResourceCandidate(current, update shared.ResourceCandidate) shared.ResourceCandidate {
	if update.ParentGroupKey != "" {
		current.ParentGroupKey = update.ParentGroupKey
	}
	if update.ParentID != "" {
		current.ParentID = update.ParentID
	}
	if update.Kind != "" {
		current.Kind = update.Kind
	}
	if update.PrimaryType != "" {
		current.PrimaryType = update.PrimaryType
	}
	current.Traits = appendUniqueStrings(current.Traits, update.Traits...)
	if update.Technical != (shared.ResourceTechnical{}) {
		current.Technical = update.Technical
	}
	if update.Lifecycle.ExpiresAt > 0 {
		current.Lifecycle.ExpiresAt = update.Lifecycle.ExpiresAt
	}
	if update.Title != "" {
		current.Title = update.Title
	}
	if update.CoverURL != "" {
		current.CoverURL = update.CoverURL
	}
	if len(update.RequiredTracks) > 0 {
		current.RequiredTracks = appendUniqueStrings(current.RequiredTracks, update.RequiredTracks...)
	}
	if len(update.Capabilities) > 0 {
		current.Capabilities = appendUniqueStrings(current.Capabilities, update.Capabilities...)
	}
	if update.Preview != nil {
		current.Preview = update.Preview
	}
	if len(update.Actions) > 0 {
		current.Actions = update.Actions
	}
	if current.Metadata == nil {
		current.Metadata = map[string]interface{}{}
	}
	for key, value := range update.Metadata {
		current.Metadata[key] = value
	}

	trackIndexes := make(map[string]int, len(current.Tracks))
	for index, track := range current.Tracks {
		trackIndexes[track.ID] = index
	}
	for _, track := range update.Tracks {
		if index, exists := trackIndexes[track.ID]; exists {
			current.Tracks[index] = mergeResourceTrack(current.Tracks[index], track)
			continue
		}
		trackIndexes[track.ID] = len(current.Tracks)
		current.Tracks = append(current.Tracks, track)
	}
	shared.NormalizeCandidateState(&current)
	return current
}

func MergeResourceCandidate(current, update shared.ResourceCandidate) shared.ResourceCandidate {
	return mergeResourceCandidate(current, update)
}

func mergeResourceTrack(current, update shared.ResourceTrack) shared.ResourceTrack {
	if update.Role != "" {
		current.Role = update.Role
	}
	if update.Executor != "" {
		current.Executor = update.Executor
	}
	if update.URL != "" {
		current.URL = update.URL
	}
	if update.CaptureKey != "" {
		current.CaptureKey = update.CaptureKey
	}
	if update.MIME != "" {
		current.MIME = update.MIME
	}
	if update.Extension != "" {
		current.Extension = update.Extension
	}
	if update.Size > 0 {
		current.Size = update.Size
	}
	if update.Quality != "" {
		current.Quality = update.Quality
	}
	if update.Width > 0 {
		current.Width = update.Width
	}
	if update.Height > 0 {
		current.Height = update.Height
	}
	if update.Bitrate > 0 {
		current.Bitrate = update.Bitrate
	}
	if update.Codecs != "" {
		current.Codecs = update.Codecs
	}
	if len(update.Headers) > 0 {
		current.Headers = update.Headers
	}
	if len(update.Processors) > 0 {
		current.Processors = update.Processors
	}
	return current
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range additions {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func stringSliceValue(value interface{}) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func resourceTypeNames(kinds []shared.ResourceKindDefinition) []string {
	out := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		if kind.ID != "" {
			out = append(out, kind.ID)
		}
	}
	return out
}

func (m *PluginManager) registerCaptureRuleTypes(settings map[string]interface{}) {
	if m.resources == nil {
		return
	}
	m.resources.RegisterTypes(resourceTypeNames(captureRuleKindDefinitions(settings["rules"])))
}

func captureRuleKindDefinitions(value interface{}) []shared.ResourceKindDefinition {
	rules, err := native.DecodeCaptureRules(value)
	if err != nil {
		return nil
	}
	definitions := make([]shared.ResourceKindDefinition, 0)
	seen := make(map[string]struct{})
	for _, rule := range rules {
		kind := rule.Resource.Kind
		if kind == "" {
			continue
		}
		if _, exists := seen[kind]; exists {
			continue
		}
		seen[kind] = struct{}{}
		name := rule.Name
		if name == "" {
			name = kind
		}
		definitions = append(definitions, shared.ResourceKindDefinition{
			ID: kind,
			Locales: map[string]shared.PluginLocale{
				"zh": {Name: name}, "en": {Name: name},
			},
		})
	}
	return definitions
}

func mergeResourceKindDefinitions(groups ...[]shared.ResourceKindDefinition) []shared.ResourceKindDefinition {
	merged := make([]shared.ResourceKindDefinition, 0)
	indexes := make(map[string]int)
	for _, definitions := range groups {
		for _, definition := range definitions {
			if definition.ID == "" {
				continue
			}
			if index, exists := indexes[definition.ID]; exists {
				if len(merged[index].Locales) == 0 && len(definition.Locales) > 0 {
					merged[index] = definition
				}
				continue
			}
			indexes[definition.ID] = len(merged)
			merged = append(merged, definition)
		}
	}
	return merged
}

func sanitizeObservation(obs shared.Observation, permissions shared.PluginPermissions) shared.Observation {
	copy := obs
	copy.Request.Headers = cloneSharedHeaders(obs.Request.Headers)
	if !permissions.Has("read-request-body") {
		copy.Request.Body = ""
		copy.Request.Truncated = false
	}
	if obs.Response != nil {
		response := *obs.Response
		response.Headers = cloneSharedHeaders(obs.Response.Headers)
		if !permissions.Has("read-response-body") {
			response.Body = ""
			response.Truncated = false
		}
		copy.Response = &response
	}
	return copy
}

func cloneSharedHeaders(headers shared.HeaderMap) shared.HeaderMap {
	out := make(shared.HeaderMap, len(headers))
	for key, values := range headers {
		out[key] = append([]string(nil), values...)
	}
	return out
}
