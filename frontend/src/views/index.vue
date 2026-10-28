<template>
  <div class="h-full flex flex-col px-5 pt-5 overflow-y-auto [&::-webkit-scrollbar]:hidden">
    <NAlert v-if="route.query.resourceId" type="info" class="mb-3 [--wails-draggable:no-drag]"
      >{{ t("operations.linked_filter") }} · {{ route.query.resourceId }} <NButton text @click="router.replace('/index')">{{ t("operations.clear_filter") }}</NButton></NAlert
    >
    <div class="pb-2 z-40" id="header">
      <NSpace>
        <NButton v-if="isProxy" secondary type="primary" @click.stop="close" class="[--wails-draggable:no-drag]">
          <span class="inline-block w-1.5 h-1.5 bg-red-600 rounded-full mr-1 animate-pulse"></span>
          {{ t("index.close_grab") }}{{ resourceTotal > 0 ? `&nbsp;${t("index.total_resources", {count: resourceTotal})}` : "" }}
        </NButton>
        <NButton v-else tertiary type="tertiary" @click.stop="open" class="[--wails-draggable:no-drag]">
          {{ t("index.open_grab") }}{{ resourceTotal > 0 ? `&nbsp;${t("index.total_resources", {count: resourceTotal})}` : "" }}
        </NButton>
        <NSelect
          class="min-w-[100px] [--wails-draggable:no-drag]"
          :placeholder="t('index.grab_type')"
          :value="resourcesType"
          multiple
          clearable
          :max-tag-count="3"
          :options="captureTypeOptions"
          @update:value="updateResourceTypes"
        ></NSelect>
        <NButtonGroup class="[--wails-draggable:no-drag]">
          <NButton v-if="rememberChoice" tertiary type="error" @click.stop="clear" class="[--wails-draggable:no-drag]">
            <template #icon>
              <n-icon>
                <TrashOutline />
              </n-icon>
            </template>
            {{ t("index.clear_list") }}
          </NButton>
          <n-popconfirm
            v-else
            @positive-click="
              () => {
                rememberChoice = rememberChoiceTmp
                clear()
              }
            "
            :show-icon="false"
          >
            <template #trigger>
              <NButton tertiary type="error" class="[--wails-draggable:no-drag]">
                <template #icon>
                  <n-icon>
                    <TrashOutline />
                  </n-icon>
                </template>
                {{ t("index.clear_list") }}
              </NButton>
            </template>
            <div>
              <div class="flex flex-row items-center text-red-700 my-2 text-base">
                <n-icon>
                  <TrashOutline />
                </n-icon>
                <p class="ml-1">{{ t("index.clear_list_tip") }}</p>
              </div>
              <NCheckbox v-model:checked="rememberChoiceTmp">
                <span class="text-app-muted">{{ t("index.remember_clear_choice") }}</span>
              </NCheckbox>
            </div>
          </n-popconfirm>

          <NButton tertiary type="primary" @click.stop="batchDown">
            <template #icon>
              <n-icon>
                <DownloadOutline />
              </n-icon>
            </template>
            {{ t("index.batch_download") }}
          </NButton>
          <NButton tertiary type="info">
            <NPopover placement="bottom" trigger="hover">
              <template #trigger>
                <NIcon size="18" class="">
                  <Apps />
                </NIcon>
              </template>
              <div class="flex flex-col">
                <NButton tertiary type="error" @click.stop="batchCancel" class="my-1">
                  <template #icon>
                    <n-icon>
                      <CloseOutline />
                    </n-icon>
                  </template>
                  {{ t("index.cancel_down") }}
                </NButton>
                <NButton tertiary type="warning" @click.stop="batchExport()" class="my-1">
                  <template #icon>
                    <n-icon>
                      <ArrowRedoCircleOutline />
                    </n-icon>
                  </template>
                  {{ t("index.batch_export") }}
                </NButton>
                <NButton tertiary type="info" @click.stop="showImport = true" class="my-1">
                  <template #icon>
                    <n-icon>
                      <ServerOutline />
                    </n-icon>
                  </template>
                  {{ t("index.batch_import") }}
                </NButton>
                <NButton tertiary type="primary" @click.stop="batchExport('url')" class="my-1">
                  <template #icon>
                    <n-icon>
                      <ArrowRedoCircleOutline />
                    </n-icon>
                  </template>
                  {{ t("index.export_url") }}
                </NButton>
              </div>
            </NPopover>
          </NButton>
        </NButtonGroup>
        <NButton v-if="nextResourceOffset > 0" secondary :loading="loadingMoreResources" @click="loadMoreResources">
          {{ t("index.load_more", {loaded: data.length, total: resourceTotal}) }}
        </NButton>
      </NSpace>
    </div>
    <div class="min-h-0 flex-1">
      <NDataTable
        class="resource-table [--wails-draggable:no-drag]"
        :columns="columns"
        :data="filteredData"
        children-key="children"
        :indent="0"
        :cascade="false"
        :bordered="false"
        :max-height="tableHeight"
        :row-key="rowKey"
        virtual-scroll
        :header-height="48"
        :min-row-height="48"
        :checked-row-keys="checkedRowKeysValue"
        :expanded-row-keys="expandedRowKeys"
        :row-class-name="resourceRowClassName"
        @update:checked-row-keys="handleCheck"
        @update:expanded-row-keys="(keys: any) => (expandedRowKeys = keys)"
        @update:filters="updateFilters"
      />
    </div>
    <div
      class="text-app-muted [&_span]:transition-colors [&_span]:duration-[160ms] [&_span]:ease-[ease] [&_span:hover]:text-app-accent flex items-center justify-center"
      id="bottom"
    >
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="Browser.OpenURL(certUrl)">{{ t("footer.cert_download") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="openDocumentation('home')">{{ t("footer.documentation") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="Browser.OpenURL('https://github.com/putyy/res-downloader')">{{ t("footer.source_code") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="Browser.OpenURL('https://github.com/putyy/res-downloader/issues')">{{ t("footer.help") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="Browser.OpenURL('https://github.com/putyy/res-downloader/releases')">{{
        t("footer.update_log")
      }}</span>
    </div>
    <Preview v-model:showModal="showPreviewRow" :previewRow="previewRow" />
    <ShowLoading :loadingText="loadingText" :isLoading="loading" />
    <ImportJson v-model:showModal="showImport" @submit="handleImport" />
    <Password v-model:showModal="showPassword" @submit="handlePassword" />
    <OperationPagePicker v-model:show="operationPickerVisible" :sessions="operationPickerSessions" :loading="operationPickerLoading" @submit="submitSelectedOperationPage" />
  </div>
</template>

<script lang="ts" setup>
import type {DataTableFilterState, DataTableRowKey} from "naive-ui"
import {NButton, NDataTable, NIcon, NPopover, NSpace} from "naive-ui"
import {computed, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch} from "vue"
import type {appType} from "@/types/app"
import Preview from "@/components/Preview.vue"
import ShowLoading from "@/components/ShowLoading.vue"
import {useIndexStore} from "@/stores"
import appApi from "@/api/app"
import {openDocumentation} from "@/services/documentation"
import operationsApi, {OperationAPIError} from "@/api/operations"
import {executionActive, operationErrorMessage, operationPageUnavailableMessage} from "@/services/operations"
import type {OperationDefinition, OperationExecution, OperationSession} from "@/types/operations"
import OperationPagePicker from "@/components/resource/OperationPagePicker.vue"
import {useRouter, useRoute} from "vue-router"
import {useResourceTableColumns} from "@/components/resource/useResourceTableColumns"
import {exportableResource, findResourceInTree, mergeResource, primaryURL, removeResourceFromTree, resourceSome, selectedResourceRoots, visitResource} from "@/services/resources"
import {downloadTaskProgress, formatDownloadBytes} from "@/services/downloadProgress"
import ImportJson from "@/components/ImportJson.vue"
import {useEventStore} from "@/stores/event"
import {useDownloadStore} from "@/stores/download"
import {activeTaskStates} from "@/components/task/taskState"
import {Browser, Clipboard} from "@wailsio/runtime"
import Password from "@/components/Password.vue"
import {useI18n} from "vue-i18n"
import {Apps, ArrowRedoCircleOutline, CloseOutline, DownloadOutline, ServerOutline, TrashOutline} from "@vicons/ionicons5"
import {useCertificateStore} from "@/stores/certificate"
import type {MessageReactive} from "naive-ui"

const {t, locale} = useI18n()
const eventStore = useEventStore()
const downloadStore = useDownloadStore()
const isProxy = computed(() => {
  return store.isProxy
})
const certUrl = computed(() => {
  return store.baseUrl + "/api/certificate/download"
})
const data = ref<appType.ResourceView[]>([])
const resourceTotal = ref(0)
const nextResourceOffset = ref(0)
const loadingMoreResources = ref(false)
const resourcePageSize = 1000
const resourceWarningStart = 1500
const resourceWarningStep = 500
let nextResourceWarningAt = resourceWarningStart
let resourceWarning: MessageReactive | undefined

const updateResourceRecordCount = (value: unknown, afterCleanup = false) => {
  const count = Number(value)
  if (!Number.isFinite(count) || count < 0) return
  const nextThreshold = Math.max(resourceWarningStart, (Math.floor(count / resourceWarningStep) + 1) * resourceWarningStep)
  if (afterCleanup) {
    nextResourceWarningAt = nextThreshold
    resourceWarning?.destroy()
    resourceWarning = undefined
    return
  }
  if (count < nextResourceWarningAt) return
  // A large batch gets one warning; repeated updates of the same count do not.
  nextResourceWarningAt = nextThreshold
  resourceWarning?.destroy()
  resourceWarning = window.$message?.warning(t("index.resource_count_warning", {count}), {
    duration: 10000,
    closable: true,
  })
}
const filterKinds = ref<string[]>([])
const resourceDepths = computed(() => {
  const depths = new Map<string, number>()
  const collect = (item: appType.ResourceView, depth: number) => {
    depths.set(item.id, depth)
    ;(item.children ?? []).forEach((child) => collect(child, depth + 1))
  }
  data.value.forEach((root) => collect(root, 0))
  return depths
})
const filteredData = computed(() => {
  let result = data.value
  const resourceId = String(route.query.resourceId || "")
  if (resourceId) return result.filter((item) => resourceSome(item, (child) => child.id === resourceId))

  if (filterKinds.value.length > 0) {
    result = result.filter((item) =>
      resourceSome(item, (child) => (!!child.primaryType && filterKinds.value.includes(child.primaryType)) || (!!child.kind && filterKinds.value.includes(child.kind))),
    )
  }

  if (descriptionSearchValue.value) {
    const expected = descriptionSearchValue.value.toLowerCase()
    result = result.filter((item) => resourceSome(item, (child) => !!child.title?.toLowerCase().includes(expected)))
  }

  if (urlSearchValue.value) {
    const expected = urlSearchValue.value.toLowerCase()
    result = result.filter((item) => resourceSome(item, (child) => primaryURL(child).toLowerCase().includes(expected)))
  }

  return result
})

const store = useIndexStore()
const certificateStore = useCertificateStore()
const tableHeight = ref(800)
const resourcesType = ref<string[]>(["all"])
const pluginResourceKinds = ref<appType.ResourceKindDefinition[]>([])
const pluginActionDefinitions = ref<Record<string, Record<string, appType.PluginActionDefinition>>>({})
const pluginOperationDefinitions = ref<Record<string, Record<string, OperationDefinition>>>({})

const classifyAlias: Record<string, string> = {
  "media.image": "image",
  "media.audio": "audio",
  "media.video": "video",
  "media.collection": "collection",
  "stream.hls": "m3u8",
  "stream.live": "live",
  "document.xls": "xls",
  "document.doc": "doc",
  "document.pdf": "pdf",
  image: "image",
  audio: "audio",
  video: "video",
  m3u8: "m3u8",
  live: "live",
  xls: "xls",
  doc: "doc",
  pdf: "pdf",
  stream: "stream",
  font: "font",
}

const dwStatus = computed<any>(() => {
  return {
    ready: t("index.ready"),
    partial: t("index.partial"),
    pending: t("index.pending"),
    running: t("index.running"),
    error: t("index.error"),
    done: t("index.done"),
    handle: t("index.handle"),
  }
})

const descriptionSearchValue = ref("")
const urlSearchValue = ref("")
const rememberChoice = ref(false)
const rememberChoiceTmp = ref(false)

const checkedRowKeysValue = ref<DataTableRowKey[]>([])
const expandedRowKeys = ref<DataTableRowKey[]>([])
const showPreviewRow = ref(false)
const previewRow = ref<appType.ResourceView>()
const loading = ref(false)
const loadingText = ref("")
const showImport = ref(false)
const showPassword = ref(false)
const proxyAction = ref<"enable" | "disable">("enable")
const disposers: Array<() => void> = []
const router = useRouter()
const route = useRoute()
const executions = ref<Record<string, OperationExecution>>({})
let executionPoll: ReturnType<typeof setTimeout> | undefined
let executionPolling = false
let executionRefreshing = false
let executionPollGeneration = 0
let executionSubmission = 0
const refreshExecutions = async () => {
  if (!executionPolling || executionRefreshing) return
  executionRefreshing = true
  const generation = executionPollGeneration
  const submission = executionSubmission
  const current = () => executionPolling && generation === executionPollGeneration
  try {
    const ids = new Set<string>()
    for (const root of filteredData.value)
      visitResource(root, (row) => {
        ids.add(row.id)
      })
    const resourceIds = [...ids]
    const latest: Record<string, OperationExecution> = {}
    for (let offset = 0; offset < resourceIds.length; offset += 1000) {
      const items = await operationsApi.resources(resourceIds.slice(offset, offset + 1000))
      if (!current()) return
      for (const execution of items) {
        if (execution.resourceId) latest[execution.resourceId] = execution
      }
    }
    // A response started before a resource action must not erase its new ID.
    if (current() && submission === executionSubmission) executions.value = latest
  } catch {
    if (current()) for (const execution of Object.values(executions.value)) execution.syncUnavailable = true
  } finally {
    executionRefreshing = false
    if (executionPolling) {
      if (Date.now() - pluginMetadataAttemptedAt >= 15000) void refreshPluginMetadata()
      executionPoll = setTimeout(refreshExecutions, current() ? 2000 : 0)
    }
  }
}
const stopExecutionPolling = () => {
  executionPolling = false
  executionPollGeneration++
  clearTimeout(executionPoll)
}
const updateExecutionPolling = () => {
  if (document.hidden || route.path !== "/index") {
    stopExecutionPolling()
  } else if (!executionPolling) {
    executionPolling = true
    pluginMetadataAttemptedAt = 0
    void refreshExecutions()
  }
}
const handleWindowResize = () => resetTableHeight()

let pluginMetadataGeneration = 0
let pluginMetadataAttemptedAt = 0
let pluginMetadataLoading: Promise<void> | undefined
const refreshPluginMetadata = (): Promise<void> => {
  if (pluginMetadataLoading) return pluginMetadataLoading
  const generation = ++pluginMetadataGeneration
  pluginMetadataAttemptedAt = Date.now()
  pluginMetadataLoading = (async () => {
    try {
      const res = (await appApi.plugins()) as appType.Res
      if (res.code !== 1 || generation !== pluginMetadataGeneration) return
      const plugins: appType.PluginStatus[] = res.data.plugins ?? []
      const loadedPlugins = plugins.filter((plugin) => plugin.loaded)
      pluginResourceKinds.value = loadedPlugins.flatMap((plugin) => plugin.manifest.resourceKinds ?? [])
      pluginActionDefinitions.value = Object.fromEntries(plugins.map((plugin) => [plugin.manifest.id, plugin.manifest.actions ?? {}]))
      pluginOperationDefinitions.value = Object.fromEntries(plugins.map((plugin) => [plugin.manifest.id, plugin.manifest.operations ?? {}]))
      removeUnavailableResourceTypes()
    } catch {
      // Keep current labels; retry while the resource list remains open.
    } finally {
      pluginMetadataLoading = undefined
    }
  })()
  return pluginMetadataLoading
}

// The home route is kept alive. Installation/reload can change action definitions
// without remounting it, so refresh both initially and when returning to the page.
onActivated(updateExecutionPolling)
onDeactivated(stopExecutionPolling)

onMounted(() => {
  document.addEventListener("visibilitychange", updateExecutionPolling)
  updateExecutionPolling()
  try {
    window.addEventListener("resize", handleWindowResize)
  } catch (e) {
    window.$message?.error(JSON.stringify(e), {duration: 5000})
  }

  restoreResourceTypes()
  void downloadStore.load().catch((error) => window.$message?.error(String(error.message ?? error)))

  appApi.listResources({offset: 0, limit: resourcePageSize}).then((res: appType.Res) => {
    if (res.code !== 1) {
      window?.$message?.error(res.message)
      return
    }
    appendResourcePage(res.data?.items ?? [])
    resourceTotal.value = Number(res.data?.total ?? data.value.length)
    updateResourceRecordCount(res.data?.recordCount)
    nextResourceOffset.value = Number(res.data?.nextOffset ?? 0)
  })
  if (store.globalConfig.AutoProxy) void open()
  const choiceCache = localStorage.getItem("remember-clear-choice")
  if (choiceCache === "1") {
    rememberChoice.value = true
  }

  disposers.push(
    watch(rememberChoice, () => {
      if (rememberChoice.value) {
        localStorage.setItem("remember-clear-choice", "1")
      } else {
        localStorage.removeItem("remember-clear-choice")
      }
    }),
  )

  resetTableHeight()

  disposers.push(
    eventStore.addHandle({
      type: "resourceAdded",
      event: (res: appType.ResourceView) => {
        const exists = data.value.some((item) => item.id === res.id)
        upsertResourceRoot(res)
        if (!exists) resourceTotal.value++
      },
    }),
  )

  disposers.push(
    eventStore.addHandle({
      type: "resourceUpdated",
      event: (res: appType.ResourceView) => {
        upsertResourceRoot(res)
      },
    }),
  )

  disposers.push(
    eventStore.addHandle({
      type: "resourcesBatch",
      event: (payload: {items?: appType.ResourceView[]; total?: number; recordCount?: number}) => {
        for (const resource of payload?.items ?? []) upsertResourceRoot(resource)
        const nextTotal = Number(payload?.total ?? Math.max(resourceTotal.value, data.value.length))
        if (nextResourceOffset.value > 0 && nextTotal > resourceTotal.value) {
          nextResourceOffset.value += nextTotal - resourceTotal.value
        }
        resourceTotal.value = nextTotal
        updateResourceRecordCount(payload?.recordCount)
      },
    }),
  )

  disposers.push(
    eventStore.addHandle({
      type: "resourceActionProgress",
      event: (res: {status: string; outputPath?: string; message?: string}) => {
        if (res.status === "done") {
          window?.$message?.success(t("index.plugin_action_done", {path: res.outputPath || ""}), {
            duration: 5000,
          })
        } else if (res.status === "error") {
          window?.$message?.error(t("index.plugin_action_failed", {message: res.message || ""}), {
            duration: 5000,
          })
        }
      },
    }),
  )
})

onUnmounted(() => {
  pluginMetadataGeneration++
  stopExecutionPolling()
  document.removeEventListener("visibilitychange", updateExecutionPolling)
  window.removeEventListener("resize", handleWindowResize)
  disposers.splice(0).forEach((dispose) => dispose())
  resourceWarning?.destroy()
})

const loadMoreResources = async () => {
  if (nextResourceOffset.value <= 0 || loadingMoreResources.value) return
  loadingMoreResources.value = true
  try {
    const response = (await appApi.listResources({
      offset: nextResourceOffset.value,
      limit: resourcePageSize,
    })) as appType.Res
    if (response.code !== 1) {
      window.$message?.error(response.message)
      return
    }
    appendResourcePage(response.data?.items ?? [])
    resourceTotal.value = Number(response.data?.total ?? data.value.length)
    updateResourceRecordCount(response.data?.recordCount)
    nextResourceOffset.value = Number(response.data?.nextOffset ?? 0)
  } finally {
    loadingMoreResources.value = false
  }
}

// A linked operation result can refer to an older page or a collection child.
watch(
  [() => route.query.resourceId, () => data.value.length, nextResourceOffset],
  async () => {
    const id = String(route.query.resourceId || "")
    if (!id || route.path !== "/index") return
    const root = data.value.find((item) => resourceSome(item, (child) => child.id === id))
    if (root) {
      const expandAncestors = (item: appType.ResourceView): boolean => {
        if (item.id === id) return true
        if (!(item.children ?? []).some(expandAncestors)) return false
        if (!expandedRowKeys.value.includes(item.id)) expandedRowKeys.value.push(item.id)
        return true
      }
      expandAncestors(root)
      return
    }
    if (nextResourceOffset.value > 0 && !loadingMoreResources.value) {
      try {
        await loadMoreResources()
      } catch {
        window.$message?.error(t("operations.connection_error"))
      }
    }
  },
  {flush: "post"},
)

const upsertResourceRoot = (resource: appType.ResourceView) => {
  const index = data.value.findIndex((item) => item.id === resource.id)
  if (index >= 0) {
    data.value[index] = mergeResource(resource, data.value[index])
    return
  }
  if (store.globalConfig.InsertTail) data.value.push(resource)
  else data.value.unshift(resource)
}

const appendResourcePage = (resources: appType.ResourceView[]) => {
  for (const resource of resources) {
    const index = data.value.findIndex((item) => item.id === resource.id)
    if (index >= 0) data.value[index] = mergeResource(resource, data.value[index])
    else data.value.push(resource)
  }
}

watch(resourcesType, () => {
  localStorage.setItem("resource-kind-filter", JSON.stringify({res: resourcesType.value}))
  appApi.setResourceFilter(resourcesType.value)
})

const updateDescription = async (id: string, value: string) => {
  const item = findResource(id)
  if (!item || item.title === value) return

  const previousValue = item.title || ""
  item.title = value
  try {
    const response = (await appApi.updateResource({id, title: value})) as appType.Res<{
      id: string
      title: string
    }>
    if (response.code !== 1) throw new Error(response.message)
  } catch (error: any) {
    if (item.title === value) item.title = previousValue
    window.$message?.error(t("index.description_save_failed", {message: error?.message || String(error)}))
  }
}

const downloadState = (row: appType.ResourceView): appType.ResourceDownloadState | undefined => {
  const task = downloadStore.resourceTasks.get(row.id)
  if (downloadStore.creating.has(row.id) && (!task || !activeTaskStates.includes(task.state))) return {state: "pending", message: t("index.pending")}
  if (!task) return undefined
  const progress = downloadTaskProgress(task)
  const message =
    task.error ||
    (task.state === "paused"
      ? t("tasks.paused")
      : (task.items?.length ?? 0) > 0
        ? `${progress.downloaded}/${progress.total}`
        : progress.total > 0
          ? `${Math.floor(progress.percentage)}%`
          : formatDownloadBytes(progress.downloaded))
  return {
    createdAt: task.createdAt,
    startedAt: task.startedAt,
    taskId: task.id,
    state: task.state,
    outputPath: task.outputPath || "",
    message,
    downloaded: task.downloaded,
    total: task.total,
  }
}

const isActiveDownload = (row: appType.ResourceView) => activeTaskStates.some((state) => state === downloadState(row)?.state)

const resourcesBlockedByDownloads = computed(() => {
  const blocked = new Set<string>()
  const collect = (row: appType.ResourceView, ancestorActive: boolean): boolean => {
    const active = isActiveDownload(row)
    let subtreeActive = active
    for (const child of row.children ?? []) {
      if (collect(child, ancestorActive || active)) subtreeActive = true
    }
    // Collection tasks protect descendants; a child's task protects its ancestors.
    if (ancestorActive || subtreeActive) blocked.add(row.id)
    return subtreeActive
  }
  data.value.forEach((root) => collect(root, false))
  return blocked
})
const canDeleteResource = (row: appType.ResourceView) => !resourcesBlockedByDownloads.value.has(row.id)

const findResource = (id: string): appType.ResourceView | undefined => {
  return findResourceInTree(data.value, id)
}

const removeResource = (id: string) => {
  const resource = findResource(id)
  const removedIds = new Set<string>()
  if (resource) visitResource(resource, (item) => removedIds.add(item.id))
  if (removeResourceFromTree(data.value, id)) {
    expandedRowKeys.value = expandedRowKeys.value.filter((key) => !removedIds.has(String(key)))
    checkedRowKeysValue.value = checkedRowKeysValue.value.filter((key) => !removedIds.has(String(key)))
  }
}

const resetTableHeight = () => {
  try {
    const headerHeight = document.getElementById("header")?.offsetHeight || 0
    const bottomHeight = document.getElementById("bottom")?.offsetHeight || 0
    // @ts-ignore
    const theadHeight = document.getElementsByClassName("n-data-table-thead")[0]?.offsetHeight || 0
    const height = document.documentElement.clientHeight || window.innerHeight
    tableHeight.value = height - headerHeight - bottomHeight - theadHeight - 20
  } catch (e) {
    console.log(e)
  }
}

const resourceTypeOptions = computed(() => {
  const allOption = {value: "all", label: t("index.all")}
  const primaryOptions = [
    {value: "video", label: t("index.video")},
    {value: "audio", label: t("index.audio")},
    {value: "image", label: t("index.image")},
    {value: "document", label: t("index.document")},
    {value: "archive", label: t("index.archive")},
    {value: "collection", label: t("index.collection")},
    {value: "other", label: t("index.other")},
  ]
  const primaryKindAliases = new Set(["media.video", "media.audio", "media.image", "media.collection", "document.text"])
  const seen = new Set<string>(["all", ...primaryOptions.map((option) => option.value)])
  const detailedOptions = pluginResourceKinds.value.flatMap((definition) => {
    if (!definition.id || primaryKindAliases.has(definition.id) || seen.has(definition.id)) return []
    seen.add(definition.id)
    return [{value: definition.id, label: localizedResourceKindName(definition)}]
  })
  return {
    classify: [allOption, ...primaryOptions, ...detailedOptions],
    capture: [
      allOption,
      {
        type: "group",
        key: "primary-types",
        label: t("index.primary_types"),
        children: primaryOptions,
      },
      ...(detailedOptions.length > 0
        ? [
            {
              type: "group",
              key: "detailed-types",
              label: t("index.detailed_types"),
              children: detailedOptions,
            },
          ]
        : []),
    ],
  }
})
const classify = computed(() => resourceTypeOptions.value.classify)
const captureTypeOptions = computed(() => resourceTypeOptions.value.capture)

const restoreResourceTypes = () => {
  const cached = localStorage.getItem("resource-kind-filter")
  if (!cached) {
    appApi.setResourceFilter(resourcesType.value)
    return
  }
  try {
    const saved = JSON.parse(cached)?.res
    resourcesType.value = Array.isArray(saved) && saved.length > 0 ? Array.from(new Set(saved.filter((value) => typeof value === "string"))) : ["all"]
  } catch {
    resourcesType.value = ["all"]
  }
}

const removeUnavailableResourceTypes = () => {
  const available = new Set(classify.value.map((option) => option.value))
  const next = resourcesType.value.filter((value) => available.has(value))
  if (next.length === resourcesType.value.length) return
  resourcesType.value = next.length > 0 ? next : ["all"]
}

const updateResourceTypes = (values: string[]) => {
  const unique = Array.from(new Set(values))
  if (unique.includes("all")) {
    if (!resourcesType.value.includes("all")) {
      resourcesType.value = ["all"]
      return
    }
    const specific = unique.filter((value) => value !== "all")
    resourcesType.value = specific.length > 0 ? specific : ["all"]
    return
  }
  resourcesType.value = unique.length > 0 ? unique : ["all"]
}

const localizedResourceKindName = (definition: appType.ResourceKindDefinition) => {
  const entries = definition.locales ?? {}
  const current = locale.value
  const language = current.split("-")[0]
  return (
    entries[current]?.name ||
    entries[language]?.name ||
    entries.en?.name ||
    Object.values(entries)[0]?.name ||
    (classifyAlias[definition.id] ? t(`index.${classifyAlias[definition.id]}`) : "") ||
    definition.id
  )
}

const hasCapability = (row: appType.ResourceView, capability: string) => Array.isArray(row.capabilities) && row.capabilities.includes(capability)

const canDownload = (row: appType.ResourceView) => hasCapability(row, "download") && row.state !== "partial"

const localizedActionEntry = (definition: appType.PluginActionDefinition) => {
  const entries = definition.locales ?? {}
  const current = locale.value
  const language = current.split("-")[0]
  return entries[current] ?? entries[language] ?? entries.en ?? Object.values(entries)[0] ?? {}
}

const resourceActions = (row: appType.ResourceView): appType.DisplayResourceAction[] => {
  const definitions = pluginActionDefinitions.value[row.source?.pluginId || ""] ?? {}
  return (row.actions ?? []).map((action) => {
    const definition = definitions[action.id]
    const localized = definition ? localizedActionEntry(definition) : {}
    return {
      id: action.id,
      kind: definition?.kind,
      label: localized.name || action.label || action.id,
      description: localized.description || "",
    }
  })
}

const operationPickerVisible = ref(false)
const operationPickerLoading = ref(false)
const operationPickerSessions = ref<OperationSession[]>([])
let operationPickerTarget: {row: appType.ResourceView; actionId: string} | undefined
const submittingResourceOperations = new Set<string>()
const runResourceAction = async (row: appType.ResourceView, actionId: string, pageSessionId?: string) => {
  if (submittingResourceOperations.has(row.id)) return
  const current = executions.value[row.id]
  if (current && executionActive(current)) {
    window?.$message?.info(t(`operations.states.${current.state}`))
    return
  }
  submittingResourceOperations.add(row.id)
  try {
    const response: appType.Res<OperationExecution | {cancelled?: boolean; started?: boolean}> = await appApi.runResourceAction({id: row.id, actionId, pageSessionId})
    if (response.code !== 1) {
      const error = response.data as unknown as {errorCode?: string} | undefined
      if (error?.errorCode === "page_ambiguous") {
        const [sessions] = await Promise.all([operationsApi.sessions(), refreshPluginMetadata()])
        const operationId = pluginActionDefinitions.value[row.source?.pluginId || ""]?.[actionId]?.operation
        if (!operationId) throw new OperationAPIError("page_ambiguous", response.message)
        operationPickerSessions.value = sessions.filter(
          (session) => session.pluginId === row.source?.pluginId && session.operations.includes(operationId) && session.connected && session.ready,
        )
        if (!operationPickerSessions.value.length) throw new OperationAPIError("page_unavailable", "")
        operationPickerTarget = {row, actionId}
        operationPickerVisible.value = true
        return
      }
      throw new OperationAPIError(error?.errorCode || "operation_failed", response.message)
    }
    if ("executionId" in response.data) {
      executionSubmission++
      executions.value[row.id] = response.data
      operationPickerVisible.value = false
      operationPickerTarget = undefined
      window?.$message?.info(t("operations.submitted"))
    } else if (!response.data.cancelled) {
      window?.$message?.info(t("index.plugin_action_started"))
    }
  } catch (error) {
    let message = operationErrorMessage(error, t)
    if (error instanceof OperationAPIError && error.code === "page_unavailable") {
      try {
        const [sessions] = await Promise.all([operationsApi.sessions(), refreshPluginMetadata()])
        const pluginId = row.source?.pluginId || ""
        const operationId = pluginActionDefinitions.value[pluginId]?.[actionId]?.operation
        message = operationPageUnavailableMessage(
          sessions,
          {
            pluginId,
            operationId,
            pageSessionId,
            definition: operationId ? pluginOperationDefinitions.value[pluginId]?.[operationId] : undefined,
          },
          t,
        )
      } catch {
        // Diagnostics must never replace the original operation failure.
      }
    }
    window?.$message?.error(message)
  } finally {
    submittingResourceOperations.delete(row.id)
  }
}

const submitSelectedOperationPage = async (pageSessionId: string) => {
  if (!operationPickerTarget || operationPickerLoading.value) return
  operationPickerLoading.value = true
  const {row, actionId} = operationPickerTarget
  try {
    await runResourceAction(row, actionId, pageSessionId)
  } finally {
    operationPickerLoading.value = false
  }
}

const dataAction = (row: appType.ResourceView, index: number, type: string) => {
  if (type.startsWith("plugin-action:")) {
    const actionId = type.substring("plugin-action:".length)
    void runResourceAction(row, actionId)
    return
  }
  switch (type) {
    case "down":
      download(row, index)
      break
    case "cancel": {
      const taskId = downloadState(row)?.taskId
      if (taskId)
        appApi.cancelDownloadTask(taskId).then((res) => {
          if (res.code === 0) window?.$message?.error(res.message)
        })
      break
    }
    case "copy":
      Clipboard.SetText(primaryURL(row))
        .then(() => window?.$message?.success(t("common.copy_success")))
        .catch(() => window?.$message?.error(t("common.copy_fail")))
      break
    case "json":
      Clipboard.SetText(encodeURIComponent(JSON.stringify(exportableResource(row))))
        .then(() => window?.$message?.success(t("common.copy_success")))
        .catch(() => window?.$message?.error(t("common.copy_fail")))
      break
    case "open":
      Browser.OpenURL(primaryURL(row))
      break
    case "delete":
      if (!canDeleteResource(row)) {
        window?.$message?.error(t("index.delete_tip"))
        return
      }
      appApi.deleteResources({ids: [row.id]}).then((res) => {
        if (res.code === 1) {
          const isRoot = data.value.some((item) => item.id === row.id)
          removeResource(row.id)
          if (isRoot) resourceTotal.value = Math.max(0, resourceTotal.value - 1)
          updateResourceRecordCount(res.data?.recordCount, true)
        } else window?.$message?.error(res.message)
      })
      break
  }
}

const rowKey = (row: appType.ResourceView) => {
  return row.id
}

const resourceRowClassName = (row: appType.ResourceView) => {
  return [checkedRowKeysValue.value.includes(row.id) ? "resource-row--checked" : "", (resourceDepths.value.get(row.id) ?? 0) > 0 ? "resource-row--child" : ""]
    .filter(Boolean)
    .join(" ")
}

const {columns} = useResourceTableColumns({
  t,
  classify,
  pluginResourceKinds,
  resourceKindLabel: localizedResourceKindName,
  checkedRowKeys: checkedRowKeysValue,
  resourceDepths,
  descriptionSearch: descriptionSearchValue,
  urlSearch: urlSearchValue,
  previewRow,
  showPreview: showPreviewRow,
  downloadStatuses: dwStatus,
  executions,
  downloadState,
  filterKinds,
  hasCapability,
  canDownload,
  download: (row, index) => download(row, index),
  updateDescription,
  resourceActions,
  dataAction,
})

const handleCheck = (rowKeys: DataTableRowKey[]) => {
  checkedRowKeysValue.value = rowKeys
}

function updateFilters(filters: DataTableFilterState) {
  filterKinds.value = (filters.primaryType as string[] | null) ?? []
}

const selectedResources = (predicate?: (item: appType.ResourceView) => boolean) => selectedResourceRoots(data.value, new Set(checkedRowKeysValue.value.map(String)), predicate)

const batchDown = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }

  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }

  selectedResources(canDownload).forEach(download)

  checkedRowKeysValue.value = []
}

const batchCancel = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }
  loading.value = true
  const cancelTasks: Promise<any>[] = []
  const selectedIds = new Set(checkedRowKeysValue.value.map(String))
  const taskIds = new Set<string>()
  data.value.forEach((root) =>
    visitResource(root, (item) => {
      if (!selectedIds.has(item.id)) return
      const taskId = downloadState(item)?.taskId
      if (isActiveDownload(item) && taskId) taskIds.add(taskId)
    }),
  )
  taskIds.forEach((taskId) =>
    cancelTasks.push(
      appApi.cancelDownloadTask(taskId).then((res) => {
        if (res.code !== 1) window?.$message?.error(res.message)
      }),
    ),
  )
  await Promise.allSettled(cancelTasks)
  loading.value = false
  checkedRowKeysValue.value = []
}

const batchExport = (type?: string) => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }

  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }

  loadingText.value = t("common.loading")
  loading.value = true

  let jsonData: Array<object | string> = selectedResources()

  if (type === "url") {
    jsonData = (jsonData as appType.ResourceView[]).map((item) => primaryURL(item))
  } else {
    jsonData = (jsonData as appType.ResourceView[]).map((item) => encodeURIComponent(JSON.stringify(exportableResource(item))))
  }

  appApi.exportResources({content: jsonData.join("\n")}).then((res: appType.Res) => {
    loading.value = false
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t("index.import_success"))
    window?.$message?.info(t("index.save_path") + "：" + res.data?.file_name, {
      duration: 5000,
    })
  })
}

