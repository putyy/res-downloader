/** Global editor types for res-downloader plugin API v1. */

type JSONPrimitive = string | number | boolean | null
type JSONValue = JSONPrimitive | JSONValue[] | {[key: string]: JSONValue}
type HeaderMap = Record<string, string[]>

type PluginRuntime = 'javascript' | 'declarative'
type ObservationStage = 'request' | 'response'
type PrimaryType = 'video' | 'audio' | 'image' | 'document' | 'archive' | 'collection' | 'other'
type ResourceCapability = 'download' | 'preview' | 'open' | 'copy'
type AcquisitionExecutor = 'http-file' | 'capture-file' | 'hls' | 'ffmpeg-hls'
type PreviewRenderer = 'image' | 'audio' | 'video' | 'pdf' | 'text'
type PluginCapability =
  | 'observe-request'
  | 'read-request-body'
  | 'intercept-request'
  | 'observe-response'
  | 'read-response-body'
  | 'modify-response'
  | 'emit-resource'
  | 'process-download'
  | 'media.basic'
  | 'media.ffmpeg'
  | 'media.ffmpeg.network'
  | 'inject-page-script'
  | 'page-bridge'
  | 'capture-response-body'
  | 'enqueue-download'

interface PluginLocale {
  name?: string
  description?: string
}

interface PluginAuthor {
  name?: string
  url?: string
}

interface PluginPermissions {
  domains: string[]
  capabilities: PluginCapability[]
  bodyLimit?: number
}

interface PluginMatchRule {
  stage?: ObservationStage
  host?: string
  path?: string
  url?: string
  method?: string
  contentTypes?: string[]
  readBody?: boolean
}

interface PluginPageScriptMatch {
  host: string
  path?: string
  url?: string
}

interface PluginPageScript {
  id: string
  entry: string
  match: PluginPageScriptMatch[]
  runAt?: 'document-start'
  frames?: 'top' | 'all'
  bridge?: boolean
}

interface ResourceKindDefinition {
  id: string
  icon?: string
  color?: string
  locales?: Record<string, PluginLocale>
}

interface PluginProcessorDefinition {
  runtime: 'wasm'
  entry: string
  apiVersion: 1
}

interface PluginActionDefinition {
  kind: 'process-file'
  processor: string
  inputExtensions?: string[]
  outputExtension?: string
  locales?: Record<string, PluginLocale>
}

interface Selector {
  path?: string
  value?: JSONValue
}

interface DeclarativeResource {
  url: Selector
  title?: Selector
  coverUrl?: Selector
  kind?: Selector
  role?: Selector
  executor?: Selector
  contentType?: Selector
  extension?: Selector
  size?: Selector
  preview?: Selector
  metadata?: Record<string, Selector>
}

interface DeclarativeExtractor {
  stage: ObservationStage
  format: 'json'
  root?: string
  resource: DeclarativeResource
}

interface PluginManifest {
  /** Stable ID. builtin. is host-only; official. requires a bundled or official-store source. */
  id: string
  name: string
  author?: PluginAuthor
  version: string
  apiVersion: 1
  runtime: PluginRuntime
  entry?: string
  priority?: number
  enabled?: boolean
  permissions: PluginPermissions
  match: PluginMatchRule[]
  pageScripts?: PluginPageScript[]
  resourceKinds?: ResourceKindDefinition[]
  settingsSchema?: Record<string, unknown>
  extractors?: DeclarativeExtractor[]
  processors?: Record<string, PluginProcessorDefinition>
  actions?: Record<string, PluginActionDefinition>
  locales?: Record<string, PluginLocale>
  requires?: {ffmpeg?: string}
}

interface RequestSnapshot {
  method: string
  url: string
  host: string
  path: string
  headers: HeaderMap
  body?: string
  truncated?: boolean
}

interface ResponseSnapshot {
  statusCode: number
  headers: HeaderMap
  contentType: string
  body?: string
  truncated?: boolean
}

interface Observation {
  stage: ObservationStage
  request: RequestSnapshot
  response?: ResponseSnapshot
  settings?: Record<string, unknown>
}

interface DownloadStep {
  type: 'plugin-wasm' | 'xor-prefix' | string
  options?: Record<string, unknown>
}

interface ResourceTrack {
  id: string
  role: string
  executor?: AcquisitionExecutor
  url?: string
  captureKey?: string
  mime?: string
  extension?: string
  size?: number
  quality?: string
  width?: number
  height?: number
  bitrate?: number
  codecs?: string
  headers?: Record<string, string>
  nonPersistentHeaders?: string[]
  processors?: DownloadStep[]
}

