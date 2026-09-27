<template>
  <NModal v-model:show="visible" preset="card" :title="t(status.available ? 'update.title' : 'update.version')"
          class="w-[600px] max-w-[calc(100vw-40px)] [--wails-draggable:no-drag]" :mask-closable="true">
    <div class="flex items-center gap-4 rounded-xl bg-black/[0.035] dark:bg-white/[0.045] px-5 py-4 mb-5">
      <img src="@/assets/image/logo.png" alt="" class="w-12 h-12 rounded-xl" />
      <div class="flex-1 min-w-0">
        <div class="text-xs text-app-muted mb-1">{{ t('update.version') }}</div>
        <div class="flex items-center gap-3 text-lg font-semibold tabular-nums">
          <span class="text-app-muted">{{ store.appInfo.Version }}</span>
          <template v-if="status.available">
            <span class="text-app-muted font-normal" aria-hidden="true">→</span>
            <span class="text-emerald-700 dark:text-emerald-400">{{ status.manifest?.version }}</span>
          </template>
        </div>
      </div>
      <NTag v-if="status.available" size="small" type="success" round :bordered="false">{{ t('update.stable') }}</NTag>
    </div>
    <template v-if="status.available">
      <div class="text-sm font-medium mb-2">{{ t('update.notes') }}</div>
      <NScrollbar style="max-height: 260px" class="mb-5">
        <div class="whitespace-pre-wrap break-words text-sm leading-7 pr-3 text-app-muted">{{ status.manifest?.notes || t('update.no_notes') }}</div>
      </NScrollbar>
    </template>
    <p v-if="status.state === 'checking'" role="status" aria-live="polite" class="text-sm text-app-muted mb-4">{{ t('update.checking') }}</p>
    <NAlert v-else-if="status.manifest && !status.available && status.state === 'available' && !failure && !status.error"
            type="success" :show-icon="false" class="mb-4">{{ t('update.up_to_date') }}</NAlert>
    <div v-if="status.state === 'downloading'" class="mb-4" role="status" aria-live="polite">
      <div class="flex justify-between text-xs text-app-muted mb-2">
        <span>{{ t('update.downloading') }} · {{ bytes(status.received) }} / {{ bytes(status.total) }}</span>
        <span>{{ bytes(status.speed) }}/s</span>
      </div>
      <NProgress type="line" :percentage="percentage" :show-indicator="false" :height="6" color="#177858" />
    </div>
    <NAlert v-if="status.slow" type="warning" :show-icon="false" class="mb-4">
      {{ t('update.slow') }}
      <NButton text type="primary" class="mt-2" @click="openWebsite">{{ t('update.website') }} ↗</NButton>
    </NAlert>
    <NAlert v-if="failure || status.error" type="error" :show-icon="false" class="mb-4">
      <div>{{ t((failure ? failureAction === 'check' : !!status.errorCode) ? 'update.check_failed' : 'update.failed') }}</div>
      <div class="text-xs break-words mt-1">{{ errorMessage }}</div>
    </NAlert>
    <NAlert v-if="busy" type="info" :show-icon="false" class="mb-4">{{ t('update.busy') }}</NAlert>
    <p v-if="status.available" class="text-xs leading-5 text-app-muted mb-0">{{ status.canInstall ? t('update.restart_hint') : t('update.manual_hint') }}</p>
    <template #footer>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <NButton text @click="openWebsite">{{ t('update.website') }} ↗</NButton>
        <div class="flex items-center gap-2">
          <NButton v-if="status.state === 'downloading'" :disabled="working" @click="cancel">{{ t('update.cancel') }}</NButton>
          <NButton @click="visible = false">{{ t('update.close') }}</NButton>
          <NButton v-if="canCheck || status.state === 'checking'" :type="status.available ? 'default' : 'primary'"
                   :loading="status.state === 'checking'" :disabled="working" @click="check()">{{ t('update.check') }}</NButton>
          <NButton v-if="status.state === 'error' && status.available && status.canInstall" :disabled="working" @click="download(true)">{{ t('update.direct') }}</NButton>
          <NButton v-if="status.available && status.canInstall" type="primary" :loading="working || status.state === 'installing'"
                   :disabled="working || status.state === 'checking' || status.state === 'downloading' || status.state === 'installing'"
                   @click="status.state === 'ready' ? install() : download(false)">
            {{ status.state === 'ready' ? t('update.restart') : status.state === 'installing' ? t('update.installing') : t('update.download') }}
          </NButton>
        </div>
      </div>
    </template>
  </NModal>
</template>

