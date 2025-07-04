<template>
  <div style="--wails-draggable:no-drag" class="grid grid-cols-6 gap-1.5">
    <n-icon
        v-if="row.Classify !== 'live' && row.Classify !== 'm3u8'"
        size="28"
        class="text-emerald-600 dark:text-emerald-400 bg-emerald-500/20 dark:bg-emerald-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-emerald-500/40 transition-colors"
        @click="action('down')"
    >
      <DownloadOutline/>
    </n-icon>

    <n-icon
        size="28"
        class="text-blue-600 dark:text-blue-300 bg-blue-500/20 dark:bg-blue-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-blue-500/40 transition-colors"
        @click="action('copy')"
    >
      <LinkOutline/>
    </n-icon>

    <n-icon
        v-if="row.Classify !== 'live' && row.Classify !== 'm3u8'"
        size="28"
        class="text-blue-500 dark:text-blue-200 bg-blue-400/20 dark:bg-blue-400/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-blue-400/40 transition-colors"
        @click="action('open')"
    >
      <GlobeOutline/>
    </n-icon>

    <n-icon
        v-if="row.DecodeKey"
        size="28"
        class="text-orange-400 dark:text-red-300 bg-orange-500/20 dark:bg-orange-200/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-orange-200/40 transition-colors"
        @click="action('decode')"
    >
      <LockOpenSharp/>
    </n-icon>

    <n-icon
        size="28"
        class="text-sky-400 dark:text-sky-200 bg-sky-500/20 dark:bg-sky-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-sky-500/40 transition-colors"
        @click="action('json')"
    >
      <CopyOutline/>
    </n-icon>

    <n-icon
        size="28"
        class="text-red-500 dark:text-red-300 bg-red-500/20 dark:bg-red-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-red-500/40 transition-colors"
        @click="action('delete')"
    >
      <TrashOutline/>
    </n-icon>
  </div>
</template>

<script setup lang="ts">
import {useI18n} from 'vue-i18n'
import {
  DownloadOutline,
  CopyOutline,
  GlobeOutline,
  LockOpenSharp,
  LinkOutline,
  TrashOutline
} from "@vicons/ionicons5"

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