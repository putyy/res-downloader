export interface MediaDisplayItem {
  Classify?: string
  Description?: string
  OtherData?: Record<string, string>
}

type Translate = (key: string, params?: Record<string, string>) => string

export const hasCapturedImageSetAudio = (item: MediaDisplayItem): boolean => {
  return item.Classify === 'image_set' && Boolean(item.OtherData?.image_set_audio_url)
}

export const formatMediaTypeLabel = (
  item: MediaDisplayItem,
  fallback: string,
  t: Translate,
): string => {
  return hasCapturedImageSetAudio(item) ? t('index.image_set_with_audio') : fallback
}

export const formatImageSetDescription = (item: MediaDisplayItem, t: Translate): string => {
  const count = item.OtherData?.image_set_count || '0'
  const labelKey = hasCapturedImageSetAudio(item)
    ? 'index.image_set_count_with_audio'
    : 'index.image_set_count'
  return `[${t(labelKey, {count})}] ${item.Description || ''}`
}
