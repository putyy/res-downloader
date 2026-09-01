<template>
  <div class="h-full flex flex-col px-5 pt-5 overflow-y-auto [&::-webkit-scrollbar]:hidden">
    <div class="pb-2 z-40" id="header">
      <NSpace>
        <NButton v-if="isProxy" secondary type="primary" @click.stop="close" style="--wails-draggable:no-drag">
          <span class="inline-block w-1.5 h-1.5 bg-red-600 rounded-full mr-1 animate-pulse"></span>
          {{ t("index.close_grab") }}{{ resourceTotal > 0 ? `&nbsp;${t('index.total_resources', {count: resourceTotal})}` : '' }}
        </NButton>
        <NButton v-else tertiary type="tertiary" @click.stop="open" style="--wails-draggable:no-drag">
          {{ t("index.open_grab") }}{{ resourceTotal > 0 ? `&nbsp;${t('index.total_resources', {count: resourceTotal})}` : '' }}
        </NButton>
        <NSelect style="min-width: 100px;--wails-draggable:no-drag" :placeholder="t('index.grab_type')"
                 :value="resourcesType" multiple clearable
                 :max-tag-count="3" :options="captureTypeOptions" @update:value="updateResourceTypes"></NSelect>
        <NButtonGroup style="--wails-draggable:no-drag">

          <NButton v-if="rememberChoice" tertiary type="error" @click.stop="clear" style="--wails-draggable:no-drag">
            <template #icon>
              <n-icon>
                <TrashOutline/>
              </n-icon>
            </template>
            {{ t("index.clear_list") }}
          </NButton>
          <n-popconfirm
              v-else
              @positive-click="()=>{rememberChoice=rememberChoiceTmp;clear()}"
              :show-icon="false"
          >
            <template #trigger>
              <NButton tertiary type="error" style="--wails-draggable:no-drag">
                <template #icon>
                  <n-icon>
                    <TrashOutline/>
                  </n-icon>
                </template>
                {{ t("index.clear_list") }}
              </NButton>
            </template>
            <div>
              <div class="flex flex-row items-center text-red-700 my-2 text-base">
                <n-icon>
                  <TrashOutline/>
                </n-icon>
                <p class="ml-1">{{ t("index.clear_list_tip") }}</p>
              </div>
              <NCheckbox
                  v-model:checked="rememberChoiceTmp"
              >
                <span class="app-muted-text">{{ t('index.remember_clear_choice') }}</span>
              </NCheckbox>
            </div>
          </n-popconfirm>

          <NButton tertiary type="primary" @click.stop="batchDown">
            <template #icon>
              <n-icon>
                <DownloadOutline/>
              </n-icon>
            </template>
            {{ t('index.batch_download') }}
          </NButton>
          <NButton tertiary type="info">
            <NPopover placement="bottom" trigger="hover">
              <template #trigger>
                <NIcon size="18" class="">
                  <Apps/>
                </NIcon>
              </template>
              <div class="flex flex-col">
                <NButton tertiary type="error" @click.stop="batchCancel" class="my-1">
                  <template #icon>
                    <n-icon>
                      <CloseOutline/>
                    </n-icon>
                  </template>
                  {{ t('index.cancel_down') }}
                </NButton>
                <NButton tertiary type="warning" @click.stop="batchExport()" class="my-1">
                  <template #icon>
                    <n-icon>
                      <ArrowRedoCircleOutline/>
                    </n-icon>
                  </template>
                  {{ t('index.batch_export') }}
                </NButton>
                <NButton tertiary type="info" @click.stop="showImport=true" class="my-1">
                  <template #icon>
                    <n-icon>
                      <ServerOutline/>
                    </n-icon>
                  </template>
                  {{ t('index.batch_import') }}
                </NButton>
                <NButton tertiary type="primary" @click.stop="batchExport('url')" class="my-1">
                  <template #icon>
                    <n-icon>
                      <ArrowRedoCircleOutline/>
                    </n-icon>
                  </template>
                  {{ t('index.export_url') }}
                </NButton>
              </div>
            </NPopover>
          </NButton>
        </NButtonGroup>
		<NButton v-if="nextResourceOffset > 0" secondary :loading="loadingMoreResources" @click="loadMoreResources">
		  {{ t('index.load_more', {loaded: data.length, total: resourceTotal}) }}
		</NButton>
      </NSpace>
    </div>
    <div class="min-h-0 flex-1">
      <NDataTable
          class="resource-table"
          :columns="columns"
          :data="filteredData"
          :bordered="false"
          :max-height="tableHeight"
          :row-key="rowKey"
          :virtual-scroll="expandedRowKeys.length === 0"
          :header-height="48"
          :height-for-row="()=> 48"
          :checked-row-keys="checkedRowKeysValue"
          :expanded-row-keys="expandedRowKeys"
          :row-class-name="resourceRowClassName"
          @update:checked-row-keys="handleCheck"
          @update:expanded-row-keys="(keys: any) => expandedRowKeys = keys"
          @update:filters="updateFilters"
          style="--wails-draggable:no-drag"
      />
    </div>
    <div class="resource-footer flex items-center justify-center" id="bottom">
      <span class="cursor-pointer px-2 py-1" @click="BrowserOpenURL(certUrl)">{{ t('footer.cert_download') }}</span>
      <span class="cursor-pointer px-2 py-1" @click="BrowserOpenURL('https://res.putyy.com')">{{ t('footer.documentation') }}</span>
      <span class="cursor-pointer px-2 py-1" @click="BrowserOpenURL('https://github.com/putyy/res-downloader')">{{ t('footer.source_code') }}</span>
      <span class="cursor-pointer px-2 py-1" @click="BrowserOpenURL('https://github.com/putyy/res-downloader/issues')">{{ t('footer.help') }}</span>
      <span class="cursor-pointer px-2 py-1" @click="BrowserOpenURL('https://github.com/putyy/res-downloader/releases')">{{ t('footer.update_log') }}</span>
    </div>
    <Preview v-model:showModal="showPreviewRow" :previewRow="previewRow"/>
    <ShowLoading :loadingText="loadingText" :isLoading="loading"/>
    <ImportJson v-model:showModal="showImport" @submit="handleImport"/>
    <Password v-model:showModal="showPassword" @submit="handlePassword"/>
  </div>
