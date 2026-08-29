<template>
  <div class="h-full overflow-hidden p-5">
    <div class="mx-auto h-full min-h-0 w-full">
      <NTabs
          v-model:value="activeTab"
          type="line"
          animated
          class="h-full"
          pane-wrapper-class="min-h-0 flex-1"
          pane-class="h-full overflow-y-auto [&::-webkit-scrollbar]:hidden"
          @update:value="onTabChanged"
      >
        <template #suffix>
          <NSpace :wrap="false">
            <NButton secondary :loading="localInspecting" @click="inspectLocalPlugin">
              {{ t('plugin.install_file') }}
            </NButton>
            <NButton v-if="activeTab === 'installed'" secondary :loading="reloading" @click="reloadPlugins">
              {{ t('plugin.reload') }}
            </NButton>
            <NButton v-else secondary :loading="storeLoading" @click="loadPluginStore(true)">
              {{ t('plugin.store_refresh') }}
            </NButton>
          </NSpace>
        </template>

        <NTabPane name="installed" :tab="t('plugin.installed_tab')">
          <NSpin :show="loading">
            <NEmpty v-if="!loading && pluginStatuses.length === 0" :description="t('plugin.empty')"/>
            <div v-else class="plugin-grid" style="--wails-draggable:no-drag">
              <InstalledPluginCard
                  v-for="plugin in pluginStatuses"
                  :key="plugin.manifest.id || plugin.path"
                  :plugin="plugin"
                  :updating-id="updatingPluginID"
                  :rollback-id="rollbackPluginID"
                  :uninstalling-id="uninstallingPluginID"
                  @enabled="setPluginEnabled"
                  @resource-rules="openResourceRules"
                  @rollback="rollbackPlugin"
                  @configure="openPluginSettings"
                  @uninstall="uninstallPlugin"
              />
            </div>
          </NSpin>
        </NTabPane>

        <NTabPane name="store" style="--wails-draggable:no-drag">
          <template #tab>
            <NBadge
                :value="storeUpdateCount"
                :show="storeUpdateCount > 0"
                :max="99"
                type="warning"
                :offset="[8, 0]"
            >
              <span>{{ t('plugin.store_tab') }}</span>
            </NBadge>
          </template>
          <div class="mb-3 space-y-3">
            <NAlert type="warning" :show-icon="false">
              {{ t('plugin.store_warning') }}
            </NAlert>
            <NAlert v-if="storeStale" type="warning" :show-icon="false">
              {{ t('plugin.store_stale') }}<span v-if="storeWarning">：{{ storeWarning }}</span>
            </NAlert>
            <NInput v-model:value="storeSearch" clearable :placeholder="t('plugin.store_search')"/>
          </div>
          <NSpin :show="storeLoading">
            <NEmpty v-if="!storeLoading && filteredStoreEntries.length === 0" :description="t('plugin.store_empty')"/>
            <div v-else class="plugin-grid" style="--wails-draggable:no-drag">
              <StoreExtensionCard
                  v-for="extension in filteredStoreEntries"
                  :key="extension.repository"
                  :extension="extension"
                  :installing-repository="storeInstallingRepository"
                  :install-disabled="storeInstallDisabled(extension)"
                  :install-label="storeInstallLabel(extension)"
                  :update-available="storeCanUpdate(extension)"
                  :installed-version="installedPlugin(extension)?.manifest.version"
                  @install="installFromStore"
              />
            </div>
          </NSpin>
        </NTabPane>
      </NTabs>

      <NModal v-model:show="localInstallModalVisible">
        <NCard
            v-if="localInspection"
            class="w-[min(680px,calc(100vw-48px))]"
            :title="t('plugin.install_file_title')"
            :bordered="false"
            role="dialog"
            aria-modal="true"
            style="--wails-draggable:no-drag"
        >
          <div class="space-y-3">
            <div>
              <div class="text-base font-medium">{{ localizedPluginName(localInspection.manifest) }}</div>
              <div class="mt-1 text-xs text-gray-500">
                {{ localInspection.manifest.id }} · v{{ localInspection.manifest.version }} · API
                {{ localInspection.manifest.apiVersion }}
              </div>
            </div>
            <NAlert v-if="localStoreMatchText" :type="localStoreMatchType" :show-icon="false">
              {{ localStoreMatchText }}
            </NAlert>
            <NAlert v-if="localInspection.installed" type="info" :show-icon="false">
              {{ t('plugin.local_installed_version', {version: localInspection.installed.version}) }}
            </NAlert>
            <div>
              <div class="mb-1 text-sm font-medium">{{ t('plugin.local_domains') }}</div>
              <div class="flex flex-wrap gap-1">
                <NTag v-for="domain in localInspection.manifest.permissions?.domains ?? []" :key="domain" size="small">
                  {{ domain }}
                </NTag>
                <span v-if="!(localInspection.manifest.permissions?.domains?.length)"
                      class="text-xs text-gray-500">-</span>
              </div>
            </div>
            <div>
              <div class="mb-1 text-sm font-medium">{{ t('plugin.local_capabilities') }}</div>
              <div class="flex flex-wrap gap-1">
                <NTag
                    v-for="capability in localInspection.manifest.permissions?.capabilities ?? []"
                    :key="capability"
                    size="small"
                    type="warning"
                >
                  {{ capability }}
                </NTag>
                <span v-if="!(localInspection.manifest.permissions?.capabilities?.length)" class="text-xs text-gray-500">-</span>
              </div>
            </div>
            <NAlert v-if="hasPageInjectionPermission(localInspection.manifest)" type="error" :show-icon="false">
              {{ t('plugin.page_injection_warning') }}
            </NAlert>
            <div class="break-all text-xs text-gray-500">
              {{ t('plugin.local_content_sha256') }}：{{ localInspection.contentSha256 }}
            </div>
          </div>
          <template #footer>
            <div class="flex justify-end gap-2">
              <NButton @click="localInstallModalVisible = false">{{ t('plugin.cancel') }}</NButton>
              <NButton
                  type="primary"
                  :loading="localInstalling"
                  :disabled="!!localInspection.installed?.builtin || !!localInspection.installed?.bundled"
                  @click="installLocalPlugin"
              >
                {{ localInspection.installed ? t('plugin.local_replace') : t('plugin.store_install') }}
              </NButton>
            </div>
          </template>
        </NCard>
      </NModal>

      <NModal v-model:show="settingsModalVisible">
        <NCard
            class="w-[min(680px,calc(100vw-48px))]"
            :title="settingsModalTitle"
            :bordered="false"
            role="dialog"
            aria-modal="true"
            style="--wails-draggable:no-drag"
        >
          <NForm v-if="!advancedSettingsMode" label-placement="top">
            <NFormItem v-for="field in selectedSettingFields" :key="field.key" :label="settingFieldLabel(field.key, field.schema)">
              <div class="w-full">
                <NSelect v-if="Array.isArray(field.schema.enum)" v-model:value="pluginSettingValues[selectedPluginID][field.key]" :options="settingEnumOptions(field.schema)"/>
                <NSwitch v-else-if="field.schema.type === 'boolean'" v-model:value="pluginSettingValues[selectedPluginID][field.key]"/>
                <NInputNumber v-else-if="field.schema.type === 'number' || field.schema.type === 'integer'" v-model:value="pluginSettingValues[selectedPluginID][field.key]" class="w-full"/>
                <NInput v-else-if="field.schema.type === 'string' || !field.schema.type" v-model:value="pluginSettingValues[selectedPluginID][field.key]"/>
                <NAlert v-else type="warning" :show-icon="false">
                  {{ t('plugin.unsupported_setting', {name: field.key}) }}
                </NAlert>
                <div v-if="settingFieldDescription(field.schema)" class="mt-1 text-xs text-gray-500">
                  {{ settingFieldDescription(field.schema) }}
                </div>
              </div>
            </NFormItem>
            <NEmpty v-if="selectedSettingFields.length === 0" :description="t('plugin.no_settings_fields')"/>
          </NForm>
          <NInput v-else v-model:value="pluginSettingsJSON[selectedPluginID]" type="textarea" :rows="14" :placeholder="t('plugin.settings_json')"/>
          <template #footer>
            <div class="flex items-center justify-between gap-2">
              <NButton text type="primary" @click="toggleAdvancedSettings">
                {{ advancedSettingsMode ? t('plugin.form_view') : t('plugin.advanced_json') }}
              </NButton>
              <div class="flex gap-2">
                <NButton @click="settingsModalVisible = false">{{ t('plugin.cancel') }}</NButton>
                <NButton type="primary" :loading="savingPluginID === selectedPluginID" @click="savePluginSettings(selectedPluginID)">
                  {{ t('plugin.save_settings') }}
                </NButton>
              </div>
            </div>
          </template>
        </NCard>
      </NModal>
    </div>
  </div>
