<template>
  <div style="--wails-draggable:no-drag" class="grid grid-cols-3 gap-1.5">
    <n-icon
        size="30"
        class="text-emerald-600 dark:text-emerald-400 bg-emerald-500/20 dark:bg-emerald-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-emerald-500/40 transition-colors"
        @click="action('down')"
    >
      <DownloadOutline/>
    </n-icon>

    <n-icon
        size="28"
        class="text-red-500 dark:text-red-300 bg-red-500/20 dark:bg-red-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-red-500/40 transition-colors"
        @click="action('delete')"
    >
      <TrashOutline/>
    </n-icon>

    <NPopover placement="bottom" trigger="hover">
      <template #trigger>
        <NIcon size="30" class="text-sky-500 dark:text-sky-300 bg-sky-500/20 dark:bg-sky-200/30 rounded-full flex items-center justify-center p-2 cursor-pointer hover:bg-sky-200/40 transition-colors">
          <GridSharp/>
        </NIcon>
      </template>
      <div class="flex flex-col">
        <div class="flex items-center justify-start p-1.5 cursor-pointer" v-if="row.Status === 'running' || row.Status === 'pending'" @click="action('cancel')">
          <n-icon
              size="28"
              class="text-red-500 dark:text-red-300 bg-red-500/20 dark:bg-red-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-red-500/40 transition-colors"
          >
            <CloseOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.cancel_down") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" @click="action('copy')">
          <n-icon
              size="28"
              class="text-blue-300 dark:text-blue-300 bg-blue-300/20 dark:bg-blue-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-blue-300/40 transition-colors"
          >
            <LinkOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" v-if="row.Classify !== 'live' && row.Classify !== 'm3u8'" @click="action('open')">
          <n-icon
              size="28"
              class="text-blue-500 dark:text-blue-200 bg-blue-400/20 dark:bg-blue-400/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-blue-400/40 transition-colors"
          >
            <GlobeOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.open_link") }}</span>
        </div>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" v-if="row.DecodeKey" @click="action('decode')">
          <n-icon
              size="28"
              class="text-orange-400 dark:text-red-300 bg-orange-500/20 dark:bg-orange-200/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-orange-200/40 transition-colors"
          >
            <LockOpenSharp/>
          </n-icon>
          <span class="ml-1">{{ t("index.video_decode") }}</span>
        </div>

        <button
            v-if="showCommentAction"
            type="button"
            class="flex w-full min-h-[40px] items-center justify-start rounded p-1.5 text-left transition-colors hover:bg-purple-500/10 focus-visible:outline focus-visible:outline-2 focus-visible:outline-purple-500 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="commentActive"
            :aria-busy="commentActive"
            :aria-label="commentLabel"
            @click="action('comments')"
        >
          <n-spin v-if="commentActive" :size="24" class="mx-0.5"/>
          <n-icon
              v-else
              size="28"
              class="flex items-center justify-center rounded-full bg-purple-500/20 p-1.5 text-purple-500 transition-colors hover:bg-purple-500/40 dark:bg-purple-500/30 dark:text-purple-300"
          >
            <ChatboxOutline/>
          </n-icon>
          <span class="ml-1.5">{{ commentLabel }}</span>
        </button>

        <div class="flex items-center justify-start p-1.5 cursor-pointer" @click="action('json')">
          <n-icon
              size="28"
              class="text-sky-400 dark:text-sky-200 bg-sky-500/20 dark:bg-sky-500/30 rounded-full flex items-center justify-center p-1.5 cursor-pointer hover:bg-sky-500/40 transition-colors"
          >
            <CopyOutline/>
          </n-icon>
          <span class="ml-1">{{ t("index.copy_data") }}</span>
        </div>
      </div>
    </NPopover>
  </div>
</template>

<script setup lang="ts">
import {computed} from "vue"
import {useI18n} from 'vue-i18n'
import {isWechatChannelResource} from '@/func'
import {
  DownloadOutline,
  CopyOutline,
  GlobeOutline,
  LockOpenSharp,
  LinkOutline,
  GridSharp,
  CloseOutline,
  TrashOutline,
  ChatboxOutline
} from "@vicons/ionicons5"

const {t} = useI18n()
const props = defineProps<{
  row: any,
  index: number,
}>()

const emits = defineEmits(["action"])

const showCommentAction = computed(() => isWechatChannelResource(props.row))
const commentActive = computed(() => ['queued', 'running'].includes(props.row.CommentMeta?.status))
const commentLabel = computed(() => {
  const status = props.row.CommentMeta?.status
  if (status === 'queued') return t('index.comments_status_queued')
  if (status === 'running') return t('index.comments_status_running')
  if (props.row.CommentMeta?.fetchedAt) {
    return t('index.comments_view_history', {count: props.row.Comments?.length || 0})
  }
  return t('index.get_comments')
})

const action = (type: string) => {
  if (type === 'down' && (props.row.Classify === 'live' || props.row.Classify === 'm3u8')) {
    window?.$message?.error(t("index.download_no_tip"))
    return
  }
  emits('action', props.row, props.index, type)
}

</script>
