<template>
  <NCard size="small" :bordered="false" class="app-card app-card--interactive !flex flex-col [&>.n-card__content]:flex-1 [&>.n-card__footer]:mt-auto hover:-translate-y-0.5 h-full [--wails-draggable:no-drag]">
    <template #header>
      <div class="min-w-0">
        <div class="truncate font-medium" :title="name">{{ name }}</div>
        <div class="mt-1 flex items-start gap-2">
          <div class="flex min-w-0 flex-1 flex-wrap gap-1">
            <NTag v-if="extension.source === 'official'" size="small" type="info">{{ t('plugin.store_official') }}</NTag>
            <NTag v-else size="small">{{ t('plugin.store_community') }}</NTag>
            <NTag v-if="updateAvailable" size="small" type="warning">
              {{ t('plugin.store_update_available') }}
            </NTag>
            <NTag v-if="extension.status === 'available'" size="small" type="success">
              <template v-if="updateAvailable && installedVersion">
                v{{ installedVersion }} → v{{ extension.release?.version }}
              </template>
              <template v-else>v{{ extension.release?.version }}</template>
            </NTag>
            <NTag v-else size="small" type="warning">{{ t('plugin.store_unavailable') }}</NTag>
          </div>
          <button
              v-if="extension.stars"
              type="button"
              class="text-app-muted inline-flex h-[22px] shrink-0 cursor-pointer items-center gap-1 rounded-sm border-0 bg-transparent p-0 text-xs transition-opacity hover:opacity-75 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
              :title="t('plugin.store_repository')"
              @click="openRepository"
          >
            <span aria-hidden="true">★</span>
            <span>{{ extension.stars }}</span>
          </button>
        </div>
      </div>
    </template>

    <div class="text-app-muted text-xs">
      {{ extension.repository }}<span v-if="extension.license"> · {{ extension.license }}</span>
    </div>
    <NTooltip v-if="description" trigger="hover">
      <template #trigger>
        <div class="text-app-muted ellipsis-2 mt-2 min-h-[42px] cursor-default text-sm">{{ description }}</div>
      </template>
      <div class="max-w-[min(360px,calc(100vw-48px))] whitespace-pre-wrap break-words text-sm">{{ description }}</div>
    </NTooltip>
    <div class="text-app-muted mt-3 flex flex-wrap items-center gap-x-1 gap-y-1 text-xs">
      <span class="min-w-0 break-words">{{ t('plugin.developer') }}：{{ extension.manifest?.author?.name || extension.owner }}</span>
      <NTooltip v-if="updatedTime" trigger="hover">
        <template #trigger>
          <span class="cursor-default whitespace-nowrap">· {{ t('plugin.store_updated_at', {date: updatedTime.date}) }}</span>
        </template>
        {{ updatedTime.full }}
      </NTooltip>
    </div>
    <NTooltip v-if="capabilities.length" trigger="hover">
      <template #trigger>
        <div class="mt-2 grid grid-cols-2 gap-1">
          <NTag
              v-for="capability in visibleCapabilities"
              :key="capability"
              class="!w-full !min-w-0 !justify-center [&_.n-tag__content]:min-w-0 [&_.n-tag__content]:truncate"
              size="small"
              type="warning"
          >
            {{ capability }}
          </NTag>
          <NTag v-if="hiddenCapabilityCount" class="!w-full !min-w-0 !justify-center [&_.n-tag__content]:min-w-0 [&_.n-tag__content]:truncate" size="small" type="warning">
            +{{ hiddenCapabilityCount }}
          </NTag>
        </div>
      </template>
      <div class="max-w-[360px] break-words text-xs">{{ capabilities.join(' · ') }}</div>
    </NTooltip>
    <NAlert v-if="extension.statusMessage" class="mt-3" :type="extension.status === 'available' ? 'info' : 'warning'">
      <div class="ellipsis-3" :title="extension.statusMessage">{{ extension.statusMessage }}</div>
    </NAlert>

    <template #footer>
      <div class="flex min-h-[28px] flex-wrap items-center justify-end gap-2">
        <NButton size="small" text type="primary" @click="openRepository">{{ t('plugin.store_repository') }}</NButton>
        <NButton
            size="small"
            type="primary"
            :loading="installingRepository === extension.repository"
            :disabled="installDisabled || installingRepository !== ''"
            @click="emit('install', extension)"
        >
          <template v-if="updateAvailable" #icon>
            <NIcon><RefreshOutline/></NIcon>
          </template>
          {{ installLabel }}
        </NButton>
      </div>
    </template>
  </NCard>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {RefreshOutline} from '@vicons/ionicons5'
import type {appType} from '@/types/app'
import {BrowserOpenURL} from '../../../wailsjs/runtime'

const props = defineProps<{
  extension: appType.PluginStoreEntry
  installingRepository: string
  installDisabled: boolean
  installLabel: string
  updateAvailable: boolean
  installedVersion?: string
}>()
const emit = defineEmits<{ install: [extension: appType.PluginStoreEntry] }>()
const {t, locale} = useI18n()

const localizedEntry = computed(() => {
  const entries = props.extension.manifest?.locales ?? {}
  const current = locale.value
  return entries[current] ?? entries[current.split('-')[0]] ?? entries.en ?? Object.values(entries)[0] ?? {}
})
const name = computed(() => localizedEntry.value.name || props.extension.manifest?.name || props.extension.name)
const description = computed(() => localizedEntry.value.description || props.extension.description || '')
const updatedTime = computed(() => {
  const timestamp = Date.parse(props.extension.updatedAt ?? '')
  if (!Number.isFinite(timestamp)) return null
  const date = new Date(timestamp)
  const pad = (value: number) => String(value).padStart(2, '0')
  return {
    date: `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`,
    full: new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false, timeZoneName: 'short',
    }).format(date),
  }
})
const capabilities = computed(() => props.extension.manifest?.permissions?.capabilities ?? [])
const visibleCapabilities = computed(() =>
    capabilities.value.length > 4 ? capabilities.value.slice(0, 3) : capabilities.value)
const hiddenCapabilityCount = computed(() => capabilities.value.length - visibleCapabilities.value.length)

const openRepository = () => {
  try {
    const parsed = new URL(props.extension.repositoryUrl)
    if (parsed.protocol === 'https:' || parsed.protocol === 'http:') BrowserOpenURL(props.extension.repositoryUrl)
  } catch (_) {
    window?.$message?.error(t('plugin.invalid_homepage'))
  }
}
</script>
