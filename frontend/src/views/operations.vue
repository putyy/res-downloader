<template>
  <div class="h-full overflow-auto p-5 text-app-text [--wails-draggable:no-drag]">
    <div class="max-w-7xl mx-auto space-y-4">
      <header class="flex items-start justify-between gap-4">
        <div class="flex min-w-0 flex-col gap-2">
          <div class="min-w-0 flex ml-[-8px]">
            <NButton quaternary circle size="small" class="shrink-0" :aria-label="t('operations.back')" @click="goBack">
              <template #icon
                ><NIcon :size="20"><ChevronBackOutline /></NIcon
              ></template>
            </NButton>
            <h1 class="text-xl font-semibold">{{ t("operations.title") }}</h1>
          </div>
          <p class="text-app-muted text-sm mt-1">{{ t("operations.subtitle") }}</p>
        </div>
        <NButton class="shrink-0" :loading="loading" @click="refresh">{{ t("operations.refresh") }}</NButton>
      </header>
      <NAlert v-if="error" type="error" :title="t('operations.request_failed')">{{ error }}</NAlert>
      <NAlert v-if="status && !status.available" type="error">{{ t("operations.errors.storage_unavailable") }}</NAlert>
      <NAlert v-if="syncUnavailable" type="warning">{{ t("operations.sync_unavailable") }}</NAlert>
      <NTabs v-model:value="tab" type="line" animated>
        <NTabPane name="run" :tab="t('operations.run')">
          <div class="grid grid-cols-1 xl:grid-cols-[minmax(240px,0.8fr)_minmax(360px,1.5fr)] gap-4">
            <NCard class="app-card" :bordered="false" size="small" :title="t('operations.catalog')">
              <NInput v-model:value="search" clearable :placeholder="t('operations.search')" class="mb-3" />
              <NEmpty v-if="!filteredOperations.length" :description="t('operations.no_operations')" class="py-10" />
              <div class="space-y-2 max-h-[600px] overflow-auto">
                <button
                  v-for="operation in filteredOperations"
                  :key="operationKey(operation)"
                  type="button"
                  class="w-full rounded-lg border p-3 text-left transition-colors"
                  :class="selectedKey === operationKey(operation) ? 'border-current text-app-accent bg-app-surface-muted' : 'border-transparent hover:bg-app-surface-muted'"
                  @click="selectOperation(operation)"
                >
                  <div class="font-medium">{{ operationName(operation) }}</div>
                  <div class="text-xs text-app-muted mt-1 break-all">{{ operation.pluginId }} · {{ operation.operationId }}</div>
                  <div class="flex gap-2 mt-2">
                    <NTag size="small" :type="operation.available ? 'success' : 'warning'">{{ t(operation.available ? "operations.available" : "operations.unavailable") }}</NTag
                    ><NTag size="small">{{ operation.definition.category }}</NTag>
                  </div>
                </button>
              </div>
            </NCard>
            <NCard v-if="selected" class="app-card" :bordered="false" size="small" :title="operationName(selected)">
              <p class="text-app-muted mb-3">{{ operationDescription(selected) }}</p>
              <div class="flex flex-wrap gap-2 mb-4">
                <NTag size="small">v{{ selected.pluginVersion }}</NTag
                ><NTag v-for="effect in selected.definition.effects" :key="effect" size="small" type="warning">{{ t(`operations.effects.${effect}`, effect) }}</NTag>
              </div>
              <div class="rounded-lg bg-app-surface-muted p-3 mb-4 space-y-3">
                <div class="flex items-center justify-between gap-3">
                  <span>{{ t("operations.plugin_automation") }}</span
                  ><NSwitch :value="pluginAutomation" :disabled="saving" @update:value="setAutomation(selected!.pluginId, $event)" />
                </div>
                <div class="flex items-center justify-between gap-3">
                  <span>{{ t("operations.operation_automation") }}</span
                  ><NSwitch :value="selected.automationEnabled" :disabled="saving || !pluginAutomation" @update:value="setAutomation(operationKey(selected!), $event)" />
                </div>
                <p class="text-xs text-app-muted">{{ t("operations.automation_hint") }}</p>
              </div>
              <NCollapse class="mb-4"
                ><NCollapseItem :title="t('operations.contract')" name="schema">
                  <div class="text-xs text-app-muted mb-2">
                    {{
                      t("operations.policy", {
                        timeout: selected.definition.timeoutSeconds || 120,
                        retry: t(selected.definition.safeRetry ? "operations.yes" : "operations.no"),
                        cancel: t(selected.definition.cancellable ? "operations.yes" : "operations.no"),
                      })
                    }}
                  </div>
                  <pre class="json-block">{{
                    pretty({
                      inputSchema: selected.definition.inputSchema,
                      outputSchema: selected.definition.outputSchema,
                      examples: selected.definition.examples || [],
                    })
                  }}</pre>
                </NCollapseItem></NCollapse
              >
              <NAlert v-if="resourceId" type="info" class="mb-4"
                >{{ t("operations.resource_context", {id: resourceId}) }}
                <NButton
                  text
                  @click="
                    router.replace({
                      path: '/operations',
                      query: {pluginId: selected.pluginId, operationId: selected.operationId},
                    })
                  "
                  >{{ t("operations.clear") }}</NButton
                ></NAlert
              >
              <NAlert v-if="retryOf" type="warning" class="mb-4"
                >{{ t("operations.retry_hint", {id: retryOf}) }} <NButton text @click="retryOf = ''">{{ t("operations.clear") }}</NButton></NAlert
              >
              <NForm label-placement="top">
                <NFormItem :label="t('operations.page')">
                  <div class="w-full space-y-2">
                    <NSelect v-model:value="pageSessionId" :options="sessionOptions" :placeholder="t('operations.choose_page')" clearable />
                    <p v-if="!matchingSessions.length" class="text-amber-600 text-xs">
                      {{ t("operations.no_pages") }}
                    </p>
                    <p v-else-if="matchingSessions.length > 1 && !pageSessionId" class="text-amber-600 text-xs">
                      {{ t("operations.multiple_pages") }}
                    </p>
                    <p v-if="matchingSessions.length && !matchingSessions.some(sessionUsable)" class="text-amber-600 text-xs">
                      {{ t("operations.readiness_hint") }}
                    </p>
                    <div v-if="selectedSession" class="text-xs text-app-muted break-all">
                      {{ selectedSession.pageUrl }}<br />{{ t("operations.login") }}: {{ t(`operations.login_states.${selectedSession.login}`) }} ·
                      {{ t(selectedSession.ready ? "operations.ready" : "operations.not_ready") }}
                    </div>
                    <NAlert
                      v-if="
                        selectedSession &&
                        (!selectedSession.ready || !selectedSession.connected || (selected.definition.requiresLogin && selectedSession.login !== 'authenticated'))
                      "
                      type="warning"
                      size="small"
                      >{{ t("operations.readiness_hint") }}</NAlert
                    >
                  </div>
                </NFormItem>
                <NFormItem :label="t('operations.mode')"
                  ><NRadioGroup v-model:value="mode" :disabled="!!resourceId || !!retryOf"
                    ><NRadioButton value="single">{{ t("operations.single") }}</NRadioButton
                    ><NRadioButton value="batch">{{ t("operations.batch") }}</NRadioButton></NRadioGroup
                  ></NFormItem
                >
                <template v-if="mode === 'single' && !resourceId">
                  <NFormItem v-for="field in fields" :key="field.key" :label="`${field.schema.title || field.key}${requiredFields.includes(field.key) ? ' *' : ''}`">
                    <div class="w-full">
                      <NSelect
                        v-if="Array.isArray(field.schema.enum) && field.schema.enum.every((value: any) => typeof value === 'string' || typeof value === 'number')"
                        :value="fieldValues[field.key]"
                        :options="field.schema.enum.map((value: any) => ({label: String(value), value}))"
                        clearable
                        @update:value="setField(field.key, $event)"
                      />
                      <NSwitch v-else-if="field.schema.type === 'boolean'" :value="!!fieldValues[field.key]" @update:value="setField(field.key, $event)" />
                      <NInputNumber
                        v-else-if="['integer', 'number'].includes(field.schema.type)"
                        :value="fieldValues[field.key] ?? null"
                        :min="field.schema.minimum"
                        :max="field.schema.maximum"
                        :precision="field.schema.type === 'integer' ? 0 : undefined"
                        class="w-full"
                        @update:value="setField(field.key, $event)"
                      />
                      <NInput v-else :value="fieldValues[field.key] || ''" :type="field.schema['x-sensitive'] ? 'password' : 'text'" @update:value="setField(field.key, $event)" />
                      <p class="text-xs text-app-muted mt-1">{{ field.schema.description }}</p>
                    </div>
                  </NFormItem>
                </template>
                <NFormItem v-if="!resourceId" :label="t(mode === 'batch' ? 'operations.batch_input' : 'operations.input_json')">
                  <div class="w-full">
                    <NInput
                      v-model:value="inputJSON"
                      type="textarea"
                      :rows="mode === 'batch' ? 7 : 4"
                      :placeholder="mode === 'batch' ? '[{}, {}]' : '{}'"
                      @update:value="syncFields"
                    />
                    <p v-if="mode === 'batch'" class="text-xs text-app-muted mt-2">
                      {{ t("operations.batch_hint", {limit: status?.limits.batch || 32}) }}
                    </p>
                  </div>
                </NFormItem>
                <div v-if="!resourceId" class="grid grid-cols-2 gap-3">
                  <NFormItem :label="t('operations.cursor')"><NInput v-model:value="cursor" clearable /></NFormItem>
                  <NFormItem :label="t('operations.limit')"><NInputNumber v-model:value="limit" :min="1" :max="status?.limits.pageItems || 100" class="w-full" /></NFormItem>
                </div>
                <NFormItem v-if="!resourceId" :label="t('operations.idempotency')"><NInput v-model:value="idempotencyKey" clearable /></NFormItem>
              </NForm>
              <NButton type="primary" :loading="submitting" :disabled="!canSubmit" @click="submit">{{
                t(mode === "batch" ? "operations.submit_batch" : "operations.submit")
              }}</NButton>
            </NCard>
            <NCard v-else class="app-card" :bordered="false"><NEmpty :description="t('operations.choose_operation')" class="py-24" /></NCard>
          </div>
        </NTabPane>
        <NTabPane name="history" :tab="t('operations.history')">
          <NCard class="app-card" :bordered="false" size="small">
            <div class="flex flex-wrap gap-3 mb-4">
              <NInput v-model:value="historyPlugin" :placeholder="t('operations.plugin_filter')" clearable class="!w-64" @update:value="resetHistory" /><NSelect
                v-model:value="historyState"
                :options="stateOptions"
                clearable
                :placeholder="t('operations.state_filter')"
                class="!w-44"
                @update:value="resetHistory"
              />
            </div>
            <NDataTable :columns="historyColumns" :data="history.items" :row-key="(row: OperationExecution) => row.executionId" :bordered="false" :scroll-x="760" />
            <div class="flex justify-end mt-4">
              <NPagination v-model:page="historyPage" :page-size="20" :item-count="history.total" @update:page="loadHistory().catch(fail)" />
            </div>
          </NCard>
        </NTabPane>
        <NTabPane name="settings" :tab="t('operations.retention')">
          <NCard class="app-card max-w-2xl" :bordered="false" :title="t('operations.retention')">
            <p class="text-app-muted mb-5">{{ t("operations.retention_hint") }}</p>
            <NForm label-placement="top"
              ><NFormItem :label="t('operations.history_days')"><NInputNumber v-model:value="historyDays" :min="1" :max="30" /></NFormItem
              ><NFormItem :label="t('operations.result_hours')"><NInputNumber v-model:value="resultHours" :min="1" :max="24" /></NFormItem
            ></NForm>
            <NButton type="primary" :loading="saving" :disabled="!status" @click="saveRetention">{{ t("operations.save") }}</NButton>
            <NDivider />
            <p class="text-sm text-app-muted mb-3">{{ t("operations.clean_hint") }}</p>
            <div class="flex gap-3">
              <NPopconfirm @positive-click="clean(false)"
                ><template #trigger
                  ><NButton :disabled="saving">{{ t("operations.clean_results") }}</NButton></template
                >{{ t("operations.clean_results_confirm") }}</NPopconfirm
              ><NPopconfirm @positive-click="clean(true)"
                ><template #trigger
                  ><NButton type="error" secondary :disabled="saving">{{ t("operations.clean_history") }}</NButton></template
                >{{ t("operations.clean_history_confirm") }}</NPopconfirm
              >
            </div>
          </NCard>
        </NTabPane>
      </NTabs>
      <NCard v-if="batch" class="app-card" :bordered="false" size="small" :title="t('operations.batch_details')">
        <template #header-extra
          ><NButton v-if="batch.items.some(executionActive)" size="small" @click="cancelBatch">{{ t("operations.cancel_batch") }}</NButton></template
        >
        <div class="text-xs text-app-muted mb-3 break-all">{{ batch.batchId }}</div>
        <div class="flex flex-wrap gap-2 mb-3">
          <NTag v-for="(count, state) in batch.counts" :key="state" :type="stateType(String(state))">{{ t(`operations.states.${state}`) }} {{ count }}</NTag>
        </div>
        <p class="text-xs text-app-muted mb-3">{{ t("operations.batch_result_hint") }}</p>
        <NDataTable :columns="historyColumns" :data="batch.items" :row-key="(row: OperationExecution) => row.executionId" :bordered="false" :scroll-x="760" />
      </NCard>
    </div>
    <NDrawer v-model:show="detailsVisible" :width="Math.min(viewportWidth - 32, 700)" placement="right">
      <NDrawerContent :title="t('operations.details')" closable>
        <div v-if="execution" class="space-y-4">
          <div class="flex justify-between items-center">
            <NTag :type="stateType(execution.state)">{{ t(`operations.states.${execution.state}`) }}</NTag
            ><NButton v-if="executionActive(execution) && (execution.state === 'queued' || executionDefinition?.cancellable)" size="small" @click="cancelExecution">{{
              t("operations.cancel")
            }}</NButton>
          </div>
          <NProgress v-if="execution.progress != null && !syncUnavailable" type="line" :percentage="execution.progress" />
          <NAlert v-if="execution.errorCode" type="error">{{ executionMessage(execution, t) }}</NAlert>
          <NAlert v-if="execution.certainty === 'unknown'" type="warning">{{ t("operations.uncertain") }}</NAlert>
          <NAlert v-if="execution.state === 'interrupted'" type="warning">{{ t("operations.interrupted_hint") }}</NAlert>
          <NAlert v-if="execution.cancelRequested || execution.cancelConfirmed" type="info">{{
            t(execution.cancelConfirmed ? "operations.cancel_confirmed" : "operations.cancel_requested")
          }}</NAlert>
          <NDescriptions :column="1" label-placement="left" bordered size="small">
            <NDescriptionsItem :label="t('operations.execution_id')"
              ><span class="break-all">{{ execution.executionId }}</span></NDescriptionsItem
            >
            <NDescriptionsItem :label="t('operations.operation')">{{ execution.pluginId }} / {{ execution.operationId }} · v{{ execution.pluginVersion }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('operations.created')">{{ time(execution.createdAt) }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('operations.page')"
              ><span class="break-all">{{ execution.pageSessionId }}</span></NDescriptionsItem
            >
            <NDescriptionsItem :label="t('operations.source')">{{ execution.source }}</NDescriptionsItem>
            <NDescriptionsItem :label="t('operations.result_status')">{{ t(`operations.results.${execution.resultStatus}`) }}</NDescriptionsItem>
            <NDescriptionsItem v-if="execution.resultExpiresAt" :label="t('operations.expires')">{{ time(execution.resultExpiresAt) }}</NDescriptionsItem>
          </NDescriptions>
          <NButton v-if="execution.batchId" size="small" @click="openBatch(execution.batchId)">{{ t("operations.batch_details") }}</NButton>
          <NButton v-if="canRetry" size="small" @click="prepareRetry">{{ t("operations.retry") }}</NButton>
          <NCollapse
            ><NCollapseItem :title="t('operations.input_summary')" name="input"
              ><p class="text-xs text-app-muted mb-2">{{ t("operations.summary_hint") }}</p>
              <pre class="json-block">{{ pretty(execution.inputSummary) }}</pre>
            </NCollapseItem></NCollapse
          >
          <div v-if="execution.result !== undefined">
            <h3 class="font-medium mb-2">{{ t("operations.result") }}</h3>
            <NAlert v-if="execution.resultProjection" type="info" class="mb-2">{{ t("operations.projection_hint") }}</NAlert>
            <pre class="json-block">{{ pretty(execution.result) }}</pre>
            <NButton v-if="execution.result?.pagination?.hasMore && execution.result?.pagination?.cursor" size="small" class="mt-2" @click="prepareNextPage">{{
              t("operations.next_page")
            }}</NButton>
          </div>
          <NEmpty v-else :description="t(`operations.results.${execution.resultStatus}`)" />
          <NAlert v-if="execution.resultStatus === 'cleaned'" type="info">{{ t("operations.cleaned_hint") }}</NAlert>
          <div v-if="execution.resourceIds?.length" class="space-y-2">
            <h3 class="font-medium">{{ t("operations.resources") }}</h3>
            <NButton v-for="id in execution.resourceIds" :key="id" text type="primary" @click="router.push({path: '/index', query: {resourceId: id}})">{{ id }}</NButton>
          </div>
          <div v-if="execution.downloadTaskIds?.length" class="space-y-2">
            <h3 class="font-medium">{{ t("operations.downloads") }}</h3>
            <NButton v-for="id in execution.downloadTaskIds" :key="id" text type="primary" @click="router.push({path: '/tasks', query: {taskId: id}})">{{ id }}</NButton>
          </div>
          <div v-if="execution.artifactIds?.length" class="space-y-2">
            <h3 class="font-medium">{{ t("operations.artifacts") }}</h3>
            <NButton v-for="id in execution.artifactIds" :key="id" size="small" @click="openArtifact(id)">{{ id }}</NButton>
          </div>
          <NCard v-if="artifact" size="small" :title="t('operations.artifact_details')">
            <NDescriptions :column="1" size="small"
              ><NDescriptionsItem :label="t('operations.state')">{{ t(`operations.artifact_states.${artifact.status}`, artifact.status) }}</NDescriptionsItem
              ><NDescriptionsItem label="MIME">{{ artifact.mime }}</NDescriptionsItem
              ><NDescriptionsItem :label="t('operations.bytes')">{{ artifact.size }}</NDescriptionsItem
              ><NDescriptionsItem v-if="artifact.expiresAt" :label="t('operations.expires')">{{ time(artifact.expiresAt) }}</NDescriptionsItem
              ><NDescriptionsItem v-if="artifact.output" :label="t('operations.output')">{{ artifact.output }}</NDescriptionsItem></NDescriptions
            >
            <NButton v-if="isTextArtifact" size="small" @click="readArtifact(false)">{{ t("operations.read_text") }}</NButton>
            <pre v-if="artifactText" class="json-block mt-3">{{ artifactText }}</pre>
            <NButton v-if="textHasMore" size="small" class="mt-2" @click="readArtifact(true)">{{ t("operations.read_more") }}</NButton>
          </NCard>
        </div>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<script setup lang="ts">
import {computed, h, onMounted, onUnmounted, ref, watch} from "vue"
import {useI18n} from "vue-i18n"
import {useRoute, useRouter} from "vue-router"
import {NButton, NIcon, NTag, type DataTableColumns} from "naive-ui"
import {ChevronBackOutline} from "@vicons/ionicons5"
import api, {OperationAPIError} from "@/api/operations"
import appApi from "@/api/app"
import type {OperationInfo, OperationSession, OperationExecution, OperationBatch, OperationPage, OperationStatus, OperationArtifact, OperationRequest} from "@/types/operations"
import {executionActive, executionMessage, operationErrorMessage} from "@/services/operations"
const {t, locale} = useI18n()
const route = useRoute(),
  router = useRouter()
function goBack() {
  if (router.options.history.state.back) router.back()
  else void router.replace("/plugins")
}
const operations = ref<OperationInfo[]>([]),
  sessions = ref<OperationSession[]>([])
const status = ref<OperationStatus>(),
  selectedKey = ref(""),
  search = ref(""),
  tab = ref("run")
const loading = ref(false),
  submitting = ref(false),
  saving = ref(false),
  error = ref(""),
  syncUnavailable = ref(false)
const pageSessionId = ref<string | null>(null),
  mode = ref("single"),
  inputJSON = ref("{}"),
  fieldValues = ref<Record<string, any>>({})
const cursor = ref(""),
  limit = ref<number | null>(20),
  idempotencyKey = ref(""),
  retryOf = ref("")
const resourceId = computed(() => String(route.query.resourceId || "")),
  actionId = computed(() => String(route.query.actionId || ""))
const history = ref<OperationPage>({items: [], total: 0, offset: 0, hasMore: false}),
  historyPlugin = ref(String(route.query.pluginId || "")),
  historyState = ref<string | null>(null),
  historyPage = ref(1)
const execution = ref<OperationExecution>(),
  batch = ref<OperationBatch>(),
  detailsVisible = ref(false)
const artifact = ref<OperationArtifact>(),
  artifactText = ref(""),
  textOffset = ref(0),
  textHasMore = ref(false)
const historyDays = ref<number | null>(7),
  resultHours = ref<number | null>(2),
  viewportWidth = ref(window.innerWidth)
let timer: ReturnType<typeof setTimeout> | undefined,
  disposed = false,
  historyGeneration = 0,
  detailGeneration = 0
const operationKey = (info: OperationInfo) => `${info.pluginId}/${info.operationId}`
const selected = computed(() => operations.value.find((info) => operationKey(info) === selectedKey.value))
const operationName = (info: OperationInfo) => info.definition.locales?.[locale.value]?.name || info.definition.name || info.operationId
const operationDescription = (info: OperationInfo) => info.definition.locales?.[locale.value]?.description || info.definition.description || ""
const filteredOperations = computed(() =>
  operations.value.filter((info) => `${operationName(info)} ${info.pluginId} ${info.operationId}`.toLowerCase().includes(search.value.toLowerCase())),
)
const pluginAutomation = computed(() => (selected.value ? status.value?.settings.automation[selected.value.pluginId] !== false : false))
const matchingSessions = computed(() =>
  sessions.value.filter(
    (session) =>
      session.pluginId === selected.value?.pluginId && session.scriptId === selected.value?.definition.pageScript && session.operations?.includes(selected.value!.operationId),
  ),
)
const sessionUsable = (session: OperationSession) => session.connected && session.ready && (!selected.value?.definition.requiresLogin || session.login === "authenticated")
const sessionOptions = computed(() =>
  matchingSessions.value.map((session) => ({
    value: session.pageSessionId,
    label: `${session.title || session.pageUrl} · ${session.pageSessionId.slice(-8)} · ${t(sessionUsable(session) ? "operations.ready" : "operations.not_ready")}`,
    disabled: !sessionUsable(session),
  })),
)
const selectedSession = computed(() => matchingSessions.value.find((session) => session.pageSessionId === pageSessionId.value))
const canSubmit = computed(() => !!selected.value && status.value?.available && !!selectedSession.value && sessionUsable(selectedSession.value))
const fields = computed(() =>
  Object.entries(selected.value?.definition.inputSchema.properties || {})
    .filter(([, schema]) => ["string", "number", "integer", "boolean"].includes((schema as any).type))
    .map(([key, schema]) => ({key, schema: schema as Record<string, any>})),
)
const requiredFields = computed<string[]>(() => selected.value?.definition.inputSchema.required || [])
const executionDefinition = computed(
  () => operations.value.find((info) => info.pluginId === execution.value?.pluginId && info.operationId === execution.value?.operationId)?.definition,
)
const canRetry = computed(
  () =>
    execution.value &&
    ["failed", "timed_out", "interrupted"].includes(execution.value.state) &&
    execution.value.certainty !== "unknown" &&
    executionDefinition.value?.safeRetry &&
    !execution.value.resourceId,
)
const isTextArtifact = computed(
  () =>
    artifact.value &&
    (artifact.value.mime.startsWith("text/") || artifact.value.mime === "application/json") &&
    artifact.value.size <= 1048576 &&
    artifact.value.status === "available",
)
const states = ["queued", "running", "succeeded", "failed", "cancelled", "timed_out", "interrupted"]
const stateOptions = computed(() => states.map((value) => ({value, label: t(`operations.states.${value}`)})))
const stateType = (state: string): "success" | "error" | "warning" | "info" | "default" =>
  state === "succeeded"
    ? "success"
    : ["failed", "timed_out"].includes(state)
      ? "error"
      : state === "running"
        ? "info"
        : ["queued", "interrupted"].includes(state)
          ? "warning"
          : "default"
const time = (value: number) => new Date(value).toLocaleString(locale.value)
const pretty = (value: unknown) => JSON.stringify(value ?? null, null, 2)
const fail = (cause: unknown) => {
  error.value = operationErrorMessage(cause, t)
  window.$message?.error(error.value)
}
const historyColumns = computed<DataTableColumns<OperationExecution>>(() => [
  {title: "#", key: "index", width: 45, render: (row) => (row.batchId ? row.index + 1 : "—")},
  {
    title: t("operations.operation"),
    key: "operationId",
    minWidth: 190,
    render: (row) => h("div", [h("div", row.operationId), h("div", {class: "text-xs text-app-muted"}, row.pluginId)]),
  },
  {
    title: t("operations.state"),
    key: "state",
    width: 115,
    render: (row) => h(NTag, {size: "small", type: stateType(row.state)}, {default: () => t(`operations.states.${row.state}`)}),
  },
  {
    title: t("operations.created"),
    key: "createdAt",
    width: 170,
    render: (row) => time(row.createdAt),
  },
  {
    title: t("operations.result_status"),
    key: "resultStatus",
    width: 150,
    render: (row) => t(`operations.results.${row.resultStatus}`),
  },
  {
    title: "",
    key: "actions",
    width: 85,
    render: (row) =>
      h(
        NButton,
        {
          size: "small",
          text: true,
          type: "primary",
          onClick: () => openExecution(row.executionId),
        },
        {default: () => t("operations.details")},
      ),
  },
])
function syncFields() {
  try {
    const parsed = JSON.parse(inputJSON.value)
    if (parsed && !Array.isArray(parsed) && typeof parsed === "object") fieldValues.value = parsed
  } catch {
    /* Keep incomplete JSON editable. */
  }
}
function setField(key: string, value: unknown) {
  if (value == null) delete fieldValues.value[key]
  else fieldValues.value[key] = value
  inputJSON.value = pretty(fieldValues.value)
}
function selectOperation(info: OperationInfo) {
  if (resourceId.value && (info.pluginId !== route.query.pluginId || info.operationId !== route.query.operationId)) {
    void router.replace({
      path: "/operations",
      query: {pluginId: info.pluginId, operationId: info.operationId},
    })
    return
  }
  selectedKey.value = operationKey(info)
  inputJSON.value = pretty(info.definition.examples?.[0] || {})
  fieldValues.value = {}
  syncFields()
  pageSessionId.value = null
  cursor.value = ""
  idempotencyKey.value = ""
  retryOf.value = ""
  mode.value = "single"
  chooseUniqueSession()
}
function chooseUniqueSession() {
  const usable = matchingSessions.value.filter(sessionUsable)
  if (!pageSessionId.value && usable.length === 1 && matchingSessions.value.length === 1) pageSessionId.value = usable[0].pageSessionId
  if (pageSessionId.value && !matchingSessions.value.some((s) => s.pageSessionId === pageSessionId.value)) pageSessionId.value = null
}
watch(mode, (value) => {
  inputJSON.value = value === "batch" ? "[{}]" : "{}"
  fieldValues.value = {}
})
watch(matchingSessions, chooseUniqueSession)
async function loadHistory() {
  const generation = ++historyGeneration
  const page = await api.history(historyPlugin.value, historyState.value || "", (historyPage.value - 1) * 20)
  if (generation === historyGeneration && !disposed) history.value = page
}
function resetHistory() {
  historyPage.value = 1
  void loadHistory().catch(fail)
}
async function refresh() {
  loading.value = true
  error.value = ""
  try {
    const [infos, pages, serviceStatus] = await Promise.all([api.list(), api.sessions(), api.status()])
    operations.value = infos
    sessions.value = pages
    status.value = serviceStatus
    historyDays.value = serviceStatus.settings.historyDays
    resultHours.value = serviceStatus.settings.resultHours
    if (!selectedKey.value) {
      const match = infos.find((info) => info.pluginId === route.query.pluginId && (!route.query.operationId || info.operationId === route.query.operationId))
      if (match) selectOperation(match)
    }
    chooseUniqueSession()
    await loadHistory()
    syncUnavailable.value = false
  } catch (cause) {
    fail(cause)
  } finally {
    loading.value = false
  }
}
async function submit() {
  if (!selected.value || !canSubmit.value) return
  submitting.value = true
  error.value = ""
  try {
    if (resourceId.value) {
      const response = await appApi.runResourceAction({
        id: resourceId.value,
        actionId: actionId.value,
        pageSessionId: pageSessionId.value!,
      })
      if (response.code !== 1) throw new OperationAPIError(response.data?.errorCode || "invalid_resource", response.message)
      await openExecution(response.data.executionId)
    } else {
      let input: unknown
      try {
        input = JSON.parse(inputJSON.value)
      } catch {
        throw new OperationAPIError("invalid_input", t("operations.invalid_json"))
      }
      const inputs = mode.value === "batch" ? input : [input]
      if (
        !Array.isArray(inputs) ||
        !inputs.length ||
        inputs.length > (status.value?.limits.batch || 32) ||
        inputs.some((item) => !item || Array.isArray(item) || typeof item !== "object")
      )
        throw new OperationAPIError("invalid_input", t("operations.invalid_json"))
      const items: OperationRequest[] = inputs.map((item) => ({
        pluginId: selected.value!.pluginId,
        operationId: selected.value!.operationId,
        pageSessionId: pageSessionId.value!,
        input: item,
        cursor: cursor.value || undefined,
        limit: limit.value || undefined,
        retryOf: retryOf.value || undefined,
      }))
      if (mode.value === "batch") batch.value = await api.batch(items, idempotencyKey.value || undefined)
      else {
        const result = await api.invoke({
          ...items[0],
          idempotencyKey: idempotencyKey.value || undefined,
        })
        await openExecution(result.executionId)
      }
    }
    await loadHistory()
    window.$message?.success(t("operations.submitted"))
  } catch (cause) {
    fail(cause)
  } finally {
    submitting.value = false
  }
}
async function openExecution(id: string) {
  const generation = ++detailGeneration
  try {
    const current = await api.execution(id)
    if (generation !== detailGeneration || disposed) return
    execution.value = current
    artifact.value = undefined
    artifactText.value = ""
    detailsVisible.value = true
  } catch (cause) {
    fail(cause)
  }
}
async function openBatch(id: string) {
  try {
    batch.value = await api.batchGet(id)
    detailsVisible.value = false
  } catch (cause) {
    fail(cause)
  }
}
async function cancelExecution() {
  if (!execution.value) return
  try {
    await api.cancel(execution.value.executionId)
    execution.value = await api.execution(execution.value.executionId)
    await loadHistory()
  } catch (cause) {
    fail(cause)
  }
}
async function cancelBatch() {
  if (!batch.value) return
  try {
    await api.cancelBatch(batch.value.batchId)
    batch.value = await api.batchGet(batch.value.batchId)
    await loadHistory()
  } catch (cause) {
    fail(cause)
  }
}
function prepareRetry() {
  const old = execution.value
  const info = operations.value.find((info) => info.pluginId === old?.pluginId && info.operationId === old?.operationId)
  if (!old || !info || !canRetry.value) return
  selectOperation(info)
  inputJSON.value = "{}"
  fieldValues.value = {}
  retryOf.value = old.executionId
  detailsVisible.value = false
  tab.value = "run"
}
function prepareNextPage() {
  if (!execution.value) return
  const info = operations.value.find((info) => info.pluginId === execution.value!.pluginId && info.operationId === execution.value!.operationId)
  if (!info) return
  if (selectedKey.value !== operationKey(info)) selectOperation(info)
  cursor.value = execution.value.result.pagination.cursor
  idempotencyKey.value = ""
  retryOf.value = ""
  tab.value = "run"
  detailsVisible.value = false
  window.$message?.info(t("operations.next_page_hint"))
}
async function setAutomation(key: string, enabled: boolean) {
  if (!status.value) return
  saving.value = true
  try {
    status.value = await api.settings({
      ...status.value.settings,
      automation: {...status.value.settings.automation, [key]: enabled},
    })
    operations.value = await api.list()
  } catch (cause) {
    fail(cause)
  } finally {
    saving.value = false
  }
}
async function saveRetention() {
  if (!status.value) return
  saving.value = true
  try {
    status.value = await api.settings({
      ...status.value.settings,
      historyDays: historyDays.value || 1,
      resultHours: resultHours.value || 1,
    })
    window.$message?.success(t("operations.saved"))
  } catch (cause) {
    fail(cause)
  } finally {
    saving.value = false
  }
}
async function clean(all: boolean) {
  saving.value = true
  try {
    await api.clean(all, true)
    batch.value = undefined
    execution.value = undefined
    detailsVisible.value = false
    await loadHistory()
    window.$message?.success(t("operations.cleaned"))
  } catch (cause) {
    fail(cause)
  } finally {
    saving.value = false
  }
}
async function openArtifact(id: string) {
  try {
    artifact.value = await api.artifact(id)
    artifactText.value = ""
    textOffset.value = 0
    textHasMore.value = false
  } catch (cause) {
    fail(cause)
  }
}
async function readArtifact(append: boolean) {
  if (!artifact.value) return
  try {
    const result = await api.text(artifact.value.artifactId, append ? textOffset.value : 0)
    artifactText.value = (append ? artifactText.value : "") + result.text
    textOffset.value = result.nextOffset
    textHasMore.value = result.truncated
  } catch (cause) {
    fail(cause)
  }
}
async function poll() {
  if (disposed) return
  try {
    if (document.hidden) return
    await loadHistory()
    sessions.value = await api.sessions()
    operations.value = await api.list()
    if (execution.value && detailsVisible.value) {
      const id = execution.value.executionId
      const current = await api.execution(id)
      if (execution.value?.executionId === id) execution.value = current
    }
    if (batch.value) batch.value = await api.batchGet(batch.value.batchId)
    syncUnavailable.value = false
  } catch {
    syncUnavailable.value = true
  } finally {
    if (!disposed) timer = setTimeout(poll, 2500)
  }
}
const resize = () => {
  viewportWidth.value = window.innerWidth
}
onMounted(async () => {
  window.addEventListener("resize", resize)
  await refresh()
  if (route.query.executionId) await openExecution(String(route.query.executionId))
  if (!disposed) timer = setTimeout(poll, 2500)
})
onUnmounted(() => {
  disposed = true
  clearTimeout(timer)
  window.removeEventListener("resize", resize)
})
</script>

<style scoped>
.json-block {
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  max-height: 420px;
  overflow: auto;
  padding: 12px;
  border-radius: 8px;
  background: var(--app-surface-muted, rgba(127, 127, 127, 0.08));
}
</style>
