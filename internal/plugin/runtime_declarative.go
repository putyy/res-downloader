package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	shared "res-downloader/internal/model"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type pluginRuntimeServices struct {
	logger       *Logger
	correlations *pluginCorrelationStore
	pages        *pageBridgeHub
}

func LoadExternalPlugin(directory string, configured ...pluginRuntimeServices) (shared.RuntimePlugin, string, error) {
	return loadPluginDirectory(directory, false, false, configured...)
}

func LoadBundledPlugin(directory string, configured ...pluginRuntimeServices) (shared.RuntimePlugin, string, error) {
	return loadPluginDirectory(directory, true, true, configured...)
}

func LoadOfficialPlugin(directory string, configured ...pluginRuntimeServices) (shared.RuntimePlugin, string, error) {
	return loadPluginDirectory(directory, false, true, configured...)
}

func loadPluginDirectory(directory string, trustedBundled, allowOfficial bool, configured ...pluginRuntimeServices) (shared.RuntimePlugin, string, error) {
	services := pluginRuntimeServices{correlations: newPluginCorrelationStore()}
	if len(configured) > 0 {
		services = configured[0]
		if services.correlations == nil {
			services.correlations = newPluginCorrelationStore()
		}
	}
	manifestPath := ""
	for _, name := range []string{"plugin.json", "plugin.yaml", "plugin.yml"} {
		candidate := filepath.Join(directory, name)
		if _, err := os.Stat(candidate); err == nil {
			manifestPath = candidate
			break
		}
	}
	if manifestPath == "" {
		return nil, "", errors.New("plugin manifest not found")
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, manifestPath, err
	}
	var manifest shared.PluginManifest
	if strings.HasSuffix(manifestPath, ".json") {
		err = json.Unmarshal(raw, &manifest)
	} else {
		err = yaml.Unmarshal(raw, &manifest)
	}
	if err != nil {
		return nil, manifestPath, fmt.Errorf("parse manifest: %w", err)
	}
	if trustedBundled && !isTrustedBundledPluginDirectory(manifest.ID, directory) {
		return nil, manifestPath, errors.New("bundled plugin content does not match the application image")
	}
	if err := validateManifestForSource(manifest, allowOfficial); err != nil {
		return nil, manifestPath, err
	}
	if err := validateProcessorFiles(directory, manifest); err != nil {
		return nil, manifestPath, err
	}
	if err := validatePageScriptFiles(directory, manifest); err != nil {
		return nil, manifestPath, err
	}
	switch manifest.Runtime {
	case "declarative":
		return &declarativePlugin{manifest: manifest}, manifestPath, nil
	case "javascript":
		runtime, err := newJavaScriptPlugin(directory, manifest, services)
		return runtime, manifestPath, err
	default:
		return nil, manifestPath, fmt.Errorf("unsupported plugin runtime %q", manifest.Runtime)
	}
}

func validateManifest(manifest shared.PluginManifest) error {
	return validateManifestForSource(manifest, false)
}

