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
      <!-- Collections use a custom child table; do not also flatten children as tree rows. -->
      <NDataTable
        class="resource-table [--wails-draggable:no-drag]"
        :columns="columns"
        :data="filteredData"
        children-key="__tableTreeChildren"
        :bordered="false"
        :max-height="tableHeight"
        :row-key="rowKey"
        :virtual-scroll="expandedRowKeys.length === 0"
        :header-height="48"
        :height-for-row="() => 48"
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
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="BrowserOpenURL(certUrl)">{{ t("footer.cert_download") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="openDocumentation('home')">{{ t("footer.documentation") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="BrowserOpenURL('https://github.com/putyy/res-downloader')">{{ t("footer.source_code") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="BrowserOpenURL('https://github.com/putyy/res-downloader/issues')">{{ t("footer.help") }}</span>
      <span class="cursor-pointer px-2 py-1 [--wails-draggable:no-drag]" @click="BrowserOpenURL('https://github.com/putyy/res-downloader/releases')">{{
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
import type {DataTableBaseColumn, DataTableFilterState, DataTableRowKey} from "naive-ui"
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
import {exportableResource, findResourceInTree, mergeResourceRuntime, primaryURL, removeResourceFromTree, resourceSome, visitResource} from "@/services/resources"
import ImportJson from "@/components/ImportJson.vue"
import {useEventStore} from "@/stores/event"
import {BrowserOpenURL, ClipboardSetText} from "../../wailsjs/runtime"
import Password from "@/components/Password.vue"
import {useI18n} from "vue-i18n"
import {Apps, ArrowRedoCircleOutline, CloseOutline, DownloadOutline, ServerOutline, TrashOutline} from "@vicons/ionicons5"
import {useCertificateStore} from "@/stores/certificate"
import type {MessageReactive} from "naive-ui"

const {t, locale} = useI18n()
const eventStore = useEventStore()
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

const classifyAlias: {[key: string]: any} = {
  "media.image": computed(() => t("index.image")),
  "media.audio": computed(() => t("index.audio")),
  "media.video": computed(() => t("index.video")),
  "media.collection": computed(() => t("index.collection")),
  "stream.hls": computed(() => t("index.m3u8")),
  "stream.live": computed(() => t("index.live")),
  "document.xls": computed(() => t("index.xls")),
  "document.doc": computed(() => t("index.doc")),
  "document.pdf": computed(() => t("index.pdf")),
  image: computed(() => t("index.image")),
  audio: computed(() => t("index.audio")),
  video: computed(() => t("index.video")),
  m3u8: computed(() => t("index.m3u8")),
  live: computed(() => t("index.live")),
  xls: computed(() => t("index.xls")),
  doc: computed(() => t("index.doc")),
  pdf: computed(() => t("index.pdf")),
  stream: computed(() => t("index.stream")),
  font: computed(() => t("index.font")),
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

const classify = ref<any[]>([])
const captureTypeOptions = ref<any[]>([])

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
    for (const root of data.value)
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
      buildClassify()
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

  buildClassify()
  restoreResourceTypes()

  appApi.listResources({offset: 0, limit: resourcePageSize}).then((res: appType.Res) => {
    if (res.code !== 1) {
      window?.$message?.error(res.message)
      return
    }
    appendResourcePage(res.data?.items ?? [])
    resourceTotal.value = Number(res.data?.total ?? data.value.length)
    updateResourceRecordCount(res.data?.recordCount)
    nextResourceOffset.value = Number(res.data?.nextOffset ?? 0)
    data.value.forEach((item) => visitResource(item, (child) => ensureResourceKind(child.kind)))
    appApi.downloadTasks().then((tasksResponse: appType.Res<appType.DownloadTaskRecord[]>) => {
      if (tasksResponse.code !== 1) return
      const restored = new Set<string>()
      for (const task of tasksResponse.data ?? []) {
        if (restored.has(task.resourceId)) continue
        restored.add(task.resourceId)
        applyTaskStatus(task)
      }
    })
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
      type: "downloadTaskUpdated",
      event: (task: appType.DownloadTaskRecord) => applyTaskStatus(task),
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
      if (root.id !== id && !expandedRowKeys.value.includes(root.id)) expandedRowKeys.value.push(root.id)
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
  visitResource(resource, (child) => ensureResourceKind(child.kind))
  const index = data.value.findIndex((item) => item.id === resource.id)
  if (index >= 0) {
    data.value[index] = mergeResourceRuntime(resource, data.value[index])
    return
  }
  if (store.globalConfig.InsertTail) data.value.push(resource)
  else data.value.unshift(resource)
}

const appendResourcePage = (resources: appType.ResourceView[]) => {
  for (const resource of resources) {
    visitResource(resource, (child) => ensureResourceKind(child.kind))
    const index = data.value.findIndex((item) => item.id === resource.id)
    if (index >= 0) data.value[index] = mergeResourceRuntime(resource, data.value[index])
    else data.value.push(resource)
  }
}

watch(resourcesType, (n, o) => {
  localStorage.setItem("resource-kind-filter", JSON.stringify({res: resourcesType.value}))
  appApi.setResourceFilter(resourcesType.value)
})

const updateItem = (id: string, updater: (item: any) => void) => {
  const item = findResource(id)
  if (item) updater(item)
}

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

const applyTaskStatus = (task: appType.DownloadTaskRecord) => {
  updateItem(task.resourceId, (item) => {
    let message = task.error || ""
    if (!message) {
      if (task.state === "paused") message = t("tasks.paused")
      else if ((task.items?.length ?? 0) > 0 && task.total) message = `${task.downloaded ?? 0}/${task.total}`
      else if (task.total) message = `${Math.floor(((task.downloaded ?? 0) * 100) / task.total)}%`
    }
    item.download = {
      createdAt: task.createdAt,
      startedAt: task.startedAt,
      taskId: task.id,
      state: task.state,
      outputPath: task.outputPath || "",
      message,
      downloaded: task.downloaded,
      total: task.total,
    }
  })
}

const findResource = (id: string): appType.ResourceView | undefined => {
  return findResourceInTree(data.value, id)
}

const removeResource = (id: string) => {
  if (removeResourceFromTree(data.value, id)) {
    expandedRowKeys.value = expandedRowKeys.value.filter((key) => key !== id)
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

const buildClassify = () => {
  const allOption = {value: "all", label: t("index.all")}
  const primaryOptions = [
    {value: "video", label: computed(() => t("index.video"))},
    {value: "audio", label: computed(() => t("index.audio"))},
    {value: "image", label: computed(() => t("index.image"))},
    {value: "document", label: computed(() => t("index.document"))},
    {value: "archive", label: computed(() => t("index.archive"))},
    {value: "collection", label: computed(() => t("index.collection"))},
    {value: "other", label: computed(() => t("index.other"))},
  ]
  const primaryKindAliases = new Set(["media.video", "media.audio", "media.image", "media.collection", "document.text"])
  const seen = new Set<string>(["all", ...primaryOptions.map((option) => option.value)])
  const detailedOptions = pluginResourceKinds.value.flatMap((definition) => {
    if (!definition.id || primaryKindAliases.has(definition.id) || seen.has(definition.id)) return []
    seen.add(definition.id)
    return [{value: definition.id, label: localizedResourceKindName(definition)}]
  })
  classify.value = [allOption, ...primaryOptions, ...detailedOptions]
  captureTypeOptions.value = [
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
  ]
}

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
  return entries[current]?.name || entries[language]?.name || entries.en?.name || Object.values(entries)[0]?.name || classifyAlias[definition.id]?.value || definition.id
}

const ensureResourceKind = (_kind?: string) => undefined

watch(locale, buildClassify)

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
      const taskId = row.download?.taskId
      if (taskId)
        appApi.cancelDownloadTask(taskId).then((res) => {
          if (res.code === 0) window?.$message?.error(res.message)
          else row.download = {state: "cancelled"}
        })
      break
    }
    case "copy":
      ClipboardSetText(primaryURL(row)).then((is: boolean) => {
        if (is) {
          window?.$message?.success(t("common.copy_success"))
        } else {
          window?.$message?.error(t("common.copy_fail"))
        }
      })
      break
    case "json":
      ClipboardSetText(encodeURIComponent(JSON.stringify(exportableResource(row)))).then((is: boolean) => {
        if (is) {
          window?.$message?.success(t("common.copy_success"))
        } else {
          window?.$message?.error(t("common.copy_fail"))
        }
      })
      break
    case "open":
      BrowserOpenURL(primaryURL(row))
      break
    case "delete":
      if (isActiveDownload(row)) {
        window?.$message?.error(t("index.delete_tip"))
        return
      }
      appApi.deleteResources({ids: [row.id]}).then((res) => {
        if (res.code === 1) {
          removeResource(row.id)
          resourceTotal.value = Math.max(0, resourceTotal.value - 1)
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
  return checkedRowKeysValue.value.includes(rowKey(row)) ? "resource-row--checked" : ""
}

const {columns} = useResourceTableColumns({
  t,
  classify,
  pluginResourceKinds,
  resourceKindLabel: localizedResourceKindName,
  checkedRowKeys: checkedRowKeysValue,
  descriptionSearch: descriptionSearchValue,
  urlSearch: urlSearchValue,
  previewRow,
  showPreview: showPreviewRow,
  downloadStatuses: dwStatus,
  executions,
  rowKey,
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

const updateFilters = (filters: DataTableFilterState, initiatorColumn: DataTableBaseColumn) => {
  filterKinds.value = filters.primaryType as string[]
}

const batchDown = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }

  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }

  data.value.forEach((item, index) => {
    if (checkedRowKeysValue.value.includes(item.id) && canDownload(item)) {
      download(item, index)
    }
  })

  checkedRowKeysValue.value = []
}

const batchCancel = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }
  loading.value = true
  const cancelTasks: Promise<any>[] = []
  data.value.forEach((item) => {
    if (!checkedRowKeysValue.value.includes(item.id)) {
      return
    }

    if (isActiveDownload(item) && item.download?.taskId) {
      cancelTasks.push(
        appApi.cancelDownloadTask(item.download.taskId).then((res) => {
          if (res.code === 1) item.download = {state: "cancelled"}
          else window?.$message?.error(res.message)
        }),
      )
    }
  })
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

  let jsonData: Array<object | string> = data.value.filter((item) => checkedRowKeysValue.value.includes(item.id))

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
  row.download = {state: "pending", message: t("index.pending")}
  appApi
    .createDownload({id: row.id})
    .then((res: appType.Res<appType.DownloadTaskRecord>) => {
      if (res.code === 0) {
        row.download = {state: "ready"}
        window?.$message?.error(res.message)
        return
      }
      applyTaskStatus(res.data)
    })
    .catch(() => {
      row.download = {state: "ready"}
    })
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
    data.value.forEach((item) => {
      if (checkedRowKeysValue.value.includes(item.id) && !isActiveDownload(item)) {
        deletedIds.push(item.id)
      }
    })
    checkedRowKeysValue.value = []
  } else {
    const response = await appApi.clearResources()
    if (response.code !== 1) {
      window?.$message?.error(response.message)
      return
    }
    data.value = []
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
  deletedIds.forEach(removeResource)
  resourceTotal.value = Math.max(0, resourceTotal.value - deletedIds.length)
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
        data.value.forEach((item) => visitResource(item, (child) => ensureResourceKind(child.kind)))
      })
    })
  }
  showImport.value = false
}

const handlePassword = async (password: string) => {
  const res = proxyAction.value === "enable" ? await store.openProxy(password) : await store.unsetProxy(password)
  if (res.code === 1) showPassword.value = false
}

const isActiveDownload = (row: appType.ResourceView) => ["pending", "resolving", "downloading", "processing", "pausing"].includes(row.download?.state || "")
</script>
