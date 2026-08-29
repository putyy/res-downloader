<template>
  <div style="--wails-draggable:no-drag" class="grid grid-cols-3 gap-1.5">
    <n-icon
        v-if="canDownload"
        size="30"
        class="resource-action-icon resource-action-icon--primary rounded-full flex items-center justify-center p-1.5 cursor-pointer"
        :title="t(isLive ? 'index.start_recording' : 'index.start_down')"
        @click="action('down')"
    >
      <DownloadOutline/>
    </n-icon>

    <n-icon
        size="28"
        class="resource-action-icon resource-action-icon--danger rounded-full flex items-center justify-center p-1.5 cursor-pointer"
        @click="action('delete')"
    >
      <TrashOutline/>
    </n-icon>

    <NPopover placement="bottom" trigger="hover">
      <template #trigger>
        <NIcon size="30"
               class="resource-action-icon rounded-full flex items-center justify-center p-2 cursor-pointer">
          <GridSharp/>
        </NIcon>
      </template>
      <div class="flex flex-col">
        <div class="flex items-center justify-start p-1.5 cursor-pointer" v-if="isDownloading"
             @click="action('cancel')">
          <n-icon
              size="28"
              class="resource-action-icon resource-action-icon--danger rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <CloseOutline/>
          </n-icon>
          <span class="ml-1">{{ t(isLive ? "index.stop_recording" : "index.cancel_down") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" v-if="canCopy" @click="action('copy')">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <LinkOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" v-if="canOpen" @click="action('open')">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <GlobeOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.open_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" @click="action('json')">
          <n-icon
              size="28"
              class="resource-action-icon rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <CopyOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_data") }}</span>
        </div>

        <div
            v-for="item in pluginActions"
            :key="item.id"
            class="flex items-center justify-start p-1.5 cursor-pointer"
            :title="item.description"
            @click="action('plugin-action:' + item.id)"
        >
          <n-icon
              size="28"
              class="resource-action-icon resource-action-icon--primary rounded-full flex items-center justify-center p-1.5 cursor-pointer"
          >
            <LockOpenOutline/>
          </n-icon>
          <span class="ml-1">{{ item.label }}</span>
        </div>
      </div>
    </NPopover>
  </div>
</template>

<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import {computed} from 'vue'
import {CloseOutline, CopyOutline, DownloadOutline, GlobeOutline, GridSharp, LinkOutline, LockOpenOutline, TrashOutline} from "@vicons/ionicons5"
import type {appType} from '@/types/app'

const {t} = useI18n()
const props = defineProps<{
  row: any,
  index: number,
  pluginActions?: appType.DisplayResourceAction[],
}>()

const emits = defineEmits(["action"])

const capabilities = () => Array.isArray(props.row.capabilities) ? props.row.capabilities : []
const canDownload = computed(() => capabilities().includes('download') && props.row.state !== 'partial')
const canOpen = computed(() => capabilities().includes('open'))
const canCopy = computed(() => capabilities().includes('copy'))
const isLive = computed(() => Array.isArray(props.row.traits) && props.row.traits.includes('live'))
const isDownloading = computed(() => ['pending', 'resolving', 'downloading', 'processing', 'pausing'].includes(props.row.download?.state))

const action = (type: string) => {
  if (type === 'down' && !canDownload.value) {
    window?.$message?.error(t("index.download_no_tip"))
    return
  }
  emits('action', props.row, props.index, type)
}

</script>
