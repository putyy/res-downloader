import type {appType} from "@/types/app"
import type {OperationDefinition, OperationExecution, OperationSession} from "@/types/operations"
import {OperationAPIError} from "@/api/operations"
type Translate = (key: string) => string
export const executionActive = (execution: OperationExecution) => ["queued", "running"].includes(execution.state)
export const operationErrorMessage = (error: unknown, t: Translate): string => {
  if (error instanceof OperationAPIError) {
    const key = `operations.errors.${error.code}`
    const translated = t(key)
    return translated === key ? error.message : translated
  }
  return t("operations.connection_error")
}
export const operationPageUnavailableMessage = (
  sessions: OperationSession[],
  target: {
    pluginId: string
    operationId?: string
    pageSessionId?: string
    definition?: OperationDefinition
  },
  t: Translate,
): string => {
  const fallback = t("operations.errors.page_unavailable")
  if (!target.pluginId || !target.operationId) return fallback
  const matching = sessions.filter(
    (session) =>
      session.pluginId === target.pluginId &&
      session.operations.includes(target.operationId!) &&
      (!target.pageSessionId || session.pageSessionId === target.pageSessionId) &&
      (!target.definition || session.scriptId === target.definition.pageScript),
  )
  const connected = matching.filter((session) => session.connected)
  if (!connected.length) return t("operations.errors.page_disconnected")
  const ready = connected.filter((session) => session.ready)
  if (!ready.length) return t("operations.errors.page_not_ready")
  if (target.definition?.requiresLogin && ready.every((session) => session.login !== "authenticated")) {
    return t("operations.errors.page_login_required")
  }
  // Availability may have changed since submission; do not infer a cause or retry.
  return fallback
}
export const executionMessage = (execution: OperationExecution, t: Translate): string => {
  if (execution.syncUnavailable) return t("operations.sync_unavailable")
  if (execution.errorCode) return operationErrorMessage(new OperationAPIError(execution.errorCode, execution.errorCode), t)
  return t(`operations.states.${execution.state}`)
}
export const shouldShowExecution = (execution: OperationExecution | undefined, download: appType.ResourceDownloadState | undefined): boolean => {
  if (!execution) return false
  const taskTime = Math.max(download?.createdAt ?? 0, download?.startedAt ?? 0)
  if (taskTime >= execution.createdAt) return false
  if (executionActive(execution) || !download) return true
  return taskTime ? taskTime < execution.createdAt : execution.state !== "succeeded"
}
