<template>
  <NModal :show="true" :mask-closable="!saving" :close-on-esc="!saving" @update:show="(show) => !show && emit('close')">
    <NCard
      class="max-h-[90vh] w-[min(880px,calc(100vw-32px))] [--wails-draggable:no-drag] [&>.n-card__content]:overflow-y-auto"
      :title="title"
      :aria-label="title"
      :bordered="false"
      role="dialog"
      aria-modal="true"
    >
      <div class="grid gap-6 md:grid-cols-2">
        <div class="grid grid-cols-2 gap-x-3 gap-y-4">
          <div v-for="key in themeColorKeys" :key="key" class="min-w-0">
            <div class="mb-1 text-sm text-app-text">{{ t(`setting.theme_colors.${key}`) }}</div>
            <NColorPicker :value="theme.preview[key]" :modes="['hex']" :show-alpha="false" :disabled="saving" @update:value="draft[key] = $event ?? undefined">
              <template #trigger="{value, onClick, ref: triggerRef}">
                <button
                  :ref="triggerRef"
                  type="button"
                  class="flex w-full items-center gap-2 rounded-md bg-app-surface-muted px-2.5 py-1.5 text-left font-mono text-sm text-app-text hover:bg-app-surface-hover focus-visible:outline-app-accent"
                  :aria-label="`${t('setting.theme_colors.' + key)} ${value}`"
                  :disabled="saving"
                  @click="onClick"
                >
                  <span class="h-4 w-4 shrink-0 rounded-sm ring-1 ring-inset ring-app-border" :style="{backgroundColor: value ?? undefined}"></span>
                  {{ value }}
                </button>
              </template>
            </NColorPicker>
          </div>
        </div>
        <figure class="m-0 min-w-0">
          <figcaption class="mb-2 text-sm font-medium text-app-heading">{{ t("index.preview") }}</figcaption>
          <div class="grid grid-cols-[auto_minmax(0,1fr)] overflow-hidden rounded-lg bg-app-background" :style="theme.cssVars">
            <div class="space-y-1 bg-app-sidebar px-1.5 py-3 text-xs text-app-sidebar-text">
              <div class="rounded px-2 py-1.5">{{ t("menu.index") }}</div>
              <div class="rounded bg-[var(--app-sidebar-active)] px-2 py-1.5 font-medium text-app-sidebar-accent">{{ t("menu.plugin") }}</div>
              <div class="rounded px-2 py-1.5">{{ t("menu.tasks") }}</div>
            </div>
            <div class="min-w-0 px-3 py-4">
              <div class="mb-3 text-sm font-semibold text-app-heading">{{ t("plugin.installed_tab") }}</div>
              <div class="rounded-lg bg-app-surface p-3">
                <div class="font-semibold text-app-heading">{{ t("setting.theme_preview_plugin") }}</div>
                <div class="mt-1 break-all text-xs text-app-muted">builtin.generic-detector</div>
                <p class="my-2 text-sm text-app-text">{{ t("setting.theme_preview_description") }}</p>
                <span class="inline-block rounded bg-app-accent-soft px-2 py-1 text-xs font-medium text-app-accent">{{ t("plugin.resource_rules") }}</span>
              </div>
            </div>
          </div>
        </figure>
      </div>
      <template #footer>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <NButton secondary :disabled="saving" @click="draft = {}">{{ t("setting.theme_restore") }}</NButton>
          <div class="flex gap-2">
            <NButton :disabled="saving" @click="emit('close')">{{ t("common.cancel") }}</NButton>
            <NButton type="primary" :loading="saving" @click="save">{{ t("setting.theme_save") }}</NButton>
          </div>
        </div>
      </template>
    </NCard>
  </NModal>
</template>

<script setup lang="ts">
import {computed, ref} from "vue"
import {useI18n} from "vue-i18n"
import {useIndexStore} from "@/stores"
import {resolveAppTheme, themeColorKeys} from "@/themes"
import type {AppThemeColors, AppThemeName} from "@/themes"

const props = defineProps<{themeId: AppThemeName}>()
const emit = defineEmits<{close: []}>()
const {t} = useI18n()
const store = useIndexStore()
const saving = ref(false)
const preset = resolveAppTheme(props.themeId)
const draft = ref<AppThemeColors>({...store.globalConfig.ThemeColors?.[props.themeId]})
const theme = computed(() => resolveAppTheme(props.themeId, draft.value))
const title = computed(() => t("setting.theme_editor_title", {name: t(preset.nameKey)}))

const save = async () => {
  if (saving.value) return
  saving.value = true
  try {
    const colors: AppThemeColors = {}
    for (const key of themeColorKeys) {
      const color = theme.value.preview[key]
      if (color.toLowerCase() !== preset.preview[key].toLowerCase()) colors[key] = color
    }
    const byTheme = {...store.globalConfig.ThemeColors}
    if (Object.keys(colors).length) byTheme[props.themeId] = colors
    else delete byTheme[props.themeId]
    const activeId = resolveAppTheme(store.globalConfig.Theme).id
    store.setConfig({Theme: activeId, ThemeColors: byTheme})
    if (await store.flushConfig()) emit("close")
  } finally {
    saving.value = false
  }
}
</script>