// ParseStoreManifestJSON performs the manifest-only validation used by the
// extension index. Runtime files are validated later from the downloaded ZIP
// by the installer. Official store entries may use the reserved official.
// prefix; community entries may not.
func ParseStoreManifestJSON(raw []byte, official bool) (shared.PluginManifest, error) {
	var manifest shared.PluginManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("parse manifest: %w", err)
	}
	if err := validateManifestForSource(manifest, official); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validateManifestForSource(manifest shared.PluginManifest, trustedBundled bool) error {
	if !validIdentifier(manifest.ID) {
		return errors.New("manifest id must contain only letters, numbers, dots, dashes, or underscores")
	}
	switch reservedPluginIDPrefix(manifest.ID) {
	case "builtin.":
		return errors.New("plugin ids beginning with builtin. are reserved")
	case "official.":
		if !trustedBundled {
			return errors.New("plugin ids beginning with official. are reserved for official plugins")
		}
	}
	if manifest.Name == "" || manifest.Version == "" {
		return errors.New("manifest name and version are required")
	}
	if _, err := parseSemanticVersion(manifest.Version); err != nil {
		return err
	}
	if manifest.Requires.FFmpeg != "" && !regexp.MustCompile(`^>=\s*[0-9]+(?:\.[0-9]+){0,2}$`).MatchString(manifest.Requires.FFmpeg) {
		return errors.New("requires.ffmpeg must use a constraint such as >=6.0")
	}
	if len(manifest.Author.Name) > 100 {
		return errors.New("manifest author name is too long")
	}
	if manifest.Author.URL != "" {
		authorURL, err := url.Parse(manifest.Author.URL)
		if err != nil || authorURL.Host == "" || (authorURL.Scheme != "https" && authorURL.Scheme != "http") || len(manifest.Author.URL) > 2048 {
			return errors.New("manifest author URL must be a valid HTTP or HTTPS URL")
		}
	}
	if manifest.APIVersion != shared.PluginAPIVersion {
		return fmt.Errorf("unsupported apiVersion %d (expected %d)", manifest.APIVersion, shared.PluginAPIVersion)
	}
	if len(manifest.Permissions.Domains) == 0 {
		return errors.New("permissions.domains must not be empty")
	}
	if manifest.Permissions.BodyLimit < 0 || manifest.Permissions.BodyLimit > 32*1024*1024 {
		return errors.New("permissions.bodyLimit must be between 0 and 33554432 bytes")
	}
	allowed := map[string]bool{
		"observe-request": true, "observe-response": true, "read-request-body": true,
		"read-response-body": true, "modify-response": true, "intercept-request": true,
		"emit-resource": true, "process-download": true,
		"media.basic": true, "media.ffmpeg": true, "media.ffmpeg.network": true,
		"inject-page-script": true, "page-bridge": true,
		"capture-response-body": true, "enqueue-download": true,
	}
	for _, capability := range manifest.Permissions.Capabilities {
		if !allowed[capability] {
			return fmt.Errorf("unknown capability %q", capability)
		}
	}
	if manifest.Permissions.Has("read-request-body") && !manifest.Permissions.Has("observe-request") {
		return errors.New("read-request-body requires observe-request")
	}
	if (manifest.Permissions.Has("read-response-body") || manifest.Permissions.Has("modify-response")) &&
		!manifest.Permissions.Has("observe-response") {
		return errors.New("response body permissions require observe-response")
	}
	if manifest.Permissions.Has("capture-response-body") && !manifest.Permissions.Has("observe-response") {
		return errors.New("capture-response-body requires observe-response")
	}
	if manifest.Permissions.Has("intercept-request") && !manifest.Permissions.Has("observe-request") {
		return errors.New("intercept-request requires observe-request")
	}
	if manifest.Permissions.Has("page-bridge") && !manifest.Permissions.Has("inject-page-script") {
		return errors.New("page-bridge requires inject-page-script")
	}
	if manifest.Permissions.Has("enqueue-download") && (!manifest.Permissions.Has("page-bridge") || !manifest.Permissions.Has("emit-resource")) {
		return errors.New("enqueue-download requires page-bridge and emit-resource")
	}
	if len(manifest.PageScripts) > 0 && !manifest.Permissions.Has("inject-page-script") {
		return errors.New("pageScripts require inject-page-script")
	}
	if len(manifest.PageScripts) > 8 {
		return errors.New("manifest declares more than 8 page scripts")
	}
	seenPageScripts := make(map[string]struct{}, len(manifest.PageScripts))
	for _, script := range manifest.PageScripts {
		if !validIdentifier(script.ID) {
			return fmt.Errorf("page script id %q is invalid", script.ID)
		}
		if _, exists := seenPageScripts[script.ID]; exists {
			return fmt.Errorf("page script id %q is duplicated", script.ID)
		}
		seenPageScripts[script.ID] = struct{}{}
		if script.Entry == "" || !strings.EqualFold(filepath.Ext(script.Entry), ".js") {
			return fmt.Errorf("page script %q entry must be a .js file", script.ID)
		}
		if script.RunAt != "" && script.RunAt != "document-start" {
			return fmt.Errorf("page script %q has unsupported runAt %q", script.ID, script.RunAt)
		}
		if script.Frames != "" && script.Frames != "top" && script.Frames != "all" {
			return fmt.Errorf("page script %q has unsupported frames %q", script.ID, script.Frames)
		}
		if len(script.Match) == 0 {
			return fmt.Errorf("page script %q must declare at least one match", script.ID)
		}
		if script.Bridge && !manifest.Permissions.Has("page-bridge") {
			return fmt.Errorf("page script %q bridge requires page-bridge", script.ID)
		}
		if script.Bridge && manifest.Runtime != "javascript" {
			return fmt.Errorf("page script %q bridge requires the javascript runtime", script.ID)
		}
		for _, match := range script.Match {
			if match.Host == "" {
				return fmt.Errorf("page script %q match host is required", script.ID)
			}
			if !pageScriptDomainAllowed(manifest.Permissions.Domains, match.Host) {
				return fmt.Errorf("page script %q match host %q is outside permissions.domains", script.ID, match.Host)
			}
		}
	}
	if manifest.Runtime == "javascript" && manifest.Entry == "" {
		return errors.New("JavaScript plugin entry is required")
	}
	if len(manifest.Processors) > 0 && !manifest.Permissions.Has("process-download") {
		return errors.New("plugin processors require the process-download capability")
	}
	for id, processor := range manifest.Processors {
		if id == "" || strings.ContainsAny(id, " /\\:") {
			return fmt.Errorf("processor id %q is invalid", id)
		}
		if processor.Runtime != "wasm" {
			return fmt.Errorf("processor %q has unsupported runtime %q", id, processor.Runtime)
		}
		if processor.APIVersion != shared.PluginProcessorAPIVersion {
			return fmt.Errorf("processor %q has unsupported apiVersion %d", id, processor.APIVersion)
		}
		if processor.Entry == "" || !strings.EqualFold(filepath.Ext(processor.Entry), ".wasm") {
			return fmt.Errorf("processor %q entry must be a .wasm file", id)
		}
	}
	if len(manifest.Actions) > 32 {
		return errors.New("manifest declares more than 32 actions")
	}
	for id, action := range manifest.Actions {
		if !validIdentifier(id) {
			return fmt.Errorf("action id %q is invalid", id)
		}
		if action.Kind != shared.PluginActionProcessFile {
			return fmt.Errorf("action %q has unsupported kind %q", id, action.Kind)
		}
		if _, exists := manifest.Processors[action.Processor]; !exists {
			return fmt.Errorf("action %q references unknown processor %q", id, action.Processor)
		}
		if !validExtension(action.OutputExtension) {
			return fmt.Errorf("action %q has invalid outputExtension", id)
		}
		for _, extension := range action.InputExtensions {
			if !validExtension(extension) || extension == "" {
				return fmt.Errorf("action %q has invalid input extension %q", id, extension)
			}
		}
	}
	seenKinds := make(map[string]struct{}, len(manifest.ResourceKinds))
	for _, kind := range manifest.ResourceKinds {
		if !validIdentifier(kind.ID) {
			return fmt.Errorf("resource kind %q is invalid", kind.ID)
		}
		if _, exists := seenKinds[kind.ID]; exists {
			return fmt.Errorf("resource kind %q is duplicated", kind.ID)
		}
		seenKinds[kind.ID] = struct{}{}
		if kind.Icon != "" && !validIdentifier(kind.Icon) {
			return fmt.Errorf("resource kind %q has invalid icon %q", kind.ID, kind.Icon)
		}
		if len(kind.Color) > 32 {
			return fmt.Errorf("resource kind %q color is too long", kind.ID)
		}
	}
	return nil
}

