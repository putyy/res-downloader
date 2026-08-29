<template>
  <NCard size="small" :bordered="false" class="app-card app-card--interactive plugin-card h-full" style="--wails-draggable:no-drag">
    <template #header>
      <div class="min-w-0">
        <div class="truncate font-medium" :title="name">{{ name }}</div>
        <div class="mt-1 flex flex-wrap gap-1">
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
          <NTag v-if="extension.stars" size="small">★ {{ extension.stars }}</NTag>
        </div>
      </div>
    </template>

    <div class="app-muted-text text-xs">
      {{ extension.repository }}<span v-if="extension.license"> · {{ extension.license }}</span>
    </div>
    <div v-if="description" class="app-muted-text ellipsis-2 mt-2 min-h-[42px] text-sm">{{ description }}</div>
    <div class="app-muted-text mt-3 text-xs">
      {{ t('plugin.developer') }}：{{ extension.manifest?.author?.name || extension.owner }}
    </div>
    <NTooltip v-if="capabilities.length" trigger="hover">
      <template #trigger>
        <div class="mt-2 grid grid-cols-2 gap-1">
          <NTag
              v-for="capability in visibleCapabilities"
              :key="capability"
              class="plugin-permission-tag"
              size="small"
              type="warning"
          >
            {{ capability }}
          </NTag>
          <NTag v-if="hiddenCapabilityCount" class="plugin-permission-tag" size="small" type="warning">
            +{{ hiddenCapabilityCount }}
          </NTag>
        </div>
      </template>
      <div class="max-w-[360px] break-words text-xs">{{ capabilities.join(' · ') }}</div>
    </NTooltip>
    <NAlert v-if="hasPageInjectionPermission" class="mt-3" type="error" :show-icon="false">
      <div class="ellipsis-3" :title="t('plugin.page_injection_warning')">
        {{ t('plugin.page_injection_warning') }}
      </div>
    </NAlert>
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
const capabilities = computed(() => props.extension.manifest?.permissions?.capabilities ?? [])
const visibleCapabilities = computed(() =>
    capabilities.value.length > 4 ? capabilities.value.slice(0, 3) : capabilities.value)
const hiddenCapabilityCount = computed(() => capabilities.value.length - visibleCapabilities.value.length)
const hasPageInjectionPermission = computed(() =>
    capabilities.value.some(capability =>
      capability === 'inject-page-script' || capability === 'page-bridge' || capability === 'enqueue-download'))

const openRepository = () => {
  try {
    const parsed = new URL(props.extension.repositoryUrl)
    if (parsed.protocol === 'https:' || parsed.protocol === 'http:') BrowserOpenURL(props.extension.repositoryUrl)
  } catch (_) {
    window?.$message?.error(t('plugin.invalid_homepage'))
  }
}
</script>
