<template>
  <div class="flex items-center">
    <span>
      {{ t('index.operation') }}
    </span>
    <NPopover trigger="hover">
      <template #trigger>
        <NIcon size="18" class="app-muted-text ml-1">
          <HelpCircleOutline/>
        </NIcon>
      </template>
      <div class="flex flex-col">
        <div class="flex items-center justify-start p-1.5">
          <n-icon size="28"
                  class="resource-action-icon resource-action-icon--primary rounded-full flex items-center justify-center p-1.5 cursor-pointer">
            <DownloadOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.direct_download") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon size="28"
                  class="resource-action-icon resource-action-icon--danger rounded-full flex items-center justify-center p-1.5 cursor-pointer">
            <CloseOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.cancel_down") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <LinkOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <GlobeOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.open_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <CopyOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_data") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="resource-action-icon resource-action-icon--danger rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <TrashOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.delete_row") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-2 cursor-pointer"
          >
            <GridSharp/>
          </n-icon>
          <span class="ml-1">{{ t("index.more_operation") }}</span>
        </div>
      </div>
    </NPopover>
  </div>
</template>
<script setup lang="ts">
import {useI18n} from "vue-i18n"
import {CloseOutline, CopyOutline, DownloadOutline, GlobeOutline, GridSharp, HelpCircleOutline, LinkOutline, TrashOutline} from "@vicons/ionicons5"

const {t} = useI18n()
</script>
