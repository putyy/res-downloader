<template>
  <NCard
      size="small"
      :bordered="false"
      class="app-card app-card--interactive task-card"
      :class="{'app-card--selected': selected}"
  >
    <div class="flex items-start gap-3">
      <div v-if="selectable" class="flex h-14 shrink-0 items-center">
        <NCheckbox
            :checked="selected"
            :aria-label="t('tasks.select_task', {title})"
            @update:checked="$emit('select', task, $event)"
        />
      </div>
      <div class="app-muted-surface relative flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-xl">
        <img v-if="task.resource?.coverUrl" :src="task.resource.coverUrl" class="h-full w-full object-cover" alt=""/>
        <NIcon v-else :size="26" class="app-muted-text">
          <DocumentOutline/>
        </NIcon>
        <NTag
            class="task-status-badge absolute left-0 top-0 z-10 w-full"
            :class="{'task-status-badge--on-cover': task.resource?.coverUrl}"
            size="small"
            :type="statusType"
            :bordered="false"
            :title="statusText"
        >
          {{ statusText }}
        </NTag>
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex items-start gap-3">
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-medium" :title="title">{{ title }}</div>
            <div class="app-muted-text mt-1 truncate text-xs" :title="metadataText">{{ metadataText }}</div>
          </div>
          <div class="flex shrink-0 flex-wrap items-center justify-end gap-1">
            <NButton v-if="canPause" size="tiny" quaternary type="warning" @click="$emit('pause', task)">
              {{ t('tasks.pause') }}
            </NButton>
            <NButton v-if="canResume" size="tiny" quaternary type="primary" @click="$emit('resume', task)">
              {{ t('tasks.resume') }}
            </NButton>
            <NButton v-if="task.recording && active" size="tiny" quaternary type="warning"
                     @click="$emit('stop-recording', task)">
              {{ t('tasks.stop_recording') }}
            </NButton>
            <NButton v-else-if="active" size="tiny" quaternary type="warning" @click="$emit('cancel', task)">
              {{ t('tasks.cancel') }}
            </NButton>
            <NButton v-if="retryable" size="tiny" quaternary @click="$emit('retry', task)">
              {{ t('tasks.retry') }}
            </NButton>
            <NButton v-if="task.outputPath" size="tiny" quaternary type="primary" @click="$emit('open', task)">
              {{ t('tasks.open') }}
            </NButton>
            <NPopconfirm v-if="!active" @positive-click="$emit('delete', task)">
              <template #trigger>
                <NButton size="tiny" quaternary type="error">{{ t('tasks.delete') }}</NButton>
              </template>
              {{ t('tasks.delete_confirm') }}
            </NPopconfirm>
          </div>
        </div>

        <div v-if="showProgress" class="mt-2">
          <NProgress
              type="line"
              :percentage="progressPercentage"
              :show-indicator="false"
              :status="task.state === 'completed' ? 'success' : 'default'"
          />
        </div>

        <div v-if="showProgress || task.outputPath" class="app-muted-text mt-1 flex min-w-0 items-center gap-3 text-xs">
          <span v-if="showProgress" class="shrink-0" :title="progressSummary">{{ progressSummary }}</span>
          <span
              v-if="task.outputPath"
              class="ml-auto min-w-0 truncate text-right"
              :title="task.outputPath"
          >
            {{ t('tasks.output') }}：{{ task.outputPath }}
          </span>
        </div>

        <NAlert v-if="task.error" type="error" :bordered="false" :show-icon="false" class="mt-2 break-all">
          <div class="ellipsis-2" :title="task.error">{{ task.error }}</div>
        </NAlert>
      </div>
    </div>
  </NCard>
</template>

<script lang="ts" setup>
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {DocumentOutline} from '@vicons/ionicons5'
import {useI18n} from 'vue-i18n'
import type {appType} from '@/types/app'
import {canPauseTask, canResumeTask, canRetryTask, isActiveTask} from '@/components/task/taskState'

const props = withDefaults(defineProps<{
  task: appType.DownloadTaskRecord
  selectable?: boolean
  selected?: boolean
}>(), {
  selectable: false,
  selected: false,
})

