import type {appType} from '@/types/app'

export const activeTaskStates: appType.DownloadTaskRecord['state'][] = [
    'pending',
    'resolving',
    'downloading',
    'processing',
    'pausing',
    'paused',
]

export const isActiveTask = (task: appType.DownloadTaskRecord) => activeTaskStates.includes(task.state)

export const canPauseTask = (task: appType.DownloadTaskRecord) =>
    !!task.resumable && ['pending', 'resolving', 'downloading'].includes(task.state)

export const canResumeTask = (task: appType.DownloadTaskRecord) =>
    !!task.resumable && ['paused', 'interrupted'].includes(task.state)

export const canRetryTask = (task: appType.DownloadTaskRecord) =>
    ['failed', 'cancelled'].includes(task.state) || (task.state === 'interrupted' && !task.resumable)

export const canCancelTask = (task: appType.DownloadTaskRecord) => isActiveTask(task) && !task.recording

export const canDeleteTask = (task: appType.DownloadTaskRecord) => !isActiveTask(task)
