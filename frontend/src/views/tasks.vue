<template>
  <div class="flex h-full flex-col overflow-hidden p-5">
    <div class="mx-auto flex min-h-0 w-full flex-1 flex-col ">
      <section class="app-page-toolbar z-20 flex shrink-0 flex-col gap-4 pb-3 [&_.n-tabs-nav--top]:static">
        <NTabs v-model:value="activeTab" type="line" style="--wails-draggable:no-drag">
          <NTab name="all">{{ t('tasks.all') }} ({{ tasks.length }})</NTab>
          <NTab name="active">{{ t('tasks.active') }} ({{ activeCount }})</NTab>
          <NTab name="completed">{{ t('tasks.completed') }} ({{ completedCount }})</NTab>
          <NTab name="failed">{{ t('tasks.failed') }} ({{ incompleteCount }})</NTab>
          <template #suffix>
            <NSpace :wrap="false">
              <NButton secondary :type="batchMode ? 'primary' : 'default'" @click="toggleBatchMode">
                {{ batchMode ? t('tasks.finish_batch') : t('tasks.batch_manage') }}
              </NButton>
              <NButton secondary :loading="loading" @click="loadTasks">{{ t('tasks.refresh') }}</NButton>
              <NPopconfirm :disabled="finishedCount === 0" @positive-click="clearFinished">
                <template #trigger>
                  <NButton secondary type="error" :disabled="finishedCount === 0" :loading="clearing">
                    {{ t('tasks.clear_finished') }}
                  </NButton>
                </template>
                {{ t('tasks.clear_finished_confirm') }}
              </NPopconfirm>
            </NSpace>
          </template>
        </NTabs>

        <NCard
            v-if="batchMode"
            size="small"
            :bordered="false"
            class="app-card app-card--selected"
            style="--wails-draggable:no-drag"
        >
          <div class="flex flex-wrap items-center gap-3">
            <NCheckbox
                :checked="allFilteredSelected"
                :indeterminate="someFilteredSelected"
                :disabled="filteredTasks.length === 0"
                @update:checked="toggleSelectAll"
            >
              {{ t('tasks.select_current', {count: filteredTasks.length}) }}
            </NCheckbox>
            <NTag round :bordered="false" type="success">
              {{ t('tasks.selected_count', {count: selectedTasks.length}) }}
            </NTag>
            <div class="min-w-4 flex-1"/>
            <NButton
                size="small"
                secondary
                type="warning"
                :disabled="batchAction !== null || eligibleCount('pause') === 0"
                :loading="batchAction === 'pause'"
                @click="runBatchAction('pause')"
            >
              {{ t('tasks.batch_pause', {count: eligibleCount('pause')}) }}
            </NButton>
            <NButton
                size="small"
                secondary
                type="primary"
                :disabled="batchAction !== null || eligibleCount('resume') === 0"
                :loading="batchAction === 'resume'"
                @click="runBatchAction('resume')"
            >
              {{ t('tasks.batch_resume', {count: eligibleCount('resume')}) }}
            </NButton>
            <NButton
                size="small"
                secondary
                :disabled="batchAction !== null || eligibleCount('retry') === 0"
                :loading="batchAction === 'retry'"
                @click="runBatchAction('retry')"
            >
              {{ t('tasks.batch_retry', {count: eligibleCount('retry')}) }}
            </NButton>
            <NPopconfirm
                :disabled="batchAction !== null || eligibleCount('cancel') === 0"
                @positive-click="runBatchAction('cancel')"
            >
              <template #trigger>
                <NButton
                    size="small"
                    secondary
                    type="warning"
                    :disabled="batchAction !== null || eligibleCount('cancel') === 0"
                    :loading="batchAction === 'cancel'"
                >
                  {{ t('tasks.batch_cancel', {count: eligibleCount('cancel')}) }}
                </NButton>
              </template>
              {{ t('tasks.batch_cancel_confirm', {count: eligibleCount('cancel')}) }}
            </NPopconfirm>
            <NPopconfirm
                :disabled="batchAction !== null || eligibleCount('delete') === 0"
                @positive-click="runBatchAction('delete')"
            >
              <template #trigger>
                <NButton
                    size="small"
                    secondary
                    type="error"
                    :disabled="batchAction !== null || eligibleCount('delete') === 0"
                    :loading="batchAction === 'delete'"
                >
                  {{ t('tasks.batch_delete', {count: eligibleCount('delete')}) }}
                </NButton>
              </template>
              {{ t('tasks.batch_delete_confirm', {count: eligibleCount('delete')}) }}
            </NPopconfirm>
          </div>
          <div v-if="selectedTasks.some(task => task.recording && isActiveTask(task))"
               class="app-muted-text mt-2 text-xs">
            {{ t('tasks.recording_batch_tip') }}
          </div>
        </NCard>
      </section>

      <div class="min-h-0 flex-1 overflow-y-auto pt-1 [&::-webkit-scrollbar]:hidden">
        <NSpin :show="loading">
          <NEmpty v-if="!loading && filteredTasks.length === 0" :description="t('tasks.empty')" class="py-16"/>
          <div v-else class="space-y-4" style="--wails-draggable:no-drag">
            <TaskListItem
                v-for="task in filteredTasks"
                :key="task.id"
                :task="task"
                :selectable="batchMode"
                :selected="selectedIds.includes(task.id)"
                @select="selectTask"
                @pause="pauseTask"
                @resume="resumeTask"
                @cancel="cancelTask"
                @stop-recording="stopRecording"
                @retry="retryTask"
                @open="openTask"
                @delete="deleteTask"
            />
          </div>
        </NSpin>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import appApi from '@/api/app'
