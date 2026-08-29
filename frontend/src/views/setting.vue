<template>
  <div class="h-full overflow-hidden p-5" :key="renderKey">
    <NTabs
        v-model:value="activeTab"
        type="line"
        animated
        class="h-full"
        pane-wrapper-class="min-h-0 flex-1"
        pane-class="h-full overflow-y-auto [&::-webkit-scrollbar]:hidden"
    >
      <NTabPane name="basic" :tab="t('setting.basic_setting')">
        <NForm
            :model="formValue"
            size="medium"
            label-placement="left"
            label-width="auto"
            require-mark-placement="right-hanging"
            style="--wails-draggable:no-drag"
            class="w-[700px]"
        >
          <NFormItem :label="t('setting.save_dir')" path="SaveDirectory">
            <NInput :value="formValue.SaveDirectory" :placeholder="t('setting.save_dir')"/>
            <NButton strong secondary type="primary" @click="selectDir" class="ml-1">{{ t('common.select') }}</NButton>
          </NFormItem>

          <NFormItem :label="t('setting.filename_template')" path="FilenameTemplate">
            <NInput v-model:value="formValue.FilenameTemplate" :placeholder="filenameTemplateExample"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.filename_template_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.filename_conflict')" path="FilenameConflict">
            <NSelect v-model:value="formValue.FilenameConflict" :options="filenameConflictOptions"/>
          </NFormItem>

          <NFormItem :label="t('setting.auto_proxy')" path="AutoProxy">
            <NSwitch v-model:value="formValue.AutoProxy"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.auto_proxy_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.insert_tail')" path="InsertTail">
            <NSwitch v-model:value="formValue.InsertTail"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.insert_tail_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem>
            <n-popconfirm @positive-click="resetHandle">
              <template #trigger>
                <NButton tertiary type="error" :loading="resetting" style="--wails-draggable:no-drag">
                  {{ t("index.start_err_positiveText") }}
                </NButton>
              </template>
              {{ t("index.reset_app_tip") }}
            </n-popconfirm>
          </NFormItem>
        </NForm>
      </NTabPane>

      <NTabPane name="appearance" :tab="t('setting.appearance_setting')">
        <AppearanceSettings/>
      </NTabPane>

      <NTabPane name="resource-rules" :tab="t('setting.capture_rules')" style="--wails-draggable:no-drag">
        <div class="w-[900px] space-y-3">
          <NAlert type="info" :show-icon="false">
            {{ t('setting.capture_rules_tip') }}
          </NAlert>
          <NSpace>
            <NButton type="primary" secondary @click="saveCaptureRules">{{ t('setting.capture_rules_save') }}</NButton>
            <NButton secondary @click="resetCaptureRules">{{ t('setting.capture_rules_reset') }}</NButton>
          </NSpace>
          <NDynamicInput v-model:value="captureRules" :on-create="createCaptureRule">
            <template #default="{ value, index }">
              <NCard size="small" class="mb-3 w-full">
                <template #header>
                  <div class="grid grid-cols-[1fr_1fr_120px_auto] gap-2 items-center">
                    <NInput v-model:value="value.id" :placeholder="t('setting.capture_rule_id')"/>
                    <NInput v-model:value="value.name" :placeholder="t('setting.capture_rule_name')"/>
                    <NInputNumber v-model:value="value.priority" :placeholder="t('setting.capture_rule_priority')"/>
                    <NSwitch v-model:value="value.enabled"/>
                  </div>
                </template>
                <NCollapse>
                  <NCollapseItem :title="t('setting.capture_rule_details')" :name="value.id">

                    <div class="text-xs font-medium text-gray-500 mb-1">{{ t('setting.capture_match') }}</div>
                    <div class="grid grid-cols-2 gap-3">
                      <div>
                        <div class="text-xs text-gray-500 mb-1">MIME</div>
                        <NDynamicTags v-model:value="value.match.mime"/>
                      </div>
                      <div>
                        <div class="text-xs text-gray-500 mb-1">URL</div>
                        <NDynamicTags v-model:value="value.match.url"/>
                      </div>
                      <div>
                        <div class="text-xs text-gray-500 mb-1">Content-Disposition</div>
                        <NDynamicTags v-model:value="value.match.contentDisposition"/>
                      </div>
                      <div>
                        <div class="text-xs text-gray-500 mb-1">HTTP Status</div>
                        <NSelect v-model:value="value.match.status" multiple tag :options="httpStatusOptions"/>
                      </div>
                      <div class="grid grid-cols-2 gap-2">
                        <NInputNumber v-model:value="value.match.minSize" :min="0"
                                      :placeholder="t('setting.capture_min_size')"/>
                        <NInputNumber v-model:value="value.match.maxSize" :min="0"
                                      :placeholder="t('setting.capture_max_size')"/>
                      </div>
                    </div>

                    <NDivider class="!my-3"/>
                    <div class="text-xs font-medium text-gray-500 mb-1">{{ t('setting.capture_output') }}</div>
                    <div class="grid grid-cols-3 gap-2">
                      <NInput v-model:value="value.resource.kind" :placeholder="t('setting.capture_kind')"/>
                      <NInput v-model:value="value.resource.role" :placeholder="t('setting.capture_role')"/>
                      <NInput v-model:value="value.resource.extension" placeholder=".mp4"/>
                      <NSelect v-model:value="value.resource.executor" :options="captureExecutorOptions"
                               :placeholder="t('setting.capture_executor')"/>
                      <NSelect v-model:value="value.resource.previewRenderer" clearable
                               :options="previewRendererOptions" :placeholder="t('setting.capture_preview')"/>
                      <NSelect v-model:value="value.resource.capabilities" multiple :options="resourceCapabilityOptions"
                               :placeholder="t('setting.capture_capabilities')"/>
                    </div>
                    <div class="mt-2 text-xs text-gray-400">#{{ index + 1 }} · {{
                        t('setting.capture_rule_order_tip')
                      }}
                    </div>
                  </NCollapseItem>
                </NCollapse>
              </NCard>
            </template>
          </NDynamicInput>
        </div>
      </NTabPane>

      <NTabPane name="media" :tab="t('setting.media_engine')" style="--wails-draggable:no-drag">
        <MediaEngineSettings
            :config="formValue"
            @update:ffmpeg="(value: any) => formValue.FFmpegPath = value"
            @update:ffprobe="(value: any) => formValue.FFprobePath = value"
        />
      </NTabPane>

      <NTabPane name="certificate" :tab="t('setting.certificate')" style="--wails-draggable:no-drag">
        <CertificateSettings :certificate-url="store.baseUrl + '/api/certificate/download'"/>
      </NTabPane>

      <NTabPane name="advanced" :tab="t('setting.advanced_setting')">
        <NForm
            :model="formValue"
            size="medium"
            label-placement="left"
            label-width="auto"
            require-mark-placement="right-hanging"
            style="--wails-draggable:no-drag"
            class="w-[700px]"
        >
          <NFormItem label="Host" path="Host" :validation-status="hostValidationFeedback==='' ? undefined : 'error'" :feedback="hostValidationFeedback">
            <NInput v-model:value="formValue.Host" placeholder="127.0.0.1"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.restart_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem label="Port" path="Port" :validation-status="portValidationFeedback==='' ? undefined : 'error'" :feedback="portValidationFeedback">
            <NInput v-model:value="formValue.Port" placeholder="8899"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.restart_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.upstream_proxy')" path="UpstreamProxy">
            <NInput v-model:value="formValue.UpstreamProxy" placeholder="http://127.0.0.1:7890"/>
            <NSwitch v-model:value="formValue.OpenProxy" class="ml-1"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.upstream_proxy_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.download_proxy')" path="DownloadProxy">
            <NSwitch v-model:value="formValue.DownloadProxy"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.download_proxy_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.connections')" path="TaskNumber">
            <NInputNumber v-model:value="formValue.TaskNumber" :min="2" :max="64"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.connections_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.down_number')" path="DownNumber">
            <NInputNumber v-model:value="formValue.DownNumber" :min="1" :max="10"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.down_number_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem label="UserAgent" path="UserAgent">
            <NInput v-model:value="formValue.UserAgent" placeholder="UserAgent"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.user_agent_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem label="Headers" path="Headers">
            <NInput v-model:value="formValue.UseHeaders" placeholder="User-Agent,Referer,Authorization,Cookie"/>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.use_headers_tip") }}
            </NTooltip>
          </NFormItem>

          <NFormItem :label="t('setting.interception_policies')" path="InterceptionPolicies">
            <NDynamicInput v-model:value="formValue.InterceptionPolicies" :on-create="createInterceptionPolicy">
              <template #default="{ value }">
                <NCard size="small" class="mb-2 w-full">
                  <div class="grid grid-cols-[1fr_130px_auto] gap-2 items-center">
                    <NInput v-model:value="value.name" :placeholder="t('setting.policy_name')"/>
                    <NSelect v-model:value="value.action" :options="interceptionActionOptions"/>
                    <NSwitch v-model:value="value.enabled"/>
                  </div>
                  <div class="mt-2 text-xs text-gray-500">{{ t('setting.policy_domains') }}</div>
                  <NDynamicTags v-model:value="value.domains"/>
                  <div class="mt-2 text-xs text-gray-500">{{ t('setting.policy_exclude') }}</div>
                  <NDynamicTags v-model:value="value.exclude"/>
                </NCard>
              </template>
            </NDynamicInput>
            <NTooltip trigger="hover">
              <template #trigger>
                <NIcon size="18" class="ml-1 text-gray-500">
                  <HelpCircleOutline/>
                </NIcon>
              </template>
              {{ t("setting.interception_policies_tip") }}
            </NTooltip>
          </NFormItem>
        </NForm>
      </NTabPane>

    </NTabs>

    <NModal v-model:show="showResetAuthorization" preset="dialog" :title="t('index.reset_app_authorize')">
      <div class="space-y-3">
        <div class="text-sm text-gray-500">{{ t('index.reset_app_authorize_tip') }}</div>
        <NInput
            v-model:value="resetPassword"
            type="password"
            show-password-on="click"
            :placeholder="t('components.password_placeholder')"
            @keyup.enter="prepareReset"
        />
      </div>
      <template #action>
        <NButton :disabled="resetting" @click="showResetAuthorization = false">{{ t('common.cancel') }}</NButton>
        <NButton type="error" :loading="resetting" @click="prepareReset">{{ t('common.submit') }}</NButton>
      </template>
    </NModal>
  </div>
