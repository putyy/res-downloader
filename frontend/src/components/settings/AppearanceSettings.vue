<template>
  <section class="w-full" style="--wails-draggable:no-drag">
    <div class="theme-grid">
      <button
          v-for="theme in appThemes"
          :key="theme.id"
          type="button"
          class="theme-card group"
          :class="{'theme-card--active': currentTheme === theme.id}"
          :style="{'--preview-accent': theme.preview.accent}"
          :aria-pressed="currentTheme === theme.id"
          @click="selectTheme(theme.id)"
      >
        <div
            class="theme-preview"
            :style="{backgroundColor: theme.preview.background, color: theme.preview.text}"
        >
          <div class="theme-preview__sidebar" :style="{backgroundColor: theme.preview.surface}">
            <span class="theme-preview__logo" :style="{backgroundColor: theme.preview.accent}"></span>
            <span :style="{backgroundColor: theme.preview.muted}"></span>
            <span :style="{backgroundColor: theme.preview.accent}"></span>
            <span :style="{backgroundColor: theme.preview.muted}"></span>
          </div>
          <div class="theme-preview__content">
            <div class="flex items-center justify-between">
              <span class="theme-preview__title" :style="{backgroundColor: theme.preview.text}"></span>
              <span class="theme-preview__button" :style="{backgroundColor: theme.preview.accent}"></span>
            </div>
            <div class="theme-preview__panel" :style="{backgroundColor: theme.preview.surface}">
              <span :style="{backgroundColor: theme.preview.muted}"></span>
              <span :style="{backgroundColor: theme.preview.muted}"></span>
              <span class="!w-3/5" :style="{backgroundColor: theme.preview.accent}"></span>
            </div>
          </div>
          <template v-if="theme.id === 'sakuraTheme'">
            <span class="sakura-petal sakura-petal--one"></span>
            <span class="sakura-petal sakura-petal--two"></span>
            <span class="sakura-petal sakura-petal--three"></span>
          </template>
        </div>

        <div class="mt-3 flex items-start justify-between gap-3 px-0.5">
          <div class="font-medium">{{ t(theme.nameKey) }}</div>
          <NIcon
              v-if="currentTheme === theme.id"
              :size="21"
              class="mt-0.5 shrink-0"
              :color="theme.preview.accent"
          >
            <CheckmarkCircle/>
          </NIcon>
          <span
              v-else
              class="theme-choice-dot mt-1.5 h-3.5 w-3.5 shrink-0 rounded-full border"
          ></span>
        </div>
      </button>
    </div>
  </section>
</template>

<script lang="ts" setup>
import {computed} from 'vue'
import {NIcon} from 'naive-ui'
import {CheckmarkCircle} from '@vicons/ionicons5'
import {useI18n} from 'vue-i18n'
import {useIndexStore} from '@/stores'
import {appThemes} from '@/themes'
import type {AppThemeName} from '@/themes'

const {t} = useI18n()
const store = useIndexStore()
const currentTheme = computed(() => store.globalConfig.Theme)

const selectTheme = (theme: AppThemeName) => {
  if (theme === currentTheme.value) return
  store.setConfig({Theme: theme})
}
</script>

<style scoped>
.theme-card {
  display: block;
  width: 100%;
  border: 1px solid var(--app-border);
  border-radius: 16px;
  padding: 12px;
  background: var(--app-surface);
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease;
}

.theme-card:hover {
  border-color: var(--preview-accent);
  box-shadow: var(--app-card-shadow);
  transform: translateY(-2px);
}

.theme-card--active {
  border-color: var(--preview-accent);
  box-shadow: inset 0 0 0 1px var(--preview-accent), var(--app-card-shadow);
}

.theme-choice-dot {
  border-color: var(--app-border);
  background: var(--app-surface-muted);
}

.theme-preview {
  position: relative;
  display: flex;
  height: 112px;
  overflow: hidden;
  border-radius: 11px;
  border: 1px solid rgba(127, 127, 127, 0.14);
}

.theme-preview__sidebar {
  display: flex;
  width: 38px;
  flex-direction: column;
  align-items: center;
  gap: 9px;
  padding-top: 12px;
  box-shadow: 4px 0 14px rgba(31, 41, 55, 0.04);
}

.theme-preview__sidebar span {
  width: 14px;
  height: 5px;
  border-radius: 999px;
}

.theme-preview__sidebar .theme-preview__logo {
  width: 17px;
  height: 17px;
  margin-bottom: 4px;
}

.theme-preview__content {
  flex: 1;
  padding: 15px;
}

.theme-preview__title {
  width: 52px;
  height: 7px;
  border-radius: 999px;
  opacity: 0.72;
}

.theme-preview__button {
  width: 30px;
  height: 13px;
  border-radius: 999px;
}

.theme-preview__panel {
  display: flex;
  height: 58px;
  margin-top: 12px;
  flex-direction: column;
  gap: 8px;
  border-radius: 8px;
  padding: 12px;
  box-shadow: 0 5px 16px rgba(31, 41, 55, 0.05);
}

.theme-preview__panel span {
  width: 82%;
  height: 5px;
  border-radius: 999px;
}

.sakura-petal {
  position: absolute;
  width: 7px;
  height: 11px;
  border-radius: 70% 30% 70% 30%;
  background: rgba(226, 130, 158, 0.58);
  transform: rotate(28deg);
}

.sakura-petal--one { right: 16px; bottom: 13px; }
.sakura-petal--two { right: 29px; bottom: 24px; transform: rotate(72deg) scale(0.8); }
.sakura-petal--three { right: 12px; bottom: 35px; transform: rotate(118deg) scale(0.65); }
</style>
