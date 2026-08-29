<template>
  <NCard
      size="small"
      :bordered="false"
      class="app-card app-card--interactive plugin-card h-full"
      style="--wails-draggable:no-drag"
  >
    <template #header>
      <div class="min-w-0">
        <div class="truncate font-medium" :title="name">{{ name }}</div>
        <div class="mt-1 flex flex-wrap gap-1">
          <NTag size="small">{{ plugin.manifest.runtime || 'invalid' }}</NTag>
          <NTag v-if="plugin.source === 'builtin'" size="small" type="info">{{ t('plugin.builtin') }}</NTag>
          <NTag v-else-if="plugin.source === 'official'" size="small" type="info">{{ t('plugin.official') }}</NTag>
          <NTag v-else size="small">{{ t('plugin.store_community') }}</NTag>
          <NTag v-if="plugin.loaded" size="small" type="success">{{ t('plugin.loaded') }}</NTag>
          <NTag v-else size="small">{{ t('plugin.disabled') }}</NTag>
        </div>
      </div>
    </template>
    <template #header-extra>
      <NSwitch
          :value="plugin.loaded"
          :loading="updatingId === plugin.manifest.id"
          :disabled="plugin.builtin || !!plugin.error || !plugin.manifest.id || updatingId !== ''"
          @update:value="(enabled: any) => emit('enabled', plugin.manifest.id, enabled)"
      />
    </template>

    <div class="app-muted-text text-xs">
      {{ plugin.manifest.id || '-' }} · v{{ plugin.manifest.version || '-' }} · API {{
        plugin.manifest.apiVersion || '-'
      }}
    </div>
    <div v-if="description" class="app-muted-text ellipsis-2 mt-2 min-h-[42px] text-sm">{{ description }}</div>
    <div class="app-muted-text mt-3 flex items-center gap-1 text-xs">
      <span>{{ t('plugin.developer') }}：</span>
      <NButton v-if="plugin.manifest.author?.url" text type="primary" size="tiny" @click="openHomepage">
        {{ plugin.manifest.author?.name || t('plugin.unknown_developer') }}
      </NButton>
      <span v-else>{{ plugin.manifest.author?.name || t('plugin.unknown_developer') }}</span>
    </div>
    <div v-if="plugin.manifest.requires?.ffmpeg" class="app-muted-text mt-2 text-xs">
      {{ t('plugin.requires_ffmpeg', {version: plugin.manifest.requires.ffmpeg}) }}
    </div>
    <NAlert v-if="plugin.error" class="mt-3" type="error">
      <div class="ellipsis-3" :title="plugin.error">{{ plugin.error }}</div>
    </NAlert>
    <PluginRuntimeHealth :health="plugin.health"/>

    <template #footer>
      <div class="flex min-h-[28px] flex-wrap items-center justify-end gap-2">
        <NButton v-if="plugin.manifest.id === genericDetectorID" size="small" secondary @click="emit('resourceRules')">
          {{ t('plugin.resource_rules') }}
        </NButton>
        <NPopconfirm
            v-if="plugin.rollbackAvailable"
            :positive-text="t('plugin.rollback')"
            :negative-text="t('plugin.cancel')"
            @positive-click="emit('rollback', plugin.manifest.id)"
        >
          <template #trigger>
            <NButton size="small" secondary :loading="rollbackId === plugin.manifest.id">
              {{ t('plugin.rollback') }}
            </NButton>
          </template>
          {{ t('plugin.rollback_confirm', {name}) }}
        </NPopconfirm>
        <NButton
            v-if="plugin.manifest.id !== genericDetectorID && hasSettings"
            size="small"
            secondary
            @click="emit('configure', plugin.manifest.id)"
        >
          {{ t('plugin.configure') }}
        </NButton>
        <NPopconfirm
            v-if="!plugin.builtin"
            :positive-text="t('plugin.uninstall')"
            :negative-text="t('plugin.cancel')"
            @positive-click="emit('uninstall', plugin.manifest.id)"
        >
          <template #trigger>
            <NButton
                size="small"
                secondary
                type="error"
                :loading="uninstallingId === plugin.manifest.id"
                :disabled="uninstallingId !== ''"
            >
              {{ t('plugin.uninstall') }}
            </NButton>
          </template>
          {{ t('plugin.uninstall_confirm', {name}) }}
        </NPopconfirm>
      </div>
    </template>
  </NCard>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import type {appType} from '@/types/app'
import {BrowserOpenURL} from '../../../wailsjs/runtime'
import PluginRuntimeHealth from './PluginRuntimeHealth.vue'

const genericDetectorID = 'builtin.generic-detector'
const props = defineProps<{
  plugin: appType.PluginStatus
  updatingId: string
  rollbackId: string
  uninstallingId: string
}>()
const emit = defineEmits<{
  enabled: [id: string, enabled: boolean]
  resourceRules: []
  rollback: [id: string]
  configure: [id: string]
  uninstall: [id: string]
}>()
const {t, locale} = useI18n()

const localizedEntry = computed(() => {
  const entries = props.plugin.manifest.locales ?? {}
  const current = locale.value
  return entries[current] ?? entries[current.split('-')[0]] ?? entries.en ?? Object.values(entries)[0] ?? {}
})
const name = computed(() => localizedEntry.value.name || props.plugin.manifest.name || props.plugin.manifest.id)
const description = computed(() => localizedEntry.value.description || '')
const hasSettings = computed(() => {
  const schema = props.plugin.manifest.settingsSchema
  return !!schema && Object.keys(schema).length > 0
})

const openHomepage = () => {
  const target = props.plugin.manifest.author?.url
  if (!target) return
  try {
    const parsed = new URL(target)
    if (parsed.protocol === 'https:' || parsed.protocol === 'http:') BrowserOpenURL(target)
    else throw new Error('unsupported protocol')
  } catch (_) {
    window?.$message?.error(t('plugin.invalid_homepage'))
  }
}
</script>