func reservedPluginIDPrefix(id string) string {
	lowerID := strings.ToLower(id)
	for _, prefix := range []string{"builtin.", "official."} {
		if strings.HasPrefix(lowerID, prefix) {
			return prefix
		}
	}
	return ""
}

func validateProcessorFiles(directory string, manifest shared.PluginManifest) error {
	for id, processor := range manifest.Processors {
		path, err := securePluginFilePath(directory, processor.Entry)
		if err != nil {
			return fmt.Errorf("processor %q: %w", id, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("processor %q: %w", id, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("processor %q entry is not a regular file", id)
		}
		if info.Size() <= 8 || info.Size() > maxPluginWASMSize {
			return fmt.Errorf("processor %q WASM size must be between 9 and %d bytes", id, maxPluginWASMSize)
		}
		header := make([]byte, 4)
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("processor %q: %w", id, err)
		}
		_, readErr := io.ReadFull(file, header)
		closeErr := file.Close()
		if readErr != nil {
			return fmt.Errorf("processor %q: %w", id, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("processor %q: %w", id, closeErr)
		}
		if string(header) != "\x00asm" {
			return fmt.Errorf("processor %q entry is not a WebAssembly binary", id)
		}
	}
	return nil
}

const maxPluginPageScriptSize int64 = 256 * 1024

func validatePageScriptFiles(directory string, manifest shared.PluginManifest) error {
	for _, script := range manifest.PageScripts {
		path, err := securePluginFilePath(directory, script.Entry)
		if err != nil {
			return fmt.Errorf("page script %q: %w", script.ID, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("page script %q: %w", script.ID, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("page script %q entry is not a regular file", script.ID)
		}
		if info.Size() <= 0 || info.Size() > maxPluginPageScriptSize {
			return fmt.Errorf("page script %q size must be between 1 and %d bytes", script.ID, maxPluginPageScriptSize)
		}
	}
	return nil
}

func pageScriptDomainAllowed(domains []string, matchHost string) bool {
	host := strings.ToLower(strings.TrimSpace(matchHost))
	probe := strings.TrimPrefix(host, "*.")
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "*" || domain == host {
			return true
		}
		if strings.HasPrefix(domain, "*.") {
			suffix := strings.TrimPrefix(domain, "*.")
			if probe == suffix || strings.HasSuffix(probe, "."+suffix) {
				return true
			}
		}
	}
	return false
}

func securePluginFilePath(directory, entry string) (string, error) {
	if entry == "" || filepath.IsAbs(entry) {
		return "", errors.New("entry must be relative to the plugin directory")
	}
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.Clean(entry))
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if candidate == root || !strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return "", errors.New("entry must stay inside the plugin directory")
	}
	return candidate, nil
}

type declarativePlugin struct{ manifest shared.PluginManifest }

func (p *declarativePlugin) Manifest() shared.PluginManifest { return p.manifest }

func (p *declarativePlugin) Handle(_ context.Context, obs shared.Observation) (shared.PluginResult, error) {
	result := shared.PluginResult{}
	for _, extractor := range p.manifest.Extractors {
		if extractor.Stage != obs.Stage {
			continue
		}
		if extractor.Format != "json" {
			return result, fmt.Errorf("unsupported declarative format %q", extractor.Format)
		}
		body := obs.Request.Body
		if obs.Stage == shared.StageResponse && obs.Response != nil {
			body = obs.Response.Body
		}
		var document interface{}
		if err := json.Unmarshal([]byte(body), &document); err != nil {
			return result, fmt.Errorf("parse observed JSON: %w", err)
		}
		roots := selectMany(document, extractor.Root)
		for _, root := range roots {
			candidate := buildDeclarativeCandidate(root, extractor.Resource)
			if len(candidate.Tracks) > 0 && candidate.Tracks[0].URL != "" {
				result.Resources = append(result.Resources, candidate)
			}
		}
	}
	return result, nil
}

func (p *declarativePlugin) Resolve(_ context.Context, _ shared.ResourceCandidate, _ shared.DownloadOptions) (shared.DownloadPlan, bool, error) {
	return shared.DownloadPlan{}, false, nil
}

func buildDeclarativeCandidate(root interface{}, definition shared.DeclarativeResource) shared.ResourceCandidate {
	urlValue := selectorString(root, definition.URL)
	kind := selectorString(root, definition.Kind)
	role := selectorString(root, definition.Role)
	if role == "" {
		role = resourceKindLeaf(kind)
	}
	executor := selectorString(root, definition.Executor)
	if executor == "" {
		executor = "http-file"
	}
	contentType := selectorString(root, definition.ContentType)
	extension := selectorString(root, definition.Extension)
	var size int64
	if rawSize := selectorString(root, definition.Size); rawSize != "" {
		size, _ = strconv.ParseInt(rawSize, 10, 64)
	}
	candidate := shared.ResourceCandidate{
		Title: selectorString(root, definition.Title), CoverURL: selectorString(root, definition.CoverURL), Kind: kind,
		Tracks: []shared.ResourceTrack{{
			ID: "primary", Role: role, Executor: executor, URL: urlValue,
			MIME: contentType, Extension: extension, Size: size,
		}},
		RequiredTracks: []string{role},
		Capabilities: []string{
			shared.ResourceCapabilityDownload, shared.ResourceCapabilityOpen, shared.ResourceCapabilityCopy,
		},
		Metadata: map[string]interface{}{},
	}
	if renderer := selectorString(root, definition.Preview); renderer != "" {
		candidate.Capabilities = append(candidate.Capabilities, shared.ResourceCapabilityPreview)
		candidate.Preview = &shared.PreviewSpec{Renderer: renderer, Mode: "proxy", MIME: contentType, TrackID: "primary"}
	}
	for key, selector := range definition.Metadata {
		candidate.Metadata[key] = selectOne(root, selector)
	}
	return candidate
}

func selectorString(root interface{}, selector shared.Selector) string {
	value := selectOne(root, selector)
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func selectOne(root interface{}, selector shared.Selector) interface{} {
	if selector.Path != "" {
		values := selectMany(root, selector.Path)
		if len(values) > 0 {
			return values[0]
		}
		return nil
	}
	return selector.Value
}

// selectMany implements a deliberately small, deterministic JSON path subset:
// $.a.b, a.b, numeric array indexes and a trailing [*].
func selectMany(root interface{}, expression string) []interface{} {
	if expression == "" || expression == "$" {
		return []interface{}{root}
	}
	expression = strings.TrimPrefix(expression, "$.")
	segments := strings.Split(expression, ".")
	current := []interface{}{root}
	for _, segment := range segments {
		all := strings.HasSuffix(segment, "[*]")
		segment = strings.TrimSuffix(segment, "[*]")
		next := make([]interface{}, 0)
		for _, value := range current {
			if segment != "" {
				switch object := value.(type) {
				case map[string]interface{}:
					value = object[segment]
				case []interface{}:
					index, err := strconv.Atoi(segment)
					if err != nil || index < 0 || index >= len(object) {
						continue
					}
					value = object[index]
				default:
					continue
				}
			}
			if all {
				if array, ok := value.([]interface{}); ok {
					next = append(next, array...)
				}
			} else if value != nil {
				next = append(next, value)
			}
		}
		current = next
	}
	return current
}
