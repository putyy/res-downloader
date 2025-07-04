<template>
  <div class="flex items-center">
    <span>
      {{ t('index.operation') }}
    </span>
    <NTooltip trigger="hover">
      <template #trigger>
        <NIcon size="18" class="ml-1 text-gray-500">
          <HelpCircleOutline/>
        </NIcon>
      </template>
      <div class="flex flex-col">
        <div class="flex items-center justify-start p-1.5">
          <n-icon size="28"
                  class="text-emerald-600 dark:text-emerald-400 bg-emerald-500/20 dark:bg-emerald-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-emerald-500/40 transition-colors">
            <DownloadOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.direct_download") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="text-blue-600 dark:text-blue-300 bg-blue-500/20 dark:bg-blue-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-blue-500/40 transition-colors"
          >
            <LinkOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="text-blue-500 dark:text-blue-200 bg-blue-400/20 dark:bg-blue-400/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-blue-400/40 transition-colors"
          >
            <GlobeOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.open_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="text-orange-400 dark:text-red-300 bg-orange-500/20 dark:bg-orange-200/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-orange-200/40 transition-colors"
          >
            <LockOpenSharp/>
          </n-icon>
          <span class="ml-1">{{ t("index.video_decode") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="text-sky-400 dark:text-sky-200 bg-sky-500/20 dark:bg-sky-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-sky-500/40 transition-colors"
          >
            <CopyOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_data") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="text-red-500 dark:text-red-300 bg-red-500/20 dark:bg-red-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-red-500/40 transition-colors"
          >
            <TrashOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.delete_row") }}</span>
        </div>

      </div>
    </NTooltip>
  </div>
</template>
<script setup lang="ts">
import {useI18n} from "vue-i18n"
import {
  CopyOutline,
  DownloadOutline,
  GlobeOutline,
  HelpCircleOutline, LinkOutline, LockOpenSharp,
  TrashOutline
} from "@vicons/ionicons5"

const {t} = useI18n()
</script>