</template>

<script lang="ts" setup>
import type {DataTableBaseColumn, DataTableFilterState, DataTableRowKey} from "naive-ui"
import {NButton, NDataTable, NIcon, NPopover, NSpace} from "naive-ui"
import {computed, onMounted, onUnmounted, ref, watch} from "vue"
import type {appType} from "@/types/app"
import Preview from "@/components/Preview.vue"
import ShowLoading from "@/components/ShowLoading.vue"
import {useIndexStore} from "@/stores"
import appApi from "@/api/app"
import {useResourceTableColumns} from '@/components/resource/useResourceTableColumns'
import {exportableResource, findResourceInTree, mergeResourceRuntime, primaryURL, removeResourceFromTree, resourceSome, visitResource} from '@/services/resources'
import ImportJson from "@/components/ImportJson.vue"
import {useEventStore} from "@/stores/event"
import {BrowserOpenURL, ClipboardSetText} from "../../wailsjs/runtime"
import Password from "@/components/Password.vue"
import {useI18n} from 'vue-i18n'
import {Apps, ArrowRedoCircleOutline, CloseOutline, DownloadOutline, ServerOutline, TrashOutline} from "@vicons/ionicons5"
import {useCertificateStore} from '@/stores/certificate'

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
const filterKinds = ref<string[]>([])
const filteredData = computed(() => {
  let result = data.value

  if (filterKinds.value.length > 0) {
    result = result.filter(item => resourceSome(item, child =>
        (!!child.primaryType && filterKinds.value.includes(child.primaryType)) ||
        (!!child.kind && filterKinds.value.includes(child.kind)),
    ))
  }

  if (descriptionSearchValue.value) {
    const expected = descriptionSearchValue.value.toLowerCase()
    result = result.filter(item => resourceSome(item, child => !!child.title?.toLowerCase().includes(expected)))
  }

  if (urlSearchValue.value) {
    const expected = urlSearchValue.value.toLowerCase()
    result = result.filter(item => resourceSome(item, child => primaryURL(child).toLowerCase().includes(expected)))
  }

  return result
})

const store = useIndexStore()
const certificateStore = useCertificateStore()
const tableHeight = ref(800)
const resourcesType = ref<string[]>(["all"])
const pluginResourceKinds = ref<appType.ResourceKindDefinition[]>([])
const pluginActionDefinitions = ref<Record<string, Record<string, appType.PluginActionDefinition>>>({})

const classifyAlias: { [key: string]: any } = {
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
  font: computed(() => t("index.font"))
}

