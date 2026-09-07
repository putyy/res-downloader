import type {appType} from '@/types/app'

type Translate = (key: string) => string
const errorKeys: Record<string, string> = {
    page_command_already_active: 'index.page_command_already_active',
    page_command_limit_reached: 'index.page_command_limit_reached',
    page_command_no_page: 'index.page_command_no_page',
    page_command_queue_full: 'index.page_command_queue_full',
    page_command_unavailable: 'index.page_command_unavailable',
    page_command_too_large: 'index.page_command_too_large',
    page_command_start_failed: 'index.page_command_start_failed',
    page_command_not_accepted: 'index.page_command_not_accepted',
    page_command_timeout: 'index.page_command_timeout',
    page_command_target_unavailable: 'index.page_command_target_unavailable',
    page_command_reloaded: 'index.page_command_reloaded',
}

export const pageCommandErrorMessage = (code: string | undefined, fallback: string, t: Translate): string =>
    code && Object.prototype.hasOwnProperty.call(errorKeys, code) ? t(errorKeys[code]) : fallback

export const pageCommandMessage = (command: appType.PageCommandStatus, t: Translate): string =>
    command.syncUnavailable ? t('index.page_command_sync_unavailable') :
        pageCommandErrorMessage(command.errorCode, command.message || t(`index.page_command_${command.state}`), t)

export const shouldShowPageCommand = (
    command: appType.PageCommandStatus | undefined,
    download: appType.ResourceDownloadState | undefined,
): boolean => {
    if (!command) return false
    const taskTime = Math.max(download?.createdAt ?? 0, download?.startedAt ?? 0)
    if (command.syncUnavailable && taskTime >= command.createdAt) return false
    if (command.state === 'pending' || command.state === 'running' || !download) return true
    // A previous completed download is not the result of this new page operation.
    if (taskTime) return taskTime < command.createdAt
    return command.state !== 'completed'
}