<script setup lang="ts">
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {NAlert, NButton, NModal, NProgress, NScrollbar, NTag} from 'naive-ui'
import {useIndexStore} from '@/stores'
import {useUpdateStore} from '@/stores/update'
import request from '@/api/request'
import {BrowserOpenURL} from '../../wailsjs/runtime'

type UpdateStatus = {
  state: string; available: boolean; canInstall: boolean; website: string
  received: number; total: number; speed: number; slow: boolean; error?: string
  errorCode?: string
  manifest?: {version: string; notes: string}
}

const emit = defineEmits<{available: [value: boolean]}>()
const {t} = useI18n()
const store = useIndexStore()
const updateUI = useUpdateStore()
const visible = ref(false)
const status = ref<UpdateStatus>({state: 'idle', available: false, canInstall: false, website: '', received: 0, total: 0, speed: 0, slow: false})
const failure = ref('')
const failureAction = ref<'check' | 'update'>('update')
const errorMessage = computed(() => {
  if (failure.value) return failure.value
  if (status.value.errorCode === 'metadata_unavailable') return t('update.metadata_unavailable')
  if (status.value.errorCode === 'check_network_failed') return t('update.check_network_failed')
  return status.value.error
})
const working = ref(false)
const busy = ref(false)
const canCheck = computed(() => !['checking', 'downloading', 'ready', 'installing'].includes(status.value.state))
let autoInstall = false
let stopped = false
let timer: ReturnType<typeof setTimeout> | undefined
const percentage = computed(() => status.value.total ? Math.min(100, status.value.received / status.value.total * 100) : 0)
const bytes = (value: number) => value >= 1024 * 1024 ? `${(value / 1024 / 1024).toFixed(1)} MB` : `${Math.round(value / 1024)} KB`

const call = async (action: string, direct = false): Promise<UpdateStatus & {busy?: boolean}> => {
  const result = await request({url: '/api/app/update', method: 'post', data: {action, direct}, timeout: 30000})
  if (result.code !== 1) throw new Error(result.message)
  return result.data
}

const accept = (next: UpdateStatus) => { status.value = next; emit('available', next.available) }
const openWebsite = () => { if (status.value.website) BrowserOpenURL(status.value.website) }
const check = async (manual = true) => {
  if (working.value || !canCheck.value) return
  working.value = true; busy.value = false
  if (manual) failure.value = ''
  try {
    if (manual && !await store.flushConfig()) return
    status.value = {...status.value, state: 'checking', error: '', errorCode: ''}
    accept(await call('check'))
    if (!manual && status.value.available) visible.value = true
  } catch (error) {
    status.value = {...status.value, state: 'error'}
    if (manual || !failure.value) { failure.value = String(error); failureAction.value = 'check' }
  } finally { working.value = false }
}

const download = async (direct: boolean) => {
  working.value = true; failure.value = ''; busy.value = false; failureAction.value = 'update'
  try {
    if (!await store.flushConfig()) return
    accept(await call('download', direct)); autoInstall = true
  }
  catch (error) { failure.value = String(error) }
  finally { working.value = false }
}

const cancel = async () => {
  working.value = true; autoInstall = false; failureAction.value = 'update'
  try { accept(await call('cancel')) } catch (error) { failure.value = String(error) }
  finally { working.value = false }
}

const install = async () => {
  autoInstall = false; working.value = true; failure.value = ''; failureAction.value = 'update'
  try {
    if (!await store.flushConfig()) return
    const next = await call('install')
    if (next.busy) { busy.value = true; visible.value = true }
    else { busy.value = false; accept(next) }
  } catch (error) { failure.value = String(error); visible.value = true }
  finally { working.value = false }
}

const poll = async () => {
  if (stopped) return
  try {
    if (!working.value) {
      const next = await call('status')
      if (!working.value) {
        accept(next)
        if (status.value.state === 'ready' && autoInstall) await install()
        if (status.value.state === 'error') autoInstall = false
      }
    }
  } catch { /* Transient local API errors are retried; installation may be shutting down. */ }
  if (!stopped) timer = setTimeout(poll, status.value.state === 'downloading' ? 1000 : 3000)
}

onMounted(async () => {
  try {
    accept(await call('status'))
    // Preserve a failed installation report even if the metadata network is unavailable.
    if (status.value.error && !status.value.errorCode) { failure.value = status.value.error; visible.value = true }
    await check(false)
  } catch (error) { failure.value = String(error); failureAction.value = 'check' }
  if (!stopped) void poll()
})

watch(() => updateUI.openRequest, () => { visible.value = true; void check() })
onUnmounted(() => { stopped = true; if (timer) clearTimeout(timer) })
</script>
