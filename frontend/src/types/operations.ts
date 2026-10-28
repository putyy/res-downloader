export interface OperationDefinition {
  name: string
  description?: string
  locales?: Record<string, {name?: string; description?: string}>
  category: string
  pageScript: string
  requiresLogin?: boolean
  inputSchema: Record<string, any>
  outputSchema: Record<string, any>
  examples?: Record<string, unknown>[]
  effects: string[]
  timeoutSeconds?: number
  cancellable?: boolean
  safeRetry?: boolean
  allowReload?: boolean
  automation?: boolean
  persistResult?: boolean
  resultTTLSeconds?: number
}
export interface OperationInfo {
  pluginId: string
  pluginVersion: string
  operationId: string
  definition: OperationDefinition
  automationEnabled: boolean
  available: boolean
}
export interface OperationSession {
  pluginId: string
  scriptId: string
  pageSessionId: string
  title: string
  pageUrl: string
  connected: boolean
  ready: boolean
  login: string
  revision: string
  operations: string[]
}
export interface OperationRequest {
  pluginId: string
  operationId: string
  pageSessionId?: string
  input: Record<string, unknown>
  cursor?: string
  limit?: number
  idempotencyKey?: string
  retryOf?: string
  resourceId?: string
  actionId?: string
}
export type ExecutionState = "queued" | "running" | "succeeded" | "failed" | "cancelled" | "timed_out" | "interrupted"
export interface OperationExecution {
  executionId: string
  batchId?: string
  index: number
  pluginId: string
  pluginVersion: string
  operationId: string
  pageSessionId: string
  source: string
  state: ExecutionState
  certainty: string
  errorCode?: string
  progress?: number
  cancelRequested: boolean
  cancelConfirmed: boolean
  inputSummary?: unknown
  resultStatus: string
  result?: any
  resultProjection: boolean
  resultExpiresAt?: number
  createdAt: number
  updatedAt: number
  retryOf?: string
  resourceId?: string
  resourceIds: string[]
  reloadCount?: number
  downloadTaskIds: string[]
  artifactIds: string[]
  /** A failed UI refresh does not change the host execution state. */
  syncUnavailable?: boolean
}
export interface OperationPage {
  items: OperationExecution[]
  total: number
  offset: number
  hasMore: boolean
}
export interface OperationBatch extends OperationPage {
  batchId: string
  counts: Record<string, number>
}
export interface OperationSettings {
  historyDays: number
  resultHours: number
  automation: Record<string, boolean>
}
export interface OperationStatus {
  available: boolean
  settings: OperationSettings
  limits: Record<string, number>
}
export interface OperationArtifact {
  artifactId: string
  executionId: string
  pluginId: string
  resourceId?: string
  downloadTaskId?: string
  mime: string
  size: number
  status: string
  expiresAt?: number
  output?: string
}
export interface ArtifactText {
  text: string
  offset: number
  nextOffset: number
  size: number
  truncated: boolean
  encoding: string
}
