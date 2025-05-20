<template>
  <NSpace style="--wails-draggable:no-drag">
    <NButton v-if="row.Classify != 'live' && row.Classify != 'm3u8'" type="success" :tertiary="true" size="small" @click="action('down')">
      {{ t("index.direct_download") }}
    </NButton>
    <NButton type="info" :tertiary="true" size="small" @click="action('copy')">
      {{ t("index.copy_link") }}
    </NButton>
    <NButton v-if="row.Classify != 'live' && row.Classify != 'm3u8'" type="info" :tertiary="true" size="small" @click="action('open')">
      {{ t("index.open_link") }}
    </NButton>
    <NButton v-if="row.DecodeKey" type="warning" :tertiary="true" size="small" @click="action('decode')">
      {{ t("index.video_decode") }}
    </NButton>
    <NButton type="info" :tertiary="true" size="small" @click="action('json')">
      {{ t("index.copy_data") }}
    </NButton>
    <NButton type="error" :tertiary="true" size="small" @click="action('delete')">
      {{ t("common.delete") }}
    </NButton>
  </NSpace>
</template>

<script setup lang="ts">
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const props = defineProps<{
  row: any,
  index: number,
}>()

const emits = defineEmits(["action"])

const action = (type: string) => {
  emits('action', props.row, props.index, type)
}

</script>