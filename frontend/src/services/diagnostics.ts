import * as bind from '../../wailsjs/go/app/Bind'

export const frontendErrorDetails = (error: unknown): string => {
    if (error instanceof Error) {
        return error.stack || error.message
    }
    if (typeof error === 'string') return error
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
