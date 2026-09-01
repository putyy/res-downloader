<template>
  <div class="app-shell relative flex h-full w-full items-center justify-center px-6">
    <div class="absolute inset-x-0 top-0 h-10" style="--wails-draggable:drag">
      <div v-if="showCustomWindowControls" class="w-[84px]" style="--wails-draggable:no-drag">
        <Screen/>
      </div>
    </div>

    <div class="relative z-10 w-full max-w-[620px] rounded-2xl border border-[var(--app-border)] bg-[var(--app-surface)] p-8 shadow-xl">
      <div class="flex items-center gap-4">
        <img class="h-14 w-14 rounded-full" src="@/assets/image/logo.png" alt="res-downloader logo"/>
        <div>
          <div class="text-xl font-semibold">res-downloader</div>
          <div class="app-muted-text mt-1 text-sm">{{ statusSummary }}</div>
        </div>
      </div>

      <div v-if="store.startupState === 'loading'" class="flex flex-col items-center py-14">
        <NSpin size="large"/>
        <div class="mt-5 text-base font-medium">{{ t('startup.loading_title') }}</div>
        <div class="app-muted-text mt-2 text-sm">{{ t('startup.loading_tip') }}</div>
      </div>

      <div v-else class="mt-7">
        <NAlert type="error" :show-icon="false" :title="t('startup.failed_title')">
          {{ t('startup.failed_tip') }}
        </NAlert>
        <div class="app-muted-text mt-4 text-xs font-medium">{{ t('startup.error_details') }}</div>
        <pre class="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-black/5 p-3 text-xs leading-5 dark:bg-white/5">{{ store.startupError }}</pre>
        <div class="mt-6 flex flex-wrap gap-2" style="--wails-draggable:no-drag">
          <NButton type="primary" secondary @click="retry">
            <template #icon><NIcon><RefreshOutline/></NIcon></template>
            {{ t('startup.retry') }}
          </NButton>
          <NButton secondary @click="copyDiagnostics">
            <template #icon><NIcon><CopyOutline/></NIcon></template>
            {{ t('startup.copy_error') }}
          </NButton>
          <NButton secondary @click="openLogDirectory">
            <template #icon><NIcon><FolderOpenOutline/></NIcon></template>
            {{ t('startup.open_logs') }}
          </NButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {CopyOutline, FolderOpenOutline, RefreshOutline} from '@vicons/ionicons5'
import {NButton, NIcon} from 'naive-ui'
import {useI18n} from 'vue-i18n'
import {useIndexStore} from '@/stores'
import Screen from '@/components/Screen.vue'
import {ClipboardSetText} from '../../wailsjs/runtime'
import * as bind from '../../wailsjs/go/app/Bind'

const {t} = useI18n()
const store = useIndexStore()

const showCustomWindowControls = computed(() => {
  if (store.envInfo.platform) return store.envInfo.platform !== 'darwin'
  return !/Macintosh|Mac OS X/i.test(navigator.userAgent)
})

const statusSummary = computed(() => store.startupState === 'loading'
    ? t('startup.loading_summary')
    : t('startup.failed_summary'))

const diagnosticText = () => [
  `res-downloader: ${store.appInfo.Version || 'unknown'}`,
  `platform: ${store.envInfo.platform || 'unknown'}/${store.envInfo.arch || 'unknown'}`,
  `time: ${new Date().toISOString()}`,
  `userAgent: ${navigator.userAgent}`,
  '',
  store.startupError,
].join('\n')

const retry = () => void store.init()

const copyDiagnostics = async () => {
  const details = diagnosticText()
  let copied = false
  try {
    copied = await ClipboardSetText(details)
  } catch {
    try {
      await navigator.clipboard.writeText(details)
      copied = true
    } catch {
      copied = false
    }
  }
  if (copied) window.$message?.success(t('common.copy_success'))
  else window.$message?.error(t('common.copy_fail'))
}

const openLogDirectory = async () => {
  try {
    await bind.OpenLogDirectory()
  } catch (error: any) {
    window.$message?.error(t('startup.open_logs_failed', {message: String(error?.message ?? error)}))
  }
}
</script>
