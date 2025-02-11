<template>
  <NSpace style="--wails-draggable:no-drag">
    <NButton v-if="row.Classify != 'live' && row.Classify != 'm3u8'" type="success" :tertiary="true" size="small" @click="action('down')">
      直接下载
    </NButton>
    <NButton type="info" :tertiary="true" size="small" @click="action('copy')">
      复制链接
    </NButton>
    <NButton v-if="row.Classify != 'live' && row.Classify != 'm3u8'" type="info" :tertiary="true" size="small" @click="action('open')">
      打开浏览
    </NButton>
    <NButton v-if="row.DecodeKey" type="warning" :tertiary="true" size="small" @click="action('decode')">
      视频解密
    </NButton>
    <NButton v-if="isDebug" type="info" :tertiary="true" size="small" @click="action('json')">
      复制数据
    </NButton>
    <NButton type="error" :tertiary="true" size="small" @click="action('delete')">
      删除
    </NButton>
  </NSpace>
</template>

<script setup lang="ts">
import {inject} from "vue"

const props = defineProps<{
  row: any,
  index: number,
}>()

const emits = defineEmits(["action"])

const isDebug = inject('isDebug')

const action = (type: string) => {
  emits('action', props.row, props.index, type)
}

</script>