</template>

<script lang="ts" setup>
import {HelpCircleOutline} from "@vicons/ionicons5"
import {computed, onMounted, ref, watch} from "vue"
import {useIndexStore} from "@/stores"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {useI18n} from 'vue-i18n'
import {useRoute} from 'vue-router'
import {isValidHost, isValidPort} from '@/func'
import {NButton, NIcon} from "naive-ui"
import * as bind from "../../wailsjs/go/app/Bind"
import MediaEngineSettings from '@/components/settings/MediaEngineSettings.vue'
import CertificateSettings from '@/components/settings/CertificateSettings.vue'
import AppearanceSettings from '@/components/settings/AppearanceSettings.vue'

const {t} = useI18n()
const route = useRoute()
const store = useIndexStore()
const activeTab = ref(route.query.tab === 'resource-rules' ? 'resource-rules' : 'basic')

const filenameTemplateExample = "{{author}}/{{title|default:resource|sanitize|truncate:80}}_{{date:20060102}}.{{ext}}"
const filenameConflictOptions = computed(() => [
  {value: 'rename', label: t('setting.filename_conflict_rename')},
  {value: 'overwrite', label: t('setting.filename_conflict_overwrite')},
  {value: 'skip', label: t('setting.filename_conflict_skip')},
])
const interceptionActionOptions = computed(() => [
  {value: 'mitm', label: t('setting.policy_action_mitm')},
  {value: 'pass', label: t('setting.policy_action_pass')},
])
let policySequence = 0
const createInterceptionPolicy = (): appType.InterceptionPolicy => ({
  id: `policy-${Date.now()}-${++policySequence}`,
  name: t('setting.policy_name'),
  enabled: true,
  domains: [],
  exclude: [],
  action: 'mitm',
})