interface PreviewSpec {
  renderer: PreviewRenderer
  mode?: 'proxy' | 'direct' | string
  mime?: string
  codecs?: string
  trackId?: string
}

interface ResourceAction {
  id: string
  label?: string
  data?: Record<string, unknown>
}

interface ResourceCandidate {
  id?: string
  groupKey?: string
  parentGroupKey?: string
  dedupeKey?: string
  kind: string
  primaryType?: PrimaryType
  traits?: string[]
  technical?: {mime?: string; container?: string; codecs?: string; duration?: number}
  lifecycle?: {expiresAt?: number}
  title?: string
  coverUrl?: string
  tracks?: ResourceTrack[]
  requiredTracks?: string[]
  capabilities?: ResourceCapability[]
  preview?: PreviewSpec
  metadata?: Record<string, unknown>
  actions?: ResourceAction[]
}

interface ResponsePatch {
  statusCode?: number
  headers?: Record<string, string>
  body?: string
}

interface SyntheticResponse {
  statusCode: number
  headers?: Record<string, string>
  body?: string
}

interface PluginResult {
  decision?: 'continue' | 'stop'
  handled?: boolean
  resources?: ResourceCandidate[]
  patch?: ResponsePatch
  syntheticResponse?: SyntheticResponse
  captures?: Array<{key: string; mode?: 'range-file'}>
  diagnostics?: string[]
}

interface DownloadOptions {
  selectedTrackIds?: string[]
  savePath?: string
  settings?: Record<string, unknown>
}

interface DownloadInput {
  id: string
  executor: AcquisitionExecutor
  url?: string
  captureKey?: string
  headers?: Record<string, string>
  extension?: string
  processors?: DownloadStep[]
  options?: Record<string, unknown>
}

interface PipelineStep {
  id: string
  executor: 'builtin.concat' | 'builtin.media.mux' | 'builtin.media.remux' | 'builtin.media.extract_audio' | 'plugin.ffmpeg'
  inputs: string[]
  options?: Record<string, unknown>
}

interface DownloadOutput {
  input: string
  extension?: string
  mime?: string
  processors?: DownloadStep[]
}

interface DownloadPlan {
  inputs: DownloadInput[]
  pipeline?: PipelineStep[]
  output: DownloadOutput
}

interface ResourceHookInput {
  resource: ResourceCandidate
  options: DownloadOptions
}

type ResourceRefreshResult =
  | {status?: 'refreshed'; resource: ResourceCandidate; message?: string}
  | {status: 'authenticationRequired' | 'recaptureRequired'; resource?: ResourceCandidate; message?: string}

interface CorrelationRegistration {
  groupKey: string
  trackId: string
  role: string
  aliases: string[]
}

interface CorrelationReference {
  groupKey: string
  trackId: string
  role: string
}

interface PageMessageContext {
  pageSessionId: string
  scriptId: string
  pageUrl: string
  origin: string
}

interface PageMessageResult {
  ok: boolean
  data?: unknown
  error?: string
  resources?: ResourceCandidate[]
  diagnostics?: string[]
  autoDownload?: boolean
}

interface PageSessionFilter {
  pageSessionId?: string
  scriptId?: string
  pageUrl?: string
  host?: string
}

interface PluginAPI {
  readonly pluginVersion: string
  emit(resource: ResourceCandidate): void
  upsert(resource: ResourceCandidate): void
  log(message: string): void
  correlate: {
    register(value: CorrelationRegistration): void
    find(url: string): CorrelationReference[]
  }
  page?: {
    send(pageSessionId: string, message: JSONValue): boolean
    broadcast(filter: PageSessionFilter, message: JSONValue): number
    sessions(filter?: PageSessionFilter): PageMessageContext[]
  }
}

interface PageScriptAPI {
  readonly pluginId: string
  readonly pluginVersion: string
  readonly scriptId: string
  readonly pageSessionId: string
  send(message: JSONValue): Promise<PageMessageResult>
  onMessage(listener: (message: JSONValue) => void): () => void
}

declare const pageApi: PageScriptAPI

declare function onObservation(observation: Observation, api: PluginAPI): PluginResult | void
declare function onPageMessage(message: JSONValue, context: PageMessageContext, api: PluginAPI): PageMessageResult | void
declare function refreshResource(input: ResourceHookInput): ResourceRefreshResult | null | void
declare function createDownloadPlan(input: ResourceHookInput): DownloadPlan | null | void