const download = (row: appType.ResourceView, _index: number) => {
  if (!canDownload(row)) {
    window?.$message?.error(t("index.download_no_tip"))
    return
  }
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }

  if (isActiveDownload(row)) {
    return
  }
  void downloadStore.create(row.id).catch((error) => window.$message?.error(String(error.message ?? error)))
}

const open = async () => {
  const certificate = await certificateStore.refresh()
  if (certificate.code !== 1 || !certificate.data.desktop.installed) {
    certificateStore.showGuide("capture")
    return
  }
  proxyAction.value = "enable"
  store.openProxy().then((res: appType.Res) => {
    if (res.code === 1) {
      return
    }

    if (["darwin", "linux"].includes(store.envInfo.platform)) {
      showPassword.value = true
    } else {
      window.$message?.error(res.message)
    }
  })
}

const close = () => {
  proxyAction.value = "disable"
  store.unsetProxy().then((res: appType.Res) => {
    if (res.code === 0 && ["darwin", "linux"].includes(store.envInfo.platform)) showPassword.value = true
  })
}

const clear = async () => {
  const deletedIds: string[] = []
  if (checkedRowKeysValue.value.length > 0) {
    deletedIds.push(...selectedResources(canDeleteResource).map((item) => item.id))
    checkedRowKeysValue.value = []
  } else {
    const response = await appApi.clearResources()
    if (response.code !== 1) {
      window?.$message?.error(response.message)
      return
    }
    data.value = []
    expandedRowKeys.value = []
    resourceTotal.value = 0
    updateResourceRecordCount(response.data?.recordCount ?? 0, true)
    nextResourceOffset.value = 0
    return
  }
  if (deletedIds.length === 0) return
  const response = await appApi.deleteResources({ids: deletedIds})
  if (response.code !== 1) {
    window?.$message?.error(response.message)
    return
  }
  const deletedRootCount = data.value.filter((item) => deletedIds.includes(item.id)).length
  deletedIds.forEach(removeResource)
  resourceTotal.value = Math.max(0, resourceTotal.value - deletedRootCount)
  updateResourceRecordCount(response.data?.recordCount, true)
}

const handleImport = (content: string) => {
  if (!content) {
    window?.$message?.error(t("index.import_empty"))
    return
  }
  let newItems = [] as any[]
  content.split("\n").forEach((line, index) => {
    try {
      let res = JSON.parse(decodeURIComponent(line))
      if (res && res?.id) {
        newItems.push(res)
      }
    } catch (e) {
      console.log(e)
    }
  })
  if (newItems.length > 0) {
    appApi.importResources({items: newItems}).then((res: appType.Res) => {
      if (res.code === 0) {
        window?.$message?.error(res.message)
        return
      }
      appApi.listResources({offset: 0, limit: resourcePageSize}).then((page: appType.Res) => {
        data.value = page.data?.items ?? []
        resourceTotal.value = Number(page.data?.total ?? data.value.length)
        updateResourceRecordCount(page.data?.recordCount)
        nextResourceOffset.value = Number(page.data?.nextOffset ?? 0)
      })
    })
  }
  showImport.value = false
}

const handlePassword = async (password: string) => {
  const res = proxyAction.value === "enable" ? await store.openProxy(password) : await store.unsetProxy(password)
  if (res.code === 1) showPassword.value = false
}
</script>