const formValue = ref<appType.Config>(Object.assign({}, store.globalConfig))

const renderKey = ref(999)
const genericDetectorID = "builtin.generic-detector"
const captureRules = ref<appType.CaptureRule[]>([])
const defaultCaptureRules = ref<appType.CaptureRule[]>([])
let captureRuleSequence = 0

const httpStatusOptions = [200, 206, 304].map(value => ({value, label: String(value)}))
const captureExecutorOptions = computed(() => [
  {value: 'http-file', label: t('setting.capture_executor_http')},
  {value: 'hls', label: t('setting.capture_executor_hls')},
  {value: 'ffmpeg-hls', label: t('setting.capture_executor_ffmpeg_stream')},
])
const previewRendererOptions = [
  {value: 'image', label: 'Image'},
  {value: 'audio', label: 'Audio'},
  {value: 'video', label: 'Video'},
  {value: 'pdf', label: 'PDF'},
  {value: 'text', label: 'Text'},
]
const resourceCapabilityOptions = computed(() => [
  {value: 'download', label: t('setting.capture_cap_download')},
  {value: 'preview', label: t('setting.capture_cap_preview')},
  {value: 'open', label: t('setting.capture_cap_open')},
  {value: 'copy', label: t('setting.capture_cap_copy')},
])

const createCaptureRule = (): appType.CaptureRule => ({
  id: `custom-rule-${Date.now()}-${++captureRuleSequence}`,
  name: t('setting.capture_rule_name'),
  enabled: true,
  priority: 100,
  match: {mime: [], url: [], contentDisposition: [], status: [200, 206], minSize: 0, maxSize: 0},
  resource: {
    kind: 'media.video', role: 'video', extension: '.mp4', executor: 'http-file',
    capabilities: ['download', 'preview', 'open', 'copy'], previewRenderer: 'video', previewMode: 'proxy',
  },
})

const cloneValue = <T, >(value: T): T => JSON.parse(JSON.stringify(value))

