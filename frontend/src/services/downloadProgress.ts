import type {appType} from "@/types/app"

const nonNegativeFinite = (value: number | undefined): number => (value != null && Number.isFinite(value) && value > 0 ? value : 0)

export const downloadTaskProgress = (task: appType.DownloadTaskRecord) => {
  const downloaded = nonNegativeFinite(task.downloaded)
  // HLS and recordings report -1 when the byte total is unknown.
  const total = nonNegativeFinite(task.total) || task.items?.length || 0
  const percentage = total > 0 ? Math.min(100, (downloaded / total) * 100) : 0
  return {downloaded, total, percentage}
}

export const formatDownloadBytes = (value: number) => {
  const bytes = nonNegativeFinite(value)
  if (!bytes) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const index = Math.max(0, Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1))
  return `${(bytes / Math.pow(1024, index)).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}
