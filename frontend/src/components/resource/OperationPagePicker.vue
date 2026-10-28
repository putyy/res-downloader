<template>
  <NModal
    :show="show"
    preset="card"
    :title="t('operations.choose_page')"
    :bordered="false"
    :closable="!loading"
    :mask-closable="!loading"
    :close-on-esc="!loading"
    class="app-card w-[min(480px,calc(100vw-32px))] [--wails-draggable:no-drag]"
    @update:show="updateShow"
  >
    <p class="text-app-muted text-sm mb-4">{{ t("operations.multiple_pages") }}</p>
    <NSelect
      v-model:value="selectedID"
      :options="options"
      :render-label="renderLabel"
      :placeholder="t('operations.choose_page')"
      :disabled="loading"
      :loading="loading"
      :aria-label="t('operations.choose_page')"
    />
    <p v-if="selectedSession" class="text-app-muted text-xs break-all mt-3">
      {{ selectedSession.pageUrl }}
    </p>
    <template #footer>
      <div class="flex justify-end gap-2">
        <NButton :disabled="loading" @click="updateShow(false)">{{ t("plugin.cancel") }}</NButton>
        <NButton type="primary" :loading="loading" :disabled="!selectedSession || loading" @click="submit">
          {{ t("operations.submit") }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<script setup lang="ts">
import {computed, h, ref, watch} from "vue"
import {NButton, NModal, NSelect, type SelectOption} from "naive-ui"
import {useI18n} from "vue-i18n"
import type {OperationSession} from "@/types/operations"

const props = defineProps<{
  show: boolean
  sessions: OperationSession[]
  loading: boolean
}>()
const emit = defineEmits<{
  "update:show": [show: boolean]
  submit: [pageSessionId: string]
}>()
const {t} = useI18n()
const selectedID = ref<string | null>(null)
const selectedSession = computed(() => props.sessions.find((session) => session.pageSessionId === selectedID.value && session.connected && session.ready))
const options = computed(() =>
  props.sessions.map((session) => ({
    value: session.pageSessionId,
    label: session.title || session.pageUrl,
    pageUrl: session.pageUrl,
    disabled: !session.connected || !session.ready,
  })),
)
const renderLabel = (option: SelectOption, selected: boolean) =>
  selected
    ? `${option.label || ""} · ${String(option.value).slice(-8)}`
    : h("div", {class: "min-w-0 py-1"}, [
        h("div", {class: "truncate"}, String(option.label || "")),
        h("div", {class: "text-xs text-app-muted truncate"}, `${option.pageUrl || ""} · ${String(option.value).slice(-8)}`),
      ])
watch(
  () => props.show,
  () => {
    selectedID.value = null
  },
)
watch(
  () => props.sessions,
  () => {
    if (!selectedSession.value) selectedID.value = null
  },
  {deep: true},
)
const updateShow = (show: boolean) => {
  if (!props.loading) emit("update:show", show)
}
const submit = () => {
  if (!props.loading && selectedSession.value) emit("submit", selectedSession.value.pageSessionId)
}
</script>