const dwStatus = computed<any>(() => {
  return {
    ready: t("index.ready"),
    partial: t("index.partial"),
    pending: t("index.pending"),
    running: t("index.running"),
    error: t("index.error"),
    done: t("index.done"),
    handle: t("index.handle")
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
const proxyAction = ref<'enable' | 'disable'>('enable')
const disposers: Array<() => void> = []
const handleWindowResize = () => resetTableHeight()

onMounted(() => {
  try {
    window.addEventListener("resize", handleWindowResize)
  } catch (e) {
    window.$message?.error(JSON.stringify(e), {duration: 5000})
  }

  buildClassify()
  restoreResourceTypes()
  appApi.plugins().then((res: appType.Res) => {
    if (res.code !== 1) return
    const loadedPlugins = (res.data.plugins ?? []).filter((plugin: appType.PluginStatus) => plugin.loaded)
    pluginResourceKinds.value = loadedPlugins
        .flatMap((plugin: appType.PluginStatus) => plugin.manifest.resourceKinds ?? [])
    pluginActionDefinitions.value = Object.fromEntries(
        loadedPlugins.map((plugin: appType.PluginStatus) => [plugin.manifest.id, plugin.manifest.actions ?? {}]),
    )
    buildClassify()
    removeUnavailableResourceTypes()
  })

  appApi.listResources({offset: 0, limit: resourcePageSize}).then((res: appType.Res) => {
    if (res.code !== 1) {
      window?.$message?.error(res.message)
      return
    }
    appendResourcePage(res.data?.items ?? [])
    resourceTotal.value = Number(res.data?.total ?? data.value.length)
    nextResourceOffset.value = Number(res.data?.nextOffset ?? 0)
    data.value.forEach(item => visitResource(item, child => ensureResourceKind(child.kind)))
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

  disposers.push(watch(rememberChoice, () => {
    if (rememberChoice.value) {
      localStorage.setItem("remember-clear-choice", "1")
    } else {
      localStorage.removeItem("remember-clear-choice")
    }
  }))

  resetTableHeight()

  disposers.push(eventStore.addHandle({
    type: "resourceAdded",
    event: (res: appType.ResourceView) => {
	  const exists = data.value.some(item => item.id === res.id)
	  upsertResourceRoot(res)
	  if (!exists) resourceTotal.value++
    }
  }))

  disposers.push(eventStore.addHandle({
    type: "resourceUpdated",
    event: (res: appType.ResourceView) => {
	  upsertResourceRoot(res)
    }
  }))

  disposers.push(eventStore.addHandle({
    type: 'resourcesBatch',
    event: (payload: {items?: appType.ResourceView[], total?: number}) => {
      for (const resource of payload?.items ?? []) upsertResourceRoot(resource)
      const nextTotal = Number(payload?.total ?? Math.max(resourceTotal.value, data.value.length))
      if (nextResourceOffset.value > 0 && nextTotal > resourceTotal.value) {
        nextResourceOffset.value += nextTotal - resourceTotal.value
      }
      resourceTotal.value = nextTotal
    },
  }))

  disposers.push(eventStore.addHandle({
    type: 'downloadTaskUpdated',
    event: (task: appType.DownloadTaskRecord) => applyTaskStatus(task),
  }))

  disposers.push(eventStore.addHandle({
    type: "resourceActionProgress",
    event: (res: { status: string, outputPath?: string, message?: string }) => {
      if (res.status === 'done') {
        window?.$message?.success(t('index.plugin_action_done', {path: res.outputPath || ''}), {duration: 5000})
      } else if (res.status === 'error') {
        window?.$message?.error(t('index.plugin_action_failed', {message: res.message || ''}), {duration: 5000})
      }
    }
  }))
})

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize)
  disposers.splice(0).forEach(dispose => dispose())
})

const loadMoreResources = async () => {
	if (nextResourceOffset.value <= 0 || loadingMoreResources.value) return
	loadingMoreResources.value = true
	try {
	  const response = await appApi.listResources({offset: nextResourceOffset.value, limit: resourcePageSize}) as appType.Res
	  if (response.code !== 1) {
		window.$message?.error(response.message)
		return
	  }
	  appendResourcePage(response.data?.items ?? [])
	  resourceTotal.value = Number(response.data?.total ?? data.value.length)
	  nextResourceOffset.value = Number(response.data?.nextOffset ?? 0)
	} finally {
	  loadingMoreResources.value = false
	}
}

const upsertResourceRoot = (resource: appType.ResourceView) => {
	visitResource(resource, child => ensureResourceKind(child.kind))
	const index = data.value.findIndex(item => item.id === resource.id)
	if (index >= 0) {
	  data.value[index] = mergeResourceRuntime(resource, data.value[index])
	  return
	}
	if (store.globalConfig.InsertTail) data.value.push(resource)
	else data.value.unshift(resource)
}

const appendResourcePage = (resources: appType.ResourceView[]) => {
  for (const resource of resources) {
    visitResource(resource, child => ensureResourceKind(child.kind))
    const index = data.value.findIndex(item => item.id === resource.id)
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

  const previousValue = item.title || ''
  item.title = value
  try {
    const response = await appApi.updateResource({id, title: value}) as appType.Res<{ id: string, title: string }>
    if (response.code !== 1) throw new Error(response.message)
  } catch (error: any) {
    if (item.title === value) item.title = previousValue
    window.$message?.error(t('index.description_save_failed', {message: error?.message || String(error)}))
  }
}

const applyTaskStatus = (task: appType.DownloadTaskRecord) => {
  updateItem(task.resourceId, item => {
    let message = task.error || ''
    if (task.state === 'paused') message = t('tasks.paused')
    else if ((task.items?.length ?? 0) > 0 && task.total) message = `${task.downloaded ?? 0}/${task.total}`
    else if (task.total) message = `${Math.floor(((task.downloaded ?? 0) * 100) / task.total)}%`
    item.download = {
      taskId: task.id,
      state: task.state,
      outputPath: task.outputPath || '',
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
    expandedRowKeys.value = expandedRowKeys.value.filter(key => key !== id)
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
    {value: 'video', label: computed(() => t('index.video'))},
    {value: 'audio', label: computed(() => t('index.audio'))},
    {value: 'image', label: computed(() => t('index.image'))},
    {value: 'document', label: computed(() => t('index.document'))},
    {value: 'archive', label: computed(() => t('index.archive'))},
    {value: 'collection', label: computed(() => t('index.collection'))},
    {value: 'other', label: computed(() => t('index.other'))},
  ]
  const primaryKindAliases = new Set([
    'media.video', 'media.audio', 'media.image', 'media.collection', 'document.text',
  ])
  const seen = new Set<string>(['all', ...primaryOptions.map(option => option.value)])
  const detailedOptions = pluginResourceKinds.value.flatMap(definition => {
    if (!definition.id || primaryKindAliases.has(definition.id) || seen.has(definition.id)) return []
    seen.add(definition.id)
    return [{value: definition.id, label: localizedResourceKindName(definition)}]
  })
  classify.value = [allOption, ...primaryOptions, ...detailedOptions]
  captureTypeOptions.value = [
    allOption,
    {type: 'group', key: 'primary-types', label: t('index.primary_types'), children: primaryOptions},
    ...(detailedOptions.length > 0
        ? [{type: 'group', key: 'detailed-types', label: t('index.detailed_types'), children: detailedOptions}]
        : []),
  ]
}

const restoreResourceTypes = () => {
  const cached = localStorage.getItem('resource-kind-filter')
  if (!cached) {
    appApi.setResourceFilter(resourcesType.value)
    return
  }
  try {
    const saved = JSON.parse(cached)?.res
    resourcesType.value = Array.isArray(saved) && saved.length > 0
        ? Array.from(new Set(saved.filter(value => typeof value === 'string')))
        : ['all']
  } catch {
    resourcesType.value = ['all']
  }
}

const removeUnavailableResourceTypes = () => {
  const available = new Set(classify.value.map(option => option.value))
  const next = resourcesType.value.filter(value => available.has(value))
  if (next.length === resourcesType.value.length) return
  resourcesType.value = next.length > 0 ? next : ['all']
}

const updateResourceTypes = (values: string[]) => {
  const unique = Array.from(new Set(values))
  if (unique.includes('all')) {
    if (!resourcesType.value.includes('all')) {
      resourcesType.value = ['all']
      return
    }
    const specific = unique.filter(value => value !== 'all')
    resourcesType.value = specific.length > 0 ? specific : ['all']
    return
  }
  resourcesType.value = unique.length > 0 ? unique : ['all']
}

const localizedResourceKindName = (definition: appType.ResourceKindDefinition) => {
  const entries = definition.locales ?? {}
  const current = locale.value
  const language = current.split('-')[0]
  return entries[current]?.name || entries[language]?.name || entries.en?.name || Object.values(entries)[0]?.name || classifyAlias[definition.id]?.value || definition.id
}

const ensureResourceKind = (_kind?: string) => undefined

watch(locale, buildClassify)

const hasCapability = (row: appType.ResourceView, capability: string) =>
    Array.isArray(row.capabilities) && row.capabilities.includes(capability)

const canDownload = (row: appType.ResourceView) =>
    hasCapability(row, 'download') && row.state !== 'partial'

const localizedActionEntry = (definition: appType.PluginActionDefinition) => {
  const entries = definition.locales ?? {}
  const current = locale.value
  const language = current.split('-')[0]
  return entries[current] ?? entries[language] ?? entries.en ?? Object.values(entries)[0] ?? {}
}

const resourceActions = (row: appType.ResourceView): appType.DisplayResourceAction[] => {
  const definitions = pluginActionDefinitions.value[row.source?.pluginId || ''] ?? {}
  return (row.actions ?? []).flatMap(action => {
    const definition = definitions[action.id]
    if (!definition) return []
    const localized = localizedActionEntry(definition)
    return [{
      id: action.id,
      label: localized.name || action.label || action.id,
      description: localized.description || '',
    }]
  })
}

const dataAction = (row: appType.ResourceView, index: number, type: string) => {
  if (type.startsWith('plugin-action:')) {
    const actionId = type.substring('plugin-action:'.length)
    appApi.runResourceAction({id: row.id, actionId}).then((res: appType.Res) => {
      if (res.code === 0) window?.$message?.error(res.message)
      else if (!res.data?.cancelled) window?.$message?.info(t('index.plugin_action_started'))
    })
    return
  }
  switch (type) {
    case "down":
      download(row, index)
      break
    case "cancel": {
      const taskId = row.download?.taskId
      if (taskId) appApi.cancelDownloadTask(taskId).then((res) => {
        if (res.code === 0) window?.$message?.error(res.message)
        else row.download = {state: 'cancelled'}
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
		}
        else window?.$message?.error(res.message)
      })
      break
  }
}

const rowKey = (row: appType.ResourceView) => {
  return row.id
}

const resourceRowClassName = (row: appType.ResourceView) => {
  return checkedRowKeysValue.value.includes(rowKey(row)) ? 'resource-row--checked' : ''
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
      cancelTasks.push(appApi.cancelDownloadTask(item.download.taskId).then((res) => {
        if (res.code === 1) item.download = {state: 'cancelled'}
        else window?.$message?.error(res.message)
      }))
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

  let jsonData: Array<object | string> = data.value.filter(item => checkedRowKeysValue.value.includes(item.id))

  if (type === "url") {
    jsonData = (jsonData as appType.ResourceView[]).map(item => primaryURL(item))
  } else {
    jsonData = (jsonData as appType.ResourceView[]).map(item => encodeURIComponent(JSON.stringify(exportableResource(item))))
  }

  appApi.exportResources({content: jsonData.join("\n")}).then((res: appType.Res) => {
    loading.value = false
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t("index.import_success"))
    window?.$message?.info(t("index.save_path") + "：" + res.data?.file_name, {
      duration: 5000
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
  row.download = {state: 'pending', message: t('index.pending')}
  appApi.createDownload({id: row.id}).then((res: appType.Res<appType.DownloadTaskRecord>) => {
    if (res.code === 0) {
      row.download = {state: 'ready'}
      window?.$message?.error(res.message)
      return
    }
    applyTaskStatus(res.data)
  }).catch(() => {
    row.download = {state: 'ready'}
  })
}

const open = async () => {
  const certificate = await certificateStore.refresh()
  if (certificate.code !== 1 || !certificate.data.desktop.installed) {
    certificateStore.showGuide('capture')
    return
  }
  proxyAction.value = 'enable'
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
  proxyAction.value = 'disable'
  store.unsetProxy().then((res: appType.Res) => {
    if (res.code === 0 && ['darwin', 'linux'].includes(store.envInfo.platform)) showPassword.value = true
  })
}

const clear = async () => {
  const deletedIds: string[] = []
  if (checkedRowKeysValue.value.length > 0) {
    data.value.forEach(item => {
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
		nextResourceOffset.value = Number(page.data?.nextOffset ?? 0)
		data.value.forEach(item => visitResource(item, child => ensureResourceKind(child.kind)))
	  })
    })
  }
  showImport.value = false
}

const handlePassword = async (password: string) => {
  const res = proxyAction.value === 'enable'
      ? await store.openProxy(password)
      : await store.unsetProxy(password)
  if (res.code === 1) showPassword.value = false
}

const isActiveDownload = (row: appType.ResourceView) => ['pending', 'resolving', 'downloading', 'processing', 'pausing'].includes(row.download?.state || '')
</script>
