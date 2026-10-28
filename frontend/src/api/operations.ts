import request from "@/api/request"
import type {
  OperationInfo,
  OperationSession,
  OperationRequest,
  OperationExecution,
  OperationBatch,
  OperationPage,
  OperationSettings,
  OperationStatus,
  OperationArtifact,
  ArtifactText,
} from "@/types/operations"

export class OperationAPIError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message)
  }
}
async function call<T>(endpoint: string, data: Record<string, unknown> = {}): Promise<T> {
  const response = await request({
    url: `api/operations/${endpoint}`,
    method: "post",
    data,
    timeout: 10000,
  })
  if (response.code !== 1) throw new OperationAPIError(response.data?.errorCode || "operation_failed", response.message || "operation_failed")
  return response.data as T
}
export default {
  list: (pluginId = "") => call<OperationInfo[]>("list", {pluginId}),
  sessions: () => call<OperationSession[]>("sessions"),
  invoke: (request: OperationRequest) => call<OperationExecution>("invoke", {...request}),
  batch: (items: OperationRequest[], idempotencyKey?: string) => call<OperationBatch>("batch", {items, idempotencyKey}),
  execution: (id: string) => call<OperationExecution>("execution", {id}),
  batchGet: (id: string, offset = 0, limit = 32) => call<OperationBatch>("batch-get", {id, offset, limit}),
  history: (pluginId = "", state = "", offset = 0, limit = 20) => call<OperationPage>("history", {pluginId, state, offset, limit}),
  resources: (resourceIds: string[]) => call<OperationExecution[]>("resources", {resourceIds}),
  cancel: (id: string) => call<void>("cancel", {id}),
  cancelBatch: (id: string) => call<void>("batch-cancel", {id}),
  status: () => call<OperationStatus>("status"),
  settings: (settings: OperationSettings) => call<OperationStatus>("settings", {...settings}),
  clean: (history: boolean, results: boolean) => call<void>("clean", {history, results}),
  artifact: (id: string) => call<OperationArtifact>("artifact", {id}),
  text: (id: string, offset = 0, limit = 32768) => call<ArtifactText>("text", {id, offset, limit}),
}