</template>

<script lang="ts" setup>
import {computed, onMounted, ref} from 'vue'
import axios from 'axios'
import {useI18n} from 'vue-i18n'
import {useRouter} from 'vue-router'
import appApi from '@/api/app'
import type {appType} from '@/types/app'
import InstalledPluginCard from '@/components/plugin/InstalledPluginCard.vue'
import StoreExtensionCard from '@/components/plugin/StoreExtensionCard.vue'

const {t, locale} = useI18n()
const router = useRouter()
const genericDetectorID = 'builtin.generic-detector'

interface LocalPluginInspection {
  token: string
  manifest: appType.PluginManifest
  contentSha256: string
  storeMatch: 'same-version' | 'different' | 'not-listed' | 'cache-unavailable'
  installed?: { version: string, builtin: boolean, bundled: boolean }
}

const pluginStatuses = ref<appType.PluginStatus[]>([])
const pluginSettingsJSON = ref<Record<string, string>>({})
const pluginSettingValues = ref<Record<string, Record<string, any>>>({})
const loading = ref(false)
const reloading = ref(false)
const updatingPluginID = ref('')
const savingPluginID = ref('')
const uninstallingPluginID = ref('')
const localInspecting = ref(false)
const localInstalling = ref(false)
const localInstallModalVisible = ref(false)
const localInspection = ref<LocalPluginInspection | null>(null)
const activeTab = ref<'installed' | 'store'>('installed')
const storeEntries = ref<appType.PluginStoreEntry[]>([])
const storeLoading = ref(false)
const storeLoaded = ref(false)
const storeStale = ref(false)
const storeWarning = ref('')
const storeSearch = ref('')
const storeInstallingRepository = ref('')
const settingsModalVisible = ref(false)
const selectedPluginID = ref('')
const advancedSettingsMode = ref(false)