import type {appType} from '@/types/app'
import {useEventStore} from '@/stores/event'
import TaskListItem from '@/components/task/TaskListItem.vue'
import {canCancelTask, canDeleteTask, canPauseTask, canResumeTask, canRetryTask, isActiveTask,} from '@/components/task/taskState'

const {t} = useI18n()
const eventStore = useEventStore()
const tasks = ref<appType.DownloadTaskRecord[]>([])
const activeTab = ref('all')
const loading = ref(false)
const clearing = ref(false)
const batchMode = ref(false)
const selectedIds = ref<string[]>([])
const batchAction = ref<appType.DownloadTaskBatchAction | null>(null)
const disposers: (() => void)[] = []

const activeCount = computed(() => tasks.value.filter(isActiveTask).length)
const completedCount = computed(() => tasks.value.filter(task => task.state === 'completed').length)
const incompleteCount = computed(() => tasks.value.filter(task => ['failed', 'cancelled', 'interrupted'].includes(task.state)).length)
const finishedCount = computed(() => tasks.value.filter(canDeleteTask).length)
const filteredTasks = computed(() => {
  if (activeTab.value === 'active') return tasks.value.filter(isActiveTask)
  if (activeTab.value === 'completed') return tasks.value.filter(task => task.state === 'completed')
  if (activeTab.value === 'failed') return tasks.value.filter(task => ['failed', 'cancelled', 'interrupted'].includes(task.state))
  return tasks.value
})
const selectedTasks = computed(() => tasks.value.filter(task => selectedIds.value.includes(task.id)))
const filteredTaskIds = computed(() => filteredTasks.value.map(task => task.id))
const selectedFilteredCount = computed(() => filteredTaskIds.value.filter(id => selectedIds.value.includes(id)).length)
const allFilteredSelected = computed(() => filteredTaskIds.value.length > 0 && selectedFilteredCount.value === filteredTaskIds.value.length)
const someFilteredSelected = computed(() => selectedFilteredCount.value > 0 && !allFilteredSelected.value)

const eligibility: Record<appType.DownloadTaskBatchAction, (task: appType.DownloadTaskRecord) => boolean> = {
  pause: canPauseTask,
  resume: canResumeTask,
  cancel: canCancelTask,
  retry: canRetryTask,
  delete: canDeleteTask,
}
const eligibleTasks = (action: appType.DownloadTaskBatchAction) => selectedTasks.value.filter(eligibility[action])
const eligibleCount = (action: appType.DownloadTaskBatchAction) => eligibleTasks(action).length

const sortTasks = () => tasks.value.sort((left, right) => {
  const createdAtOrder = right.createdAt - left.createdAt
  return createdAtOrder || left.id.localeCompare(right.id)
})
const upsertTask = (task: appType.DownloadTaskRecord) => {
  const index = tasks.value.findIndex(item => item.id === task.id)
  if (index < 0) tasks.value.push(task)
  else tasks.value[index] = task
  sortTasks()
}