defineEmits<{
  (event: 'select', task: appType.DownloadTaskRecord, selected: boolean): void
  (event: 'pause', task: appType.DownloadTaskRecord): void
  (event: 'resume', task: appType.DownloadTaskRecord): void
  (event: 'cancel', task: appType.DownloadTaskRecord): void
  (event: 'stop-recording', task: appType.DownloadTaskRecord): void
  (event: 'retry', task: appType.DownloadTaskRecord): void
  (event: 'open', task: appType.DownloadTaskRecord): void
  (event: 'delete', task: appType.DownloadTaskRecord): void
}>()

const {t, locale} = useI18n()
const active = computed(() => isActiveTask(props.task))
const canPause = computed(() => canPauseTask(props.task))
const canResume = computed(() => canResumeTask(props.task))
const retryable = computed(() => canRetryTask(props.task))
const title = computed(() => props.task.resource?.title || t('tasks.unknown_resource'))
const showProgress = computed(() => active.value || props.task.state === 'completed')
const statusText = computed(() => t(props.task.state === 'failed' ? 'tasks.status_failed' : `tasks.${props.task.state}`))
const statusType = computed<'default' | 'info' | 'success' | 'warning' | 'error'>(() => {
  if (props.task.state === 'completed') return 'success'
  if (props.task.state === 'failed') return 'error'
  if (['paused', 'pausing', 'cancelled', 'interrupted'].includes(props.task.state)) return 'warning'
  return props.task.state === 'pending' ? 'default' : 'info'
})
const progressPercentage = computed(() => {
  if (props.task.state === 'completed') return 100
  const total = props.task.total || props.task.items?.length || 0
  if (!total) return 0
  return Math.min(100, Math.round(((props.task.downloaded ?? 0) / total) * 100))
})
const progressText = computed(() => {
  const downloaded = props.task.downloaded ?? 0
  const total = props.task.total || props.task.items?.length || 0
  if ((props.task.items?.length ?? 0) > 0) {
    return t('tasks.collection_progress', {done: downloaded, total})
  }
  return t('tasks.byte_progress', {done: formatBytes(downloaded), total: total > 0 ? formatBytes(total) : '-'})
})

const metadataText = computed(() => [
  t('tasks.created_at', {time: formatTime(props.task.createdAt)}),
  t('tasks.attempts', {count: props.task.attempts}),
  props.task.resumes ? t('tasks.resumes', {count: props.task.resumes}) : '',
  props.task.pluginId ? t('tasks.source', {plugin: props.task.pluginId}) : '',
].filter(Boolean).join(' · '))

const formatBytes = (value: number) => {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / Math.pow(1024, index)).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

const formatTime = (value: number) => new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
}).format(new Date(value))

const now = ref(Date.now())
let timer: number | undefined
const recordingDuration = computed(() => {
  if (!props.task.startedAt) return '00:00:00'
  const seconds = Math.max(0, Math.floor((now.value - props.task.startedAt) / 1000))
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remaining = seconds % 60
  return [hours, minutes, remaining].map(value => String(value).padStart(2, '0')).join(':')
})
const progressSummary = computed(() => {
  if (!props.task.recording || !active.value) return progressText.value
  return `${progressText.value} · ${t('tasks.recording_duration', {duration: recordingDuration.value})}`
})

onMounted(() => {
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 1000)
})
onUnmounted(() => {
  if (timer !== undefined) window.clearInterval(timer)
})
</script>

<style scoped>
.task-card.app-card--interactive:hover {
  transform: none;
}

.task-status-badge.n-tag {
  max-width: none;
  height: 18px;
  padding: 0 4px;
  justify-content: center;
  border-radius: 0;
  background-color: color-mix(in srgb, var(--n-text-color) 24%, transparent);
  backdrop-filter: blur(2px);
  font-size: 10px;
  pointer-events: none;
}

.task-status-badge--on-cover.n-tag {
  color: #fff;
  background-color: color-mix(in srgb, var(--n-text-color) 38%, rgba(0, 0, 0, 0.62));
  font-weight: 600;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.65);
}

.task-status-badge.n-tag :deep(.n-tag__content) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>