const selectedPlugin = computed(() =>
    pluginStatuses.value.find(plugin => plugin.manifest.id === selectedPluginID.value),
)

const localStoreMatchType = computed(() => {
  if (localInspection.value?.storeMatch === 'same-version') return 'success'
  if (localInspection.value?.storeMatch === 'different') return 'error'
  return 'warning'
})

const localStoreMatchText = computed(() => {
  const match = localInspection.value?.storeMatch
  if (match === 'same-version') return t('plugin.local_match_same_version')
  if (match === 'different') return t('plugin.local_match_different')
  return ''
})

const settingsModalTitle = computed(() =>
    t('plugin.settings_title', {
      name: selectedPlugin.value ? localizedPluginName(selectedPlugin.value.manifest) : '',
    }),
)

const selectedSettingFields = computed(() => {
  const properties = selectedPlugin.value?.manifest.settingsSchema?.properties ?? {}
  return Object.entries(properties).map(([key, schema]) => ({key, schema: schema as Record<string, any>}))
})

const localizedPluginEntry = (manifest: appType.PluginManifest) => {
  const entries = manifest.locales ?? {}
  const current = locale.value
  const language = current.split('-')[0]
  return entries[current] ?? entries[language] ?? entries.en ?? Object.values(entries)[0] ?? {}
}

const localizedPluginName = (manifest: appType.PluginManifest) =>
    localizedPluginEntry(manifest).name || manifest.name || manifest.id

const localizedPluginDescription = (manifest: appType.PluginManifest) =>
    localizedPluginEntry(manifest).description || ''

const hasPageInjectionPermission = (manifest: appType.PluginManifest) =>
    (manifest.permissions?.capabilities ?? []).some(capability =>
      capability === 'inject-page-script' || capability === 'page-bridge' || capability === 'enqueue-download')

const openPluginSettings = (id: string) => {
  selectedPluginID.value = id
  advancedSettingsMode.value = false
  settingsModalVisible.value = true
}

const localizedSchemaValue = (values?: Record<string, any>) => {
  if (!values) return {}
  const current = locale.value
  const language = current.split('-')[0]
  return values[current] ?? values[language] ?? values.en ?? Object.values(values)[0] ?? {}
}

const settingFieldLabel = (key: string, schema: Record<string, any>) =>
    localizedSchemaValue(schema['x-locales']).name || schema.title || key

const settingFieldDescription = (schema: Record<string, any>) =>
    localizedSchemaValue(schema['x-locales']).description || schema.description || ''

const settingEnumOptions = (schema: Record<string, any>) => {
  const labels = localizedSchemaValue(schema['x-enumLabels'])
  return (schema.enum ?? []).map((value: any) => ({value, label: labels[value] || String(value)}))
}

const toggleAdvancedSettings = () => {
  const id = selectedPluginID.value
  if (!advancedSettingsMode.value) {
    pluginSettingsJSON.value[id] = JSON.stringify(pluginSettingValues.value[id] ?? {}, null, 2)
    advancedSettingsMode.value = true
    return
  }
  try {
    pluginSettingValues.value[id] = JSON.parse(pluginSettingsJSON.value[id] || '{}')
    advancedSettingsMode.value = false
  } catch (_) {
    window?.$message?.error(t('plugin.settings_invalid'))
  }
}