const normalizeCaptureRules = (rules: any[]): appType.CaptureRule[] => (rules ?? []).map((raw: any) => ({
  id: String(raw?.id ?? ''),
  name: String(raw?.name ?? ''),
  enabled: raw?.enabled !== false,
  priority: Number(raw?.priority ?? 0),
  match: {
    mime: Array.isArray(raw?.match?.mime) ? raw.match.mime : [],
    url: Array.isArray(raw?.match?.url) ? raw.match.url : [],
    contentDisposition: Array.isArray(raw?.match?.contentDisposition) ? raw.match.contentDisposition : [],
    status: Array.isArray(raw?.match?.status) ? raw.match.status.map(Number) : [],
    minSize: Number(raw?.match?.minSize ?? 0),
    maxSize: Number(raw?.match?.maxSize ?? 0),
  },
  resource: {
    kind: String(raw?.resource?.kind ?? ''),
    role: String(raw?.resource?.role ?? ''),
    extension: String(raw?.resource?.extension ?? ''),
    executor: raw?.resource?.executor || 'http-file',
    capabilities: Array.isArray(raw?.resource?.capabilities) ? raw.resource.capabilities : [],
    previewRenderer: String(raw?.resource?.previewRenderer ?? ''),
    previewMode: String(raw?.resource?.previewMode ?? 'proxy'),
  },
}))

const hostValidationFeedback = ref("")
const portValidationFeedback = ref("")
const resetting = ref(false)
const showResetAuthorization = ref(false)
const resetPassword = ref('')

watch(formValue.value, () => {
  formValue.value.Port = formValue.value.Port.trim()
  formValue.value.Host = formValue.value.Host.trim()

  if (!isValidHost(formValue.value.Host)) {
    hostValidationFeedback.value = t("setting.host_format_error")
    return
  } else {
    hostValidationFeedback.value = ''
  }

  if (!isValidPort(parseInt(formValue.value.Port))) {
    portValidationFeedback.value = t("setting.port_format_error")
    return
  } else {
    portValidationFeedback.value = ''
  }
  store.setConfig(formValue.value)
}, {deep: true})

watch(() => {
  return store.globalConfig.Theme
}, () => {
  formValue.value.Theme = store.globalConfig.Theme
})

watch(() => store.globalConfig.Locale, () => {
  formValue.value.Locale = store.globalConfig.Locale
  renderKey.value++
})

const selectDir = () => {
  appApi.openDirectoryDialog().then((res: any) => {
    if (res.code === 1) {
      formValue.value.SaveDirectory = res.data.folder
    }
  }).catch((err: any) => {
    window?.$message?.error(err)
  })
}

const resetHandle = () => {
  if (['darwin', 'linux'].includes(store.envInfo.platform)) {
    showResetAuthorization.value = true
    return
  }
  void prepareReset()
}

const prepareReset = async () => {
  if (showResetAuthorization.value && !resetPassword.value) {
    window.$message?.error(t('components.password_empty'))
    return
  }
  resetting.value = true
  try {
    await bind.PrepareReset(resetPassword.value)
    localStorage.clear()
    showResetAuthorization.value = false
    bind.ResetApp()
  } catch (error: any) {
    window.$message?.error(t('index.reset_app_failed', {message: String(error?.message ?? error)}), {duration: 10000})
  } finally {
    resetPassword.value = ''
    resetting.value = false
  }
}

const loadCaptureRules = () => {
  appApi.plugins().then((res: appType.Res) => {
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    const plugin = (res.data.plugins ?? []).find(
        (item: appType.PluginStatus) => item.manifest.id === genericDetectorID,
    )
    const defaults = plugin?.manifest.settingsSchema?.properties?.rules?.default ?? []
    defaultCaptureRules.value = normalizeCaptureRules(cloneValue(defaults))
    captureRules.value = normalizeCaptureRules(cloneValue(res.data.settings?.[genericDetectorID]?.rules ?? defaults))
  })
}

const saveCaptureRules = () => {
  const rules = normalizeCaptureRules(cloneValue(captureRules.value))
  for (const rule of rules) {
    const capabilities = new Set(rule.resource.capabilities ?? [])
    if (rule.resource.previewRenderer) capabilities.add('preview')
    else capabilities.delete('preview')
    rule.resource.capabilities = Array.from(capabilities)
  }
  appApi.setPluginSettings({id: genericDetectorID, settings: {rules}}).then((res: appType.Res) => {
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    captureRules.value = rules
    window?.$message?.success(t('setting.capture_rules_saved'))
    loadCaptureRules()
  })
}

const resetCaptureRules = () => {
  captureRules.value = normalizeCaptureRules(cloneValue(defaultCaptureRules.value))
}

onMounted(loadCaptureRules)
</script>
