package model

import (
	"context"
	"encoding/json"
)

const (
	PluginAPIVersion          = 1
	PluginProcessorAPIVersion = 1
	StageRequest              = "request"
	StageResponse             = "response"

	DecisionContinue = "continue"
	DecisionStop     = "stop"
)

// PluginManifest is the stable, serialisable contract shared by native,
// declarative and JavaScript plugins.
type PluginManifest struct {
	ID             string                               `json:"id" yaml:"id"`
	Name           string                               `json:"name" yaml:"name"`
	Author         PluginAuthor                         `json:"author,omitempty" yaml:"author,omitempty"`
	Version        string                               `json:"version" yaml:"version"`
	APIVersion     int                                  `json:"apiVersion" yaml:"apiVersion"`
	Runtime        string                               `json:"runtime" yaml:"runtime"`
	Entry          string                               `json:"entry,omitempty" yaml:"entry,omitempty"`
	Priority       int                                  `json:"priority,omitempty" yaml:"priority,omitempty"`
	Enabled        *bool                                `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Permissions    PluginPermissions                    `json:"permissions" yaml:"permissions"`
	Match          []PluginMatchRule                    `json:"match" yaml:"match"`
	PageScripts    []PluginPageScript                   `json:"pageScripts,omitempty" yaml:"pageScripts,omitempty"`
	ResourceKinds  []ResourceKindDefinition             `json:"resourceKinds,omitempty" yaml:"resourceKinds,omitempty"`
	SettingsSchema map[string]interface{}               `json:"settingsSchema,omitempty" yaml:"settingsSchema,omitempty"`
	Extractors     []DeclarativeExtractor               `json:"extractors,omitempty" yaml:"extractors,omitempty"`
	Processors     map[string]PluginProcessorDefinition `json:"processors,omitempty" yaml:"processors,omitempty"`
	Actions        map[string]PluginActionDefinition    `json:"actions,omitempty" yaml:"actions,omitempty"`
	Locales        map[string]PluginLocale              `json:"locales,omitempty" yaml:"locales,omitempty"`
	Requires       PluginRequirements                   `json:"requires,omitempty" yaml:"requires,omitempty"`
}

// PluginRequirements declares optional host tools used by individual plugin
// operations. A missing tool does not prevent capture; only the dependent
// action or download plan is unavailable.
type PluginRequirements struct {
	FFmpeg string `json:"ffmpeg,omitempty" yaml:"ffmpeg,omitempty"`
}

// PluginAuthor identifies the publisher shown by the plugin manager. URL is
// opened by the host, so plugins cannot run custom UI code for this link.
type PluginAuthor struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}

// PluginLocale lets the frontend present plugin-owned text without baking
// platform plugin names and descriptions into the application translations.
// Name remains the stable fallback used by tooling and unsupported locales.
type PluginLocale struct {
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ResourceKindDefinition lets plugins introduce opaque resource kinds while
// still giving the frontend localised display metadata. The core only relies
// on ID; icons and colours are presentation hints.
type ResourceKindDefinition struct {
	ID      string                  `json:"id" yaml:"id"`
	Icon    string                  `json:"icon,omitempty" yaml:"icon,omitempty"`
	Color   string                  `json:"color,omitempty" yaml:"color,omitempty"`
	Locales map[string]PluginLocale `json:"locales,omitempty" yaml:"locales,omitempty"`
}

// CaptureRule is the generic detector's serialisable rule format. Complex
// sites should emit ResourceCandidate values from JavaScript instead.
type CaptureRule struct {
	ID       string              `json:"id"`
	Name     string              `json:"name,omitempty"`
	Enabled  bool                `json:"enabled"`
	Priority int                 `json:"priority,omitempty"`
	Match    CaptureRuleMatch    `json:"match"`
	Resource CaptureRuleResource `json:"resource"`
}

type CaptureRuleMatch struct {
	MIME               []string `json:"mime,omitempty"`
	URL                []string `json:"url,omitempty"`
	ContentDisposition []string `json:"contentDisposition,omitempty"`
	Status             []int    `json:"status,omitempty"`
	MinSize            int64    `json:"minSize,omitempty"`
	MaxSize            int64    `json:"maxSize,omitempty"`
}

type CaptureRuleResource struct {
	Kind            string   `json:"kind"`
	Role            string   `json:"role,omitempty"`
	Extension       string   `json:"extension,omitempty"`
	Executor        string   `json:"executor,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
	PreviewRenderer string   `json:"previewRenderer,omitempty"`
	PreviewMode     string   `json:"previewMode,omitempty"`
}

