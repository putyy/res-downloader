<template>
  <section class="w-full [--wails-draggable:no-drag]">
    <div class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
      <button
          v-for="theme in appThemes"
          :key="theme.id"
          type="button"
          class="block w-full cursor-pointer rounded-xl border bg-app-surface p-2.5 text-left text-inherit transition-[border-color,box-shadow] duration-[180ms] ease-[ease] hover:border-[color:var(--preview-accent)] hover:shadow-[0_4px_12px_rgba(28,36,32,0.06)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[3px] focus-visible:outline-[color:var(--preview-accent)] motion-reduce:transition-none"
          :class="currentTheme === theme.id ? 'border-[color:var(--preview-accent)] shadow-[0_0_0_2px_var(--preview-soft)]' : 'border-app-border'"
          :style="{
            '--preview-accent': theme.preview.accent,
            '--preview-soft': theme.preview.accentSoft,
            '--preview-border': theme.preview.border,
          }"
          :aria-pressed="currentTheme === theme.id"
          @click="selectTheme(theme.id)"
      >
        <div
            class="relative flex h-[146px] overflow-hidden rounded-[7px] border border-[color:var(--preview-border)]"
            :class="{
              'theme-preview--sakura': theme.id === 'sakuraTheme',
            }"
            :style="{backgroundColor: theme.preview.background}"
            aria-hidden="true"
        >
          <div class="theme-preview__sidebar relative isolate flex w-12 shrink-0 flex-col items-center gap-1 px-1.5 pb-[9px] pt-3" :style="{backgroundColor: theme.preview.sidebar}">
            <span class="mb-[7px] h-[15px] w-[15px] rounded-[5px]" :style="{backgroundColor: theme.preview.sidebarAccent}"></span>
            <span
                v-for="item in 4"
                :key="item"
                class="flex h-[17px] items-center gap-1 rounded-[3px] pr-[5px]"
                :class="item === 2 && !theme.menuInset ? 'w-[calc(100%+6px)] -ml-1.5 self-start rounded-l-none pl-[11px]' : 'w-full pl-[5px]'"
                :style="{
                  color: item === 2 ? theme.menuSelection.text : theme.preview.sidebarText,
                  backgroundColor: item === 2 ? theme.menuSelection.background : 'transparent',
                  boxShadow: item === 2 ? theme.menuSelection.shadow : 'none',
                }"
            ><i class="h-[5px] w-[5px] rounded-[1px] border border-current"></i><b class="h-[3px] w-[15px] rounded-sm bg-current opacity-80"></b></span>
            <span class="mt-auto h-[9px] w-[9px] rounded-full opacity-60" :style="{backgroundColor: theme.preview.sidebarText}"></span>
          </div>
          <div class="theme-preview__content relative isolate min-w-0 flex-1 px-3 py-[15px]">
            <div class="flex items-center justify-between">
              <span class="h-1.5 w-[39%] rounded-sm opacity-80" :style="{backgroundColor: theme.preview.text}"></span>
              <span class="h-3 w-[26px] rounded-[3px]" :style="{backgroundColor: theme.preview.accent}"></span>
            </div>
            <div class="mb-2 mt-3 flex gap-2">
              <span class="h-[3px] w-[22px] rounded-sm" :style="{backgroundColor: theme.preview.accent}"></span>
              <span class="h-[3px] w-[22px] rounded-sm" :style="{backgroundColor: theme.preview.border}"></span>
            </div>
            <div class="overflow-hidden rounded border border-[color:var(--preview-border)]" :style="{backgroundColor: theme.preview.surface}">
              <div class="flex h-[15px] items-center px-[7px]" :style="{backgroundColor: theme.preview.surfaceMuted}">
                <span class="h-[3px] w-[32%] rounded-sm opacity-50" :style="{backgroundColor: theme.preview.textMuted}"></span>
              </div>
              <div v-for="row in 3" :key="row" class="flex h-[17px] items-center gap-1.5 border-t border-[color:var(--preview-border)] px-[7px]">
                <i class="h-[5px] w-[5px] rounded-[1px] border" :style="{borderColor: theme.preview.border}"></i>
                <span class="h-[3px] w-[45%] rounded-sm" :style="{backgroundColor: theme.preview.border}"></span>
                <b class="ml-auto h-1.5 w-[17px] rounded-sm" :style="{backgroundColor: row === 2 ? theme.preview.accentSoft : theme.preview.surfaceMuted}"></b>
              </div>
            </div>
          </div>
        </div>

        <div class="mt-3 flex items-start justify-between gap-3 px-0.5">
          <div>
            <div class="font-medium">{{ t(theme.nameKey) }}</div>
            <div class="mt-[7px] flex gap-[5px] [&_span]:h-[9px] [&_span]:w-[9px] [&_span]:rounded-full [&_span]:border [&_span]:border-black/[0.08]" aria-hidden="true">
              <span :style="{backgroundColor: theme.preview.sidebar}"></span>
              <span :style="{backgroundColor: theme.preview.accent}"></span>
              <span :style="{backgroundColor: theme.preview.background}"></span>
            </div>
          </div>
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
              class="border-app-border bg-app-surface-muted mt-1.5 h-3.5 w-3.5 shrink-0 rounded-full border"
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
.theme-preview--sakura .theme-preview__sidebar::before,
.theme-preview--sakura .theme-preview__content::before {
  content: '';
  position: absolute;
  z-index: -1;
  pointer-events: none;
  background: url('../../assets/image/sakura-branch.svg') no-repeat right top / contain;
}

.theme-preview--sakura .theme-preview__sidebar::before {
  bottom: 12px;
  left: 0;
  width: 36px;
  height: 30px;
  transform: rotate(180deg);
  opacity: 0.72;
}

.theme-preview--sakura .theme-preview__content::before {
  top: 0;
  right: 0;
  width: 100px;
  height: 78px;
  opacity: 0.3;
}
</style>