const openResourceRules = () => router.push({path: '/setting', query: {tab: 'resource-rules'}})

const storeExtensionName = (extension: appType.PluginStoreEntry) =>
    extension.manifest ? localizedPluginName(extension.manifest) : extension.name

const installedPlugin = (extension: appType.PluginStoreEntry) =>
    extension.id ? pluginStatuses.value.find(plugin => plugin.manifest.id === extension.id) : undefined

const compareSemanticVersions = (left: string, right: string) => {
  const parse = (value: string) => {
    const match = value.match(/^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/)
    if (!match) return null
    return {core: match.slice(1, 4).map(Number), prerelease: match[4]?.split('.') ?? []}
  }
  const a = parse(left)
  const b = parse(right)
  if (!a || !b) return left.localeCompare(right)
  for (let index = 0; index < 3; index++) {
    if (a.core[index] !== b.core[index]) return a.core[index] - b.core[index]
  }
  if (!a.prerelease.length || !b.prerelease.length) {
    return a.prerelease.length === b.prerelease.length ? 0 : (a.prerelease.length ? -1 : 1)
  }
  for (let index = 0; index < Math.max(a.prerelease.length, b.prerelease.length); index++) {
    const av = a.prerelease[index]
    const bv = b.prerelease[index]
    if (av === undefined || bv === undefined) return av === bv ? 0 : (av === undefined ? -1 : 1)
    if (av === bv) continue
    const an = /^\d+$/.test(av)
    const bn = /^\d+$/.test(bv)
    if (an && bn) return Number(av) - Number(bv)
    if (an !== bn) return an ? -1 : 1
    return av.localeCompare(bv)
  }
  return 0
}

const storeCanUpdate = (extension: appType.PluginStoreEntry) => {
  const installed = installedPlugin(extension)
  return !!installed && !installed.builtin && !!extension.release &&
      compareSemanticVersions(extension.release.version, installed.manifest.version) > 0
}

const storeEntryPriority = (extension: appType.PluginStoreEntry) => {
  if (extension.status !== 'available' || !extension.manifest || !extension.release) return 3
  if (storeCanUpdate(extension)) return 0
  if (!installedPlugin(extension)) return 1
  return 2
}

const filteredStoreEntries = computed(() => {
  const keyword = storeSearch.value.trim().toLocaleLowerCase()
  const entries = keyword
    ? storeEntries.value.filter(extension => [
      storeExtensionName(extension), extension.description, extension.repository,
      extension.manifest?.author?.name, extension.owner,
    ].some(value => value?.toLocaleLowerCase().includes(keyword)))
    : storeEntries.value
  return [...entries].sort((left, right) => storeEntryPriority(left) - storeEntryPriority(right))
})

const storeUpdateCount = computed(() => {
  if (!storeLoaded.value || storeStale.value) return 0
  return storeEntries.value.filter(storeCanUpdate).length
})

const storeInstallDisabled = (extension: appType.PluginStoreEntry) => {
  if (extension.status !== 'available' || !extension.manifest || !extension.release) return true
  return !!installedPlugin(extension) && !storeCanUpdate(extension)
}

const storeInstallLabel = (extension: appType.PluginStoreEntry) => {
  if (extension.status !== 'available') return t('plugin.store_unavailable')
  if (storeCanUpdate(extension)) return t('plugin.store_update')
  if (installedPlugin(extension)) return t('plugin.store_installed')
  return t('plugin.store_install')
}

