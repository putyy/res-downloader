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

interface ProcessFileActionDefinition {
  kind: 'process-file'
  processor: string
  inputExtensions?: string[]
  outputExtension?: string
  locales?: Record<string, PluginLocale>
}

interface OperationActionDefinition {
  kind: 'operation'
  /** ID in the same manifest's operations map. */
  operation: string
  locales?: Record<string, PluginLocale>
}

type PluginActionDefinition = ProcessFileActionDefinition | OperationActionDefinition

/** Strict subset, maximum depth 12; no $ref, unions, defaults or pattern. */
interface OperationSchema {
  type: 'object' | 'array' | 'string' | 'number' | 'integer' | 'boolean' | 'null'
  title?: string
  description?: string
  properties?: Record<string, OperationSchema>
  /** Required for every object schema. */
  additionalProperties?: false
  required?: string[]
  items?: OperationSchema
  enum?: JSONPrimitive[]
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
  minItems?: number
  maxItems?: number
  /** Opt in individual scalar leaves only. */
  'x-persist'?: boolean
  /** Excludes this subtree, overriding x-persist. */
  'x-sensitive'?: boolean
}

interface OperationDefinition {
  name: string
  description?: string
  locales?: Record<string, PluginLocale>
  category: 'search' | 'detail' | 'current-content' | 'list' | 'resolve' | 'text' | 'custom'
  pageScript: string
  pageMatch?: PluginPageScriptMatch[]
  requiresLogin?: boolean
  inputSchema: OperationSchema & {type: 'object'}
  outputSchema: OperationSchema
  examples?: Record<string, JSONValue>[]
  effects: ('read' | 'page' | 'publish' | 'download')[]
  timeoutSeconds?: number
  cancellable?: boolean
  /** Read-only only. No automatic retries; explicit retryOf has a bounded retry chain. */
  safeRetry?: boolean
  /** Defaults to false. Requires the page effect; permits one SDK-managed reload within the original execution deadline. */
  allowReload?: boolean
  automation?: boolean
  persistResult?: boolean
  resultTTLSeconds?: number
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
  operations?: Record<string, OperationDefinition>
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

interface ResourceMetadata extends Record<string, unknown> {
  /** Optional positive integer Unix milliseconds, <= 253402300799999. Never seconds or strings. */
  createdAt?: number
  /** Publication time, distinct from creation time. Same millisecond contract as createdAt. */
  publishedAt?: number
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
  metadata?: ResourceMetadata
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
  /** Effective settings; only supplied to onPageMessage, not exposed to page scripts or sessions(). */
  settings?: Record<string, unknown>
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

interface OperationContext<I = Record<string, JSONValue>> {
  executionId: string
  /** Zero initially, one after the explicitly requested reload. */
  reloadCount: 0 | 1
  input: I
  /** Plugin's original opaque cursor; host binds the public token to query/session. */
  cursor?: string
  limit: number
  resource?: {id: string; actionId: string}
  signal: AbortSignal
  /** Percent 0–100. The SDK sends heartbeat reports automatically. */
  report(progress?: number): Promise<unknown>
  /** Requires allowReload. Clean up uncommitted capture data first; never call after final submission.
   * Ends this handler and reloads the same URL once. Await or return it; do not catch it to continue work.
   */
  reload(): Promise<never>
}

interface OperationPageResult<T = JSONValue> {
  data: T
  count?: number
  hasMore?: boolean
  nextCursor?: string
  truncated?: boolean
}

interface OperationResultMessage {
  type: 'operation-result'
  operationId: string
  executionId: string
  data: JSONValue
}

interface PageSessionFilter {
  pageSessionId?: string
  scriptId?: string
  pageUrl?: string
  host?: string
}

interface PluginBaseAPI {
  readonly pluginVersion: string
  /** Writes only when the current plugin setting enableLog is boolean true; disabled by default. */
  log(message: string): void
}

interface PluginAPI extends PluginBaseAPI {
  emit(resource: ResourceCandidate): void
  upsert(resource: ResourceCandidate): void
  capture?: {
    save(data: string | ArrayBuffer | Uint8Array | Uint8ClampedArray): {captureKey: string; size: number}
  }
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
  operations: {
    /** Read-only bootstrap hint; does not claim or replay an execution. */
    hasPendingReload(): boolean
    /** Update on SPA/account/readiness changes; changing state interrupts prior work. */
    setState(state: {ready: boolean; login?: 'unknown' | 'authenticated' | 'required'; context?: string}): Promise<{revision: string}>
    handle<I = Record<string, JSONValue>, O = JSONValue>(id: string, handler: (context: OperationContext<I>) => Promise<OperationPageResult<O>> | OperationPageResult<O>): () => void
  }
  capture: {
    start(key: string): Promise<{ok: true}>
    write(key: string, body: ArrayBuffer | ArrayBufferView): Promise<{ok: true}>
    complete(key: string): Promise<{ok: true}>
    abort(key: string): Promise<{ok: true}>
  }
}

declare const pageApi: PageScriptAPI

declare function onObservation(observation: Observation, api: PluginAPI): PluginResult | void
declare function onPageMessage(message: JSONValue, context: PageMessageContext, api: PluginAPI): PageMessageResult | void
declare function refreshResource(input: ResourceHookInput, api: PluginBaseAPI): ResourceRefreshResult | null | void
declare function createDownloadPlan(input: ResourceHookInput, api: PluginBaseAPI): DownloadPlan | null | void

/** Public host discovery/execution shapes used by desktop and automation. */
interface OperationInfo {
  pluginId: string
  pluginVersion: string
  operationId: string
  definition: OperationDefinition
  automationEnabled: boolean
  available: boolean
}

interface OperationSession {
  pluginId: string
  scriptId: string
  pageSessionId: string
  title: string
  pageUrl: string
  connected: boolean
  ready: boolean
  login: 'unknown' | 'authenticated' | 'required'
  revision: string
  operations: string[]
}

interface OperationRequest {
  pluginId: string
  operationId: string
  input: Record<string, JSONValue>
  pageSessionId?: string
  cursor?: string
  limit?: number
  idempotencyKey?: string
  retryOf?: string
  resourceId?: string
  actionId?: string
}

type OperationState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'timed_out' | 'interrupted'
type OperationResultStatus = 'available' | 'expired' | 'cleaned' | 'never_persisted'

interface OperationResult<T = JSONValue> {
  data: T
  /** Omitted from a recovered persisted projection. */
  pagination?: {cursor: string; hasMore: boolean; count: number; truncated: boolean}
}

interface OperationExecution<T = JSONValue> {
  executionId: string
  batchId?: string
  index: number
  pluginId: string
  pluginVersion: string
  operationId: string
  pageSessionId: string
  revision: string
  source: 'desktop' | 'automation'
  state: OperationState
  certainty: 'not_started' | 'confirmed' | 'unknown'
  errorCode?: string
  progress?: number
  cancelRequested: boolean
  cancelConfirmed: boolean
  inputSummary?: JSONValue
  resultStatus: OperationResultStatus
  result?: OperationResult<T>
  resultProjection: boolean
  resultExpiresAt?: number
  createdAt: number
  updatedAt: number
  startedAt?: number
  acceptedAt?: number
  deadline?: number
  retryOf?: string
  /** Number of explicit page reloads, at most one. */
  reloadCount?: number
  resourceId?: string
  resourceIds: string[]
  downloadTaskIds: string[]
  /** Revoked with result cleanup/expiry; memory-only when persistResult is false. */
  artifactIds: string[]
}

interface OperationBatch {
  batchId: string
  items: OperationExecution[]
  counts: Partial<Record<OperationState, number>>
  total: number
  offset: number
  hasMore: boolean
}

interface OperationArtifact {
  artifactId: string
  executionId: string
  pluginId: string
  resourceId?: string
  downloadTaskId?: string
  mime: string
  size: number
  status: 'available' | 'pending' | 'expired' | 'unavailable'
  expiresAt?: number
  output?: string
}

interface OperationTextChunk {
  text: string
  offset: number
  nextOffset: number
  size: number
  truncated: boolean
  encoding: 'utf-8'
}

/** Shared content-item convention. Each plugin must declare it in outputSchema. */
interface OperationContentItem {
  pluginId: string
  id: string
  kind: string
  title: string
  pageUrl: string
  capabilities: ('detail' | 'list' | 'resolve' | 'text')[]
  author?: string
  coverUrl?: string
  publishedAt?: number
  duration?: number
  summary?: string
}
