<template>
  <div class="w-[760px] space-y-4">
    <NAlert type="info" :show-icon="false">{{ t('setting.media_engine_tip') }}</NAlert>
    <NForm label-placement="left" label-width="60">
      <NFormItem label="FFmpeg">
        <div class="flex w-full items-center gap-2">
          <NInput class="min-w-0 flex-1" :value="config.FFmpegPath" :placeholder="t('setting.media_auto_detect')"
                  @update:value="(value: string) => emit('update:ffmpeg', value)"/>
          <NButton class="w-32" secondary :loading="selectingTool === 'ffmpeg'" @click="selectTool('ffmpeg')">
            {{ t('setting.media_select_file') }}
          </NButton>
        </div>
      </NFormItem>
      <NFormItem label="ffprobe">
        <div class="flex w-full items-center gap-2">
          <NInput class="min-w-0 flex-1" :value="config.FFprobePath" :placeholder="t('setting.media_auto_detect')"
                  @update:value="(value: string) => emit('update:ffprobe', value)"/>
          <NButton class="w-32" secondary :loading="selectingTool === 'ffprobe'" @click="selectTool('ffprobe')">
            {{ t('setting.media_select_file') }}
          </NButton>
        </div>
      </NFormItem>
      <NFormItem>
        <div class="flex items-center gap-3">
          <NButton type="primary" secondary :disabled="checking" @click="detect">
            {{ t('setting.media_detect') }}
          </NButton>
          <NButton text type="primary" @click="BrowserOpenURL(ffmpegWebsite)">
            {{ t('setting.media_install_website') }}
          </NButton>
        </div>
      </NFormItem>
    </NForm>
    <div v-if="status" class="grid grid-cols-2 gap-3">
      <NCard size="small" title="FFmpeg">
        <NTag :type="status.ffmpeg.available ? 'success' : 'error'">
          {{ status.ffmpeg.available ? t('setting.media_available') : t('setting.media_unavailable') }}
        </NTag>
        <div class="mt-2 break-all text-xs text-gray-500">{{ status.ffmpeg.version || status.ffmpeg.error }}</div>
        <div v-if="status.ffmpeg.path" class="mt-1 break-all text-xs text-gray-400">{{ status.ffmpeg.path }}</div>
      </NCard>
      <NCard size="small" title="ffprobe">
        <NTag :type="status.ffprobe.available ? 'success' : 'error'">
          {{ status.ffprobe.available ? t('setting.media_available') : t('setting.media_unavailable') }}
        </NTag>
        <div class="mt-2 break-all text-xs text-gray-500">{{ status.ffprobe.version || status.ffprobe.error }}</div>
        <div v-if="status.ffprobe.path" class="mt-1 break-all text-xs text-gray-400">{{ status.ffprobe.path }}</div>
      </NCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import appApi from '@/api/app'
import type {appType} from '@/types/app'
import {BrowserOpenURL} from '../../../wailsjs/runtime'

defineProps<{ config: appType.Config }>()
const emit = defineEmits<{
  (event: 'update:ffmpeg', value: string): void
  (event: 'update:ffprobe', value: string): void
}>()
const {t} = useI18n()
const checking = ref(false)
const status = ref<appType.MediaEngineStatus>()
const ffmpegWebsite = 'https://ffmpeg.org/download.html'
type MediaTool = 'ffmpeg' | 'ffprobe'
const selectingTool = ref<MediaTool | null>(null)

const selectTool = async (tool: MediaTool) => {
  if (selectingTool.value) return
  selectingTool.value = tool
  try {
    const response = await appApi.openFileDialog({purpose: tool}) as appType.Res<{ file: string }>
    if (response.code !== 1) {
      window?.$message?.error(response.message)
      return
    }
    const file = response.data?.file
    if (!file) return
    if (tool === 'ffmpeg') emit('update:ffmpeg', file)
    else emit('update:ffprobe', file)
    status.value = undefined
  } catch (error) {
    window?.$message?.error(String(error))
  } finally {
    selectingTool.value = null
  }
}

const detect = async () => {
  checking.value = true
  try {
    const response = await appApi.mediaStatus() as appType.Res<appType.MediaEngineStatus>
    if (response.code === 1) status.value = response.data
  } finally {
    checking.value = false
  }
}

onMounted(detect)
</script>
