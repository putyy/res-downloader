<template>
  <div class="w-[760px] space-y-4">
    <NAlert type="info" :show-icon="false">{{ t('setting.media_engine_tip') }}</NAlert>
    <NForm label-placement="left" label-width="120">
      <NFormItem label="FFmpeg">
        <NInput :value="config.FFmpegPath" :placeholder="t('setting.media_auto_detect')"
                @update:value="(value: string) => emit('update:ffmpeg', value)"/>
      </NFormItem>
      <NFormItem label="ffprobe">
        <NInput :value="config.FFprobePath" :placeholder="t('setting.media_auto_detect')"
                @update:value="(value: string) => emit('update:ffprobe', value)"/>
      </NFormItem>
      <NFormItem>
        <NButton type="primary" secondary :loading="checking" @click="detect">{{ t('setting.media_detect') }}</NButton>
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

defineProps<{ config: appType.Config }>()
const emit = defineEmits<{
  (event: 'update:ffmpeg', value: string): void
  (event: 'update:ffprobe', value: string): void
}>()
const {t} = useI18n()
const checking = ref(false)
const status = ref<appType.MediaEngineStatus>()

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