const pluginInstallErrorMessage = (error: unknown) => {
  if (axios.isAxiosError(error) && (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT')) {
    return t('plugin.install_timeout')
  }
  const message = String(error ?? '')
  if (message.includes('context deadline exceeded') || message.includes('Client.Timeout exceeded')) {
    return t('plugin.install_timeout')
  }
  return message
}

const loadPlugins = async () => {
  loading.value = true
  try {
    const res: appType.Res = await appApi.plugins()
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    pluginStatuses.value = res.data.plugins ?? []
    const settingsJSON: Record<string, string> = {}
    const settingsValues: Record<string, Record<string, any>> = {}
    for (const plugin of pluginStatuses.value) {
      if (plugin.manifest.id && plugin.manifest.id !== genericDetectorID) {
        const values = res.data.settings?.[plugin.manifest.id] ?? {}
        settingsValues[plugin.manifest.id] = JSON.parse(JSON.stringify(values))
        settingsJSON[plugin.manifest.id] = JSON.stringify(values, null, 2)
      }
    }
    pluginSettingValues.value = settingsValues
    pluginSettingsJSON.value = settingsJSON
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    loading.value = false
  }
}

const loadPluginStore = async (force = false) => {
  if (storeLoaded.value && !force) return
  storeLoading.value = true
  try {
    const res: appType.Res = await appApi.pluginStore()
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    storeEntries.value = res.data.index?.extensions ?? []
    storeStale.value = !!res.data.stale
    storeWarning.value = res.data.warning ?? ''
    storeLoaded.value = true
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    storeLoading.value = false
  }
}

const onTabChanged = (tab: string) => {
  if (tab === 'store') loadPluginStore()
}

const installFromStore = async (extension: appType.PluginStoreEntry) => {
  if (!extension.release || !extension.manifest || storeInstallDisabled(extension)) return
  const replacing = storeCanUpdate(extension)
  storeInstallingRepository.value = extension.repository
  try {
    const res: appType.Res = await appApi.installPlugin({
      id: extension.manifest.id,
      version: extension.release.version,
      approvePermissions: replacing,
    })
    if (res.code === 0) {
      window?.$message?.error(pluginInstallErrorMessage(res.message))
      return
    }
    window?.$message?.success(t(replacing ? 'plugin.store_update_success' : 'plugin.install_success', {
      name: localizedPluginName(res.data.manifest),
    }))
    await loadPlugins()
  } catch (error) {
    window?.$message?.error(pluginInstallErrorMessage(error))
  } finally {
    storeInstallingRepository.value = ''
  }
}

const inspectLocalPlugin = async () => {
  localInspecting.value = true
  try {
    const res: appType.Res = await appApi.inspectPluginFile()
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    if (res.data?.cancelled) return
    localInspection.value = res.data as LocalPluginInspection
    localInstallModalVisible.value = true
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    localInspecting.value = false
  }
}

const installLocalPlugin = async () => {
  if (!localInspection.value) return
  localInstalling.value = true
  try {
    const res: appType.Res = await appApi.installPluginFile({
      token: localInspection.value.token,
      replace: !!localInspection.value.installed,
      approvePermissions: !!localInspection.value.installed,
    })
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t(
        localInspection.value.installed ? 'plugin.store_update_success' : 'plugin.install_success',
        {name: localizedPluginName(res.data.manifest)},
    ))
    localInstallModalVisible.value = false
    localInspection.value = null
    await loadPlugins()
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    localInstalling.value = false
  }
}

const setPluginEnabled = async (id: string, enabled: boolean) => {
  updatingPluginID.value = id
  try {
    const res: appType.Res = await appApi.enablePlugin({id, enabled})
    if (res.code === 0) window?.$message?.error(res.message)
    else pluginStatuses.value = res.data ?? []
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    updatingPluginID.value = ''
  }
}

const reloadPlugins = async () => {
  reloading.value = true
  try {
    const res: appType.Res = await appApi.reloadPlugins()
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    await loadPlugins()
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    reloading.value = false
  }
}

const uninstallPlugin = async (id: string) => {
  uninstallingPluginID.value = id
  try {
    const res: appType.Res = await appApi.uninstallPlugin(id)
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t('plugin.uninstall_success'))
    await loadPlugins()
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    uninstallingPluginID.value = ''
  }
}

const rollbackPluginID = ref('')
const rollbackPlugin = async (id: string) => {
  rollbackPluginID.value = id
  try {
    const res: appType.Res = await appApi.rollbackPlugin(id)
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t('plugin.rollback_success'))
    await loadPlugins()
  } finally {
    rollbackPluginID.value = ''
  }
}

const savePluginSettings = async (id: string) => {
  let settings: Record<string, any>
  if (advancedSettingsMode.value) {
    try {
      settings = JSON.parse(pluginSettingsJSON.value[id] || '{}')
    } catch (_) {
      window?.$message?.error(t('plugin.settings_invalid'))
      return
    }
  } else {
    settings = JSON.parse(JSON.stringify(pluginSettingValues.value[id] ?? {}))
  }

  savingPluginID.value = id
  try {
    const res: appType.Res = await appApi.setPluginSettings({id, settings})
    if (res.code === 0) window?.$message?.error(res.message)
    else {
      pluginSettingValues.value[id] = JSON.parse(JSON.stringify(settings))
      pluginSettingsJSON.value[id] = JSON.stringify(settings, null, 2)
      window?.$message?.success(t('plugin.settings_saved'))
      settingsModalVisible.value = false
    }
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    savingPluginID.value = ''
  }
}

onMounted(() => {
  void Promise.all([loadPlugins(), loadPluginStore()])
})
</script>