// PluginProcessorDefinition declares executable code bundled with an external
// plugin. The core resolves this declaration by ID; download plans never select
// arbitrary files directly.
type PluginProcessorDefinition struct {
	Runtime    string `json:"runtime" yaml:"runtime"`
	Entry      string `json:"entry" yaml:"entry"`
	APIVersion int    `json:"apiVersion" yaml:"apiVersion"`
}

const PluginActionProcessFile = "process-file"

// PluginActionDefinition describes a trusted host-rendered operation. Plugins
// provide metadata and a declared processor; they never receive filesystem
// access or inject frontend code.
type PluginActionDefinition struct {
	Kind            string                  `json:"kind" yaml:"kind"`
	Processor       string                  `json:"processor" yaml:"processor"`
	InputExtensions []string                `json:"inputExtensions,omitempty" yaml:"inputExtensions,omitempty"`
	OutputExtension string                  `json:"outputExtension,omitempty" yaml:"outputExtension,omitempty"`
	Locales         map[string]PluginLocale `json:"locales,omitempty" yaml:"locales,omitempty"`
}

func (m PluginManifest) IsEnabled() bool {
	return m.Enabled == nil || *m.Enabled
}

type PluginPermissions struct {
	Domains      []string `json:"domains" yaml:"domains"`
	Capabilities []string `json:"capabilities" yaml:"capabilities"`
	BodyLimit    int64    `json:"bodyLimit,omitempty" yaml:"bodyLimit,omitempty"`
}