const loadTasks = async () => {
  loading.value = true
  try {
    const response = await appApi.downloadTasks() as appType.Res<appType.DownloadTaskRecord[]>
    if (response.code !== 1) throw new Error(response.message)
    tasks.value = response.data ?? []
    sortTasks()
    selectedIds.value = selectedIds.value.filter(id => tasks.value.some(task => task.id === id))
  } catch (error: any) {
    showError(error)
  } finally {
    loading.value = false
  }
}

const toggleBatchMode = () => {
  batchMode.value = !batchMode.value
  selectedIds.value = []
}

const selectTask = (task: appType.DownloadTaskRecord, selected: boolean) => {
  if (selected) {
    if (!selectedIds.value.includes(task.id)) selectedIds.value = [...selectedIds.value, task.id]
    return
  }
  selectedIds.value = selectedIds.value.filter(id => id !== task.id)
}

const toggleSelectAll = (selected: boolean) => {
  const visible = new Set(filteredTaskIds.value)
  if (selected) {
    selectedIds.value = Array.from(new Set([...selectedIds.value, ...filteredTaskIds.value]))
    return
  }
  selectedIds.value = selectedIds.value.filter(id => !visible.has(id))
}

const runTaskAction = async (action: () => Promise<any>) => {
  try {
    const response = await action()
    if (response.code !== 1) throw new Error(response.message)
    return response
  } catch (error: any) {
    showError(error)
    return undefined
  }
}

const cancelTask = async (task: appType.DownloadTaskRecord) => {
  await runTaskAction(() => appApi.cancelDownloadTask(task.id))
}

const pauseTask = async (task: appType.DownloadTaskRecord) => {
  const response = await runTaskAction(() => appApi.pauseDownloadTask(task.id))
  if (response?.data) upsertTask(response.data)
}

const resumeTask = async (task: appType.DownloadTaskRecord) => {
  const response = await runTaskAction(() => appApi.resumeDownloadTask(task.id))
  if (response?.data) upsertTask(response.data)
}

const stopRecording = async (task: appType.DownloadTaskRecord) => {
  await runTaskAction(() => appApi.stopRecordingTask(task.id))
}

const retryTask = async (task: appType.DownloadTaskRecord) => {
  const response = await runTaskAction(() => appApi.retryDownload(task.id))
  if (response?.data) upsertTask(response.data)
}

const openTask = async (task: appType.DownloadTaskRecord) => {
  if (!task.outputPath) return
  await runTaskAction(() => appApi.openFolder({filePath: task.outputPath}))
}

const deleteTask = async (task: appType.DownloadTaskRecord) => {
  const response = await runTaskAction(() => appApi.deleteDownloadTask(task.id))
  if (response) tasks.value = tasks.value.filter(item => item.id !== task.id)
}

const clearFinished = async () => {
  clearing.value = true
  const response = await runTaskAction(() => appApi.clearDownloadTasks())
  clearing.value = false
  if (!response) return
  await loadTasks()
  window.$message?.success(t('tasks.clear_finished_success', {count: response.data?.count ?? 0}))
}

const runBatchAction = async (action: appType.DownloadTaskBatchAction) => {
  const targets = eligibleTasks(action)
  if (targets.length === 0 || batchAction.value) return
  const skipped = selectedTasks.value.length - targets.length
  batchAction.value = action
  const response = await runTaskAction(() => appApi.batchDownloadTasks(targets.map(task => task.id), action)) as appType.Res<appType.DownloadTaskBatchResponse> | undefined
  batchAction.value = null
  if (!response) return

  const succeeded = response.data?.succeeded ?? 0
  const failed = response.data?.failed ?? 0
  selectedIds.value = []
  await loadTasks()
  const message = t('tasks.batch_result', {succeeded, skipped, failed})
  if (failed > 0 || skipped > 0) window.$message?.warning(message, {duration: 5000})
  else window.$message?.success(message)
}

const showError = (error: any) => {
  const message = error?.message ?? String(error)
  window.$message?.error(t('tasks.operation_failed', {message}), {duration: 5000})
}

onMounted(() => {
  loadTasks()
  disposers.push(eventStore.addHandle({type: 'downloadTaskUpdated', event: upsertTask}))
  disposers.push(eventStore.addHandle({
    type: 'downloadTaskRemoved',
    event: (data: { id: string }) => {
      tasks.value = tasks.value.filter(task => task.id !== data.id)
    },
  }))
})

watch(activeTab, () => {
  selectedIds.value = []
})
onUnmounted(() => disposers.splice(0).forEach(dispose => dispose()))
</script>
