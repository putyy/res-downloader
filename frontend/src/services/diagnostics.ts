import * as bind from "../../wailsjs/go/app/Bind"
import axios from "axios"

const requestTarget = (url?: string, baseURL?: string): string => {
  if (!url) return "unknown"
  try {
    const target = new URL(url, new URL(baseURL || "/", window.location.href))
    // Exclude credentials, query parameters and fragments from diagnostics.
    return `${target.protocol}//${target.host}${target.pathname}`
  } catch {
    return "unknown"
  }
}

export const frontendErrorDetails = (error: unknown): string => {
  if (axios.isAxiosError(error)) {
    // Never serialize Axios config, headers, request/response bodies or
    // toJSON(): they may contain session tokens and user data.
    return [
      `AxiosError: ${error.response ? `HTTP ${error.response.status}` : "Request failed"}`,
      `request: ${error.config?.method?.toUpperCase() || "unknown"} ${requestTarget(error.config?.url, error.config?.baseURL)}`,
      `status: ${error.response?.status ?? "no response"}`,
      `code: ${error.code || "unknown"}`,
    ].join("\n")
  }
  if (error instanceof Error) {
    return error.stack || error.message
  }
  if (typeof error === "string") return error
  try {
    return JSON.stringify(error)
  } catch {
    return String(error)
  }
}

export const reportFrontendError = async (source: string, error: unknown): Promise<string> => {
  const details = frontendErrorDetails(error)
  try {
    await bind.LogFrontendError(`[${source}] ${details}`)
  } catch {
    // The Wails bridge itself may be the reason startup failed. The error
    // still remains visible and copyable in the startup failure screen.
  }
  return details
}