func (p PluginPermissions) Has(capability string) bool {
	for _, item := range p.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

type PluginMatchRule struct {
	Stage        string   `json:"stage,omitempty" yaml:"stage,omitempty"`
	Host         string   `json:"host,omitempty" yaml:"host,omitempty"`
	Path         string   `json:"path,omitempty" yaml:"path,omitempty"`
	URL          string   `json:"url,omitempty" yaml:"url,omitempty"`
	Method       string   `json:"method,omitempty" yaml:"method,omitempty"`
	ContentTypes []string `json:"contentTypes,omitempty" yaml:"contentTypes,omitempty"`
	ReadBody     *bool    `json:"readBody,omitempty" yaml:"readBody,omitempty"`
}

// PluginPageScript declares browser-side JavaScript injected by the host into
// matching HTML documents. The source remains a plugin file and never runs in
// the Goja plugin runtime.
type PluginPageScript struct {
	ID     string                  `json:"id" yaml:"id"`
	Entry  string                  `json:"entry" yaml:"entry"`
	Match  []PluginPageScriptMatch `json:"match" yaml:"match"`
	RunAt  string                  `json:"runAt,omitempty" yaml:"runAt,omitempty"`
	Frames string                  `json:"frames,omitempty" yaml:"frames,omitempty"`
	Bridge bool                    `json:"bridge,omitempty" yaml:"bridge,omitempty"`
}

type PluginPageScriptMatch struct {
	Host string `json:"host,omitempty" yaml:"host,omitempty"`
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}

// PageScriptInjection is an internal, already-validated page script prepared
// for the proxy. Source is never exposed through the public desktop API.
type PageScriptInjection struct {
	PluginID      string `json:"-"`
	PluginVersion string `json:"-"`
	ScriptID      string `json:"-"`
	Source        string `json:"-"`
	Frames        string `json:"-"`
	Bridge        bool   `json:"-"`
	Capture       bool   `json:"-"`
	PageSessionID string `json:"-"`
	BridgeToken   string `json:"-"`
}

type PageMessageContext struct {
	PageSessionID string `json:"pageSessionId"`
	ScriptID      string `json:"scriptId"`
	PageURL       string `json:"pageUrl"`
	Origin        string `json:"origin"`
}

type PageMessageResult struct {
	OK           bool                `json:"ok"`
	Data         interface{}         `json:"data,omitempty"`
	Error        string              `json:"error,omitempty"`
	Resources    []ResourceCandidate `json:"resources,omitempty"`
	Diagnostics  []string            `json:"diagnostics,omitempty"`
	AutoDownload bool                `json:"autoDownload,omitempty"`
}

type HeaderMap map[string][]string

type RequestSnapshot struct {
	Method    string    `json:"method" yaml:"method"`
	URL       string    `json:"url" yaml:"url"`
	Host      string    `json:"host" yaml:"host"`
	Path      string    `json:"path" yaml:"path"`
	Headers   HeaderMap `json:"headers" yaml:"headers"`
	Body      string    `json:"body,omitempty" yaml:"body,omitempty"`
	Truncated bool      `json:"truncated,omitempty" yaml:"truncated,omitempty"`
}

type ResponseSnapshot struct {
	StatusCode  int       `json:"statusCode" yaml:"statusCode"`
	Headers     HeaderMap `json:"headers" yaml:"headers"`
	ContentType string    `json:"contentType" yaml:"contentType"`
	Body        string    `json:"body,omitempty" yaml:"body,omitempty"`
	Truncated   bool      `json:"truncated,omitempty" yaml:"truncated,omitempty"`
}

type Observation struct {
	Stage    string                 `json:"stage" yaml:"stage"`
	Request  RequestSnapshot        `json:"request" yaml:"request"`
	Response *ResponseSnapshot      `json:"response,omitempty" yaml:"response,omitempty"`
	Settings map[string]interface{} `json:"settings,omitempty" yaml:"settings,omitempty"`
}

type ResponsePatch struct {
	StatusCode int               `json:"statusCode,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       *string           `json:"body,omitempty"`
}

type SyntheticResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
}

// ResponseCapture asks the proxy to mirror one successful response body into
// the host-managed capture store while the original client continues reading
// it. Key is scoped to the owning plugin before it leaves the plugin manager.
type ResponseCapture struct {
	Key  string `json:"key"`
	Mode string `json:"mode,omitempty"`
}

type ResourceSource struct {
	PluginID      string `json:"pluginId"`
	PluginVersion string `json:"pluginVersion,omitempty"`
	PluginDigest  string `json:"pluginDigest,omitempty"`
	PageURL       string `json:"pageUrl,omitempty"`
	Domain        string `json:"domain,omitempty"`
}

const (
	ResourceCapabilityDownload = "download"
	ResourceCapabilityPreview  = "preview"
	ResourceCapabilityOpen     = "open"
	ResourceCapabilityCopy     = "copy"

	ResourceStatePartial = "partial"
	ResourceStateReady   = "ready"

	ResourceKindCollection = "media.collection"

	ResourceTypeVideo      = "video"
	ResourceTypeAudio      = "audio"
	ResourceTypeImage      = "image"
	ResourceTypeDocument   = "document"
	ResourceTypeArchive    = "archive"
	ResourceTypeCollection = "collection"
	ResourceTypeOther      = "other"

	ResourceTraitEncrypted     = "encrypted"
	ResourceTraitMultiTrack    = "multiTrack"
	ResourceTraitSegmented     = "segmented"
	ResourceTraitStreaming     = "streaming"
	ResourceTraitLive          = "live"
	ResourceTraitGallery       = "gallery"
	ResourceTraitDownloadable  = "downloadable"
	ResourceTraitPreviewable   = "previewable"
	ResourceTraitMergeRequired = "mergeRequired"
	ResourceTraitHasChildren   = "hasChildren"

	ResourceAvailabilityAvailable          = "available"
	ResourceAvailabilityExpired            = "expired"
	ResourceAvailabilityPluginMissing      = "pluginMissing"
	ResourceAvailabilityPluginIncompatible = "pluginIncompatible"
	ResourceAvailabilityNeedsRefresh       = "needsRefresh"
	ResourceAvailabilityUnavailable        = "unavailable"

	ResourceRefreshOK                     = "refreshed"
	ResourceRefreshUnsupported            = "refreshUnsupported"
	ResourceRefreshAuthenticationRequired = "authenticationRequired"
	ResourceRefreshRecaptureRequired      = "recaptureRequired"
)

type ResourceTechnical struct {
	MIME      string `json:"mime,omitempty"`
	Container string `json:"container,omitempty"`
	Codecs    string `json:"codecs,omitempty"`
	Duration  int64  `json:"duration,omitempty"`
}

type ResourceLifecycle struct {
	SchemaVersion     int    `json:"schemaVersion"`
	DiscoveredAt      int64  `json:"discoveredAt"`
	UpdatedAt         int64  `json:"updatedAt"`
	ExpiresAt         int64  `json:"expiresAt,omitempty"`
	LastResolvedAt    int64  `json:"lastResolvedAt,omitempty"`
	Availability      string `json:"availability"`
	UnavailableReason string `json:"unavailableReason,omitempty"`
}

// ResourceTrack is one independently acquired stream belonging to a logical
// resource. A normal file has one primary track; split media can have video,
// audio, subtitle and attachment tracks with multiple quality variants.
type ResourceTrack struct {
	ID         string            `json:"id"`
	Role       string            `json:"role"`
	Executor   string            `json:"executor,omitempty"`
	URL        string            `json:"url"`
	CaptureKey string            `json:"captureKey,omitempty"`
	MIME       string            `json:"mime,omitempty"`
	Extension  string            `json:"extension,omitempty"`
	Size       int64             `json:"size,omitempty"`
	Quality    string            `json:"quality,omitempty"`
	Width      int               `json:"width,omitempty"`
	Height     int               `json:"height,omitempty"`
	Bitrate    int64             `json:"bitrate,omitempty"`
	Codecs     string            `json:"codecs,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	// NonPersistentHeaders is matched case-insensitively when the resource is
	// copied into resources.db. The in-memory resource keeps every header so a
	// capture can still be downloaded or previewed during the current session.
	NonPersistentHeaders []string       `json:"nonPersistentHeaders,omitempty"`
	Processors           []DownloadStep `json:"processors,omitempty"`
}

// PreviewSpec selects one of the application's trusted renderers. Plugins do
// not inject frontend code; they only describe how the core-backed preview is
// transported and which MIME/codecs should be passed to the renderer.
type PreviewSpec struct {
	Renderer string `json:"renderer"`
	Mode     string `json:"mode,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Codecs   string `json:"codecs,omitempty"`
	TrackID  string `json:"trackId,omitempty"`
}

type ResourceAction struct {
	ID    string                 `json:"id"`
	Label string                 `json:"label,omitempty"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

// ResourceCandidate is emitted by plugins. Runtime download state is kept out
// of this value and is owned by the core task scheduler.
type ResourceCandidate struct {
	ID             string                 `json:"id,omitempty"`
	GroupKey       string                 `json:"groupKey,omitempty"`
	ParentGroupKey string                 `json:"parentGroupKey,omitempty"`
	ParentID       string                 `json:"parentId,omitempty"`
	DedupeKey      string                 `json:"dedupeKey,omitempty"`
	Kind           string                 `json:"kind,omitempty"`
	PrimaryType    string                 `json:"primaryType,omitempty"`
	Traits         []string               `json:"traits,omitempty"`
	Technical      ResourceTechnical      `json:"technical,omitempty"`
	Lifecycle      ResourceLifecycle      `json:"lifecycle,omitempty"`
	Title          string                 `json:"title,omitempty"`
	CoverURL       string                 `json:"coverUrl,omitempty"`
	Tracks         []ResourceTrack        `json:"tracks,omitempty"`
	RequiredTracks []string               `json:"requiredTracks,omitempty"`
	State          string                 `json:"state,omitempty"`
	Capabilities   []string               `json:"capabilities,omitempty"`
	Preview        *PreviewSpec           `json:"preview,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Actions        []ResourceAction       `json:"actions,omitempty"`
	Source         ResourceSource         `json:"source,omitempty"`
}

type ResourceRefreshResult struct {
	Resource sharedResourceCandidateAlias `json:"resource"`
	Status   string                       `json:"status,omitempty"`
	Message  string                       `json:"message,omitempty"`
}

// sharedResourceCandidateAlias avoids a recursive method-set edge in some
// JSON tooling while keeping the wire representation identical.
type sharedResourceCandidateAlias = ResourceCandidate

type PluginResult struct {
	Decision          string              `json:"decision,omitempty"`
	Handled           bool                `json:"handled,omitempty"`
	Resources         []ResourceCandidate `json:"resources,omitempty"`
	Patch             *ResponsePatch      `json:"patch,omitempty"`
	SyntheticResponse *SyntheticResponse  `json:"syntheticResponse,omitempty"`
	Captures          []ResponseCapture   `json:"captures,omitempty"`
	Diagnostics       []string            `json:"diagnostics,omitempty"`
}

type DownloadOptions struct {
	SelectedTrackIDs []string               `json:"selectedTrackIds,omitempty"`
	SavePath         string                 `json:"savePath,omitempty"`
	Settings         map[string]interface{} `json:"settings,omitempty"`
}

type DownloadStep struct {
	Type    string                 `json:"type"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type DownloadInput struct {
	ID         string                 `json:"id"`
	Executor   string                 `json:"executor"`
	URL        string                 `json:"url"`
	CaptureKey string                 `json:"captureKey,omitempty"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Extension  string                 `json:"extension,omitempty"`
	Processors []DownloadStep         `json:"processors,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

type PipelineStep struct {
	ID       string                 `json:"id"`
	Executor string                 `json:"executor"`
	Inputs   []string               `json:"inputs"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type DownloadOutput struct {
	Input      string         `json:"input"`
	Extension  string         `json:"extension,omitempty"`
	MIME       string         `json:"mime,omitempty"`
	Processors []DownloadStep `json:"processors,omitempty"`
}

type DownloadPlan struct {
	Inputs   []DownloadInput `json:"inputs,omitempty"`
	Pipeline []PipelineStep  `json:"pipeline,omitempty"`
	Output   DownloadOutput  `json:"output,omitempty"`
}

// RuntimePlugin is implemented by all plugin runtimes. It deliberately does
// not expose net/http or goproxy values.
type RuntimePlugin interface {
	Manifest() PluginManifest
	Handle(context.Context, Observation) (PluginResult, error)
	Resolve(context.Context, ResourceCandidate, DownloadOptions) (DownloadPlan, bool, error)
}

// ResourceRefresher is optional. It lets a plugin reacquire expiring URLs,
// request headers or signatures before a download plan is created.
type ResourceRefresher interface {
	RefreshResource(context.Context, ResourceCandidate, DownloadOptions) (ResourceRefreshResult, bool, error)
}

// PageMessageHandler receives JSON messages from one of the plugin's injected
// page scripts. Implementations are still subject to the normal runtime guard.
type PageMessageHandler interface {
	HandlePageMessage(context.Context, interface{}, PageMessageContext) (PageMessageResult, bool, error)
}

// BodyAwarePlugin lets native plugins avoid buffering bodies for broad domain
// match rules. External plugins should express this by using narrow match rules.
type BodyAwarePlugin interface {
	NeedsBody(Observation) bool
}

type PluginStatus struct {
	Manifest          PluginManifest      `json:"manifest"`
	Path              string              `json:"path,omitempty"`
	Source            string              `json:"source"`
	Loaded            bool                `json:"loaded"`
	Error             string              `json:"error,omitempty"`
	Builtin           bool                `json:"builtin"`
	Bundled           bool                `json:"bundled,omitempty"`
	Reloaded          int64               `json:"reloadedAt,omitempty"`
	Digest            string              `json:"digest,omitempty"`
	Health            PluginRuntimeHealth `json:"health,omitempty"`
	RollbackAvailable bool                `json:"rollbackAvailable,omitempty"`
}

type PluginRuntimeHealth struct {
	ConsecutiveErrors int    `json:"consecutiveErrors,omitempty"`
	TotalErrors       int64  `json:"totalErrors,omitempty"`
	SlowCalls         int64  `json:"slowCalls,omitempty"`
	PausedUntil       int64  `json:"pausedUntil,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	LastDurationMS    int64  `json:"lastDurationMs,omitempty"`
}

type Selector struct {
	Path  string      `json:"path,omitempty" yaml:"path,omitempty"`
	Value interface{} `json:"value,omitempty" yaml:"value,omitempty"`
}

type DeclarativeResource struct {
	URL         Selector            `json:"url" yaml:"url"`
	Title       Selector            `json:"title,omitempty" yaml:"title,omitempty"`
	CoverURL    Selector            `json:"coverUrl,omitempty" yaml:"coverUrl,omitempty"`
	Kind        Selector            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Role        Selector            `json:"role,omitempty" yaml:"role,omitempty"`
	Executor    Selector            `json:"executor,omitempty" yaml:"executor,omitempty"`
	ContentType Selector            `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Extension   Selector            `json:"extension,omitempty" yaml:"extension,omitempty"`
	Size        Selector            `json:"size,omitempty" yaml:"size,omitempty"`
	Preview     Selector            `json:"preview,omitempty" yaml:"preview,omitempty"`
	Metadata    map[string]Selector `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type DeclarativeExtractor struct {
	Stage    string              `json:"stage" yaml:"stage"`
	Format   string              `json:"format" yaml:"format"`
	Root     string              `json:"root,omitempty" yaml:"root,omitempty"`
	Resource DeclarativeResource `json:"resource" yaml:"resource"`
}

// CloneJSON provides a safe boundary for values crossing a script runtime.
func CloneJSON[T any](value T) (T, error) {
	var out T
	raw, err := json.Marshal(value)
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(raw, &out)
	return out, err
}
