<template>
  <NModal
      :show="showModal"
      :on-update:show="changeShow"
      preset="card"
      class="comments-dialog w-[720px] max-w-[calc(100vw-24px)]"
      style="--wails-draggable:no-drag"
      :auto-focus="true"
      :trap-focus="true"
      :close-on-esc="true"
  >
    <template #header>
      <div class="min-w-0 pr-6">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-base font-semibold">{{ t('index.comments_title') }}</span>
          <NTag v-if="meta?.targetVerified" type="success" size="small" round>
            <template #icon><CheckmarkCircleOutline/></template>
            {{ t('index.comments_target_verified') }}
          </NTag>
          <NTag v-if="meta?.status === 'partial_success'" type="warning" size="small" round>
            {{ t('index.comments_partial_badge') }}
          </NTag>
        </div>
        <p v-if="resource?.Description" class="mt-1 max-w-[580px] truncate text-xs font-normal text-gray-500 dark:text-gray-400">
          {{ resource.Description }}
        </p>
      </div>
    </template>

    <section v-if="resource" aria-live="polite" aria-atomic="true">
      <div class="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('index.comments_count', {count: comments.length}) }}</span>
        <span v-if="meta?.fetchedAt">{{ t('index.comments_fetched_at', {time: formatTimestamp(meta.fetchedAt)}) }}</span>
      </div>

      <NAlert v-if="meta?.status === 'partial_success'" type="warning" :show-icon="true" class="mb-3">
        {{ t('index.comments_partial_tip') }}
      </NAlert>

      <div v-if="comments.length === 0" class="flex min-h-[220px] flex-col items-center justify-center px-6 text-center">
        <ChatbubbleEllipsesOutline class="mb-3 h-10 w-10 text-purple-400" aria-hidden="true"/>
        <p class="font-medium">{{ t('index.comments_no_comments_title') }}</p>
        <p class="mt-1 max-w-md text-sm text-gray-500 dark:text-gray-400">{{ t('index.comments_no_comments_tip') }}</p>
      </div>

      <NScrollbar v-else class="comments-scroll" style="max-height: min(56vh, 520px)">
        <ol class="divide-y divide-gray-200 pr-3 dark:divide-gray-700" :aria-label="t('index.comments_title')">
          <li v-for="(comment, index) in comments" :key="`${comment.createdAt}-${index}`" class="py-3 first:pt-0">
            <article>
              <header class="flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <span class="font-semibold">{{ comment.nickName || t('index.comments_anonymous') }}</span>
                  <span v-if="comment.region" class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('index.comments_region', {region: comment.region}) }}
                  </span>
                </div>
                <div class="flex shrink-0 gap-3 text-xs text-gray-500 dark:text-gray-400">
                  <span v-if="comment.likeCount">{{ t('index.comments_likes', {count: comment.likeCount}) }}</span>
                  <span v-if="comment.replyCount">{{ t('index.comments_replies', {count: comment.replyCount}) }}</span>
                </div>
              </header>
              <p class="mt-1.5 whitespace-pre-wrap break-words leading-6">{{ comment.content }}</p>
              <time v-if="comment.createdAt" class="mt-1.5 block text-xs text-gray-400" :datetime="toIso(comment.createdAt)">
                {{ formatCommentTime(comment.createdAt) }}
              </time>
            </article>
          </li>
        </ol>
      </NScrollbar>
    </section>

    <template #footer>
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
          <ShieldCheckmarkOutline class="h-4 w-4 shrink-0" aria-hidden="true"/>
          {{ t('index.comments_local_only') }}
        </p>
        <div class="flex justify-end gap-2">
          <NButton size="large" @click="retry">
            <template #icon><RefreshOutline/></template>
            {{ t('index.comments_retry') }}
          </NButton>
          <NButton type="primary" size="large" @click="changeShow(false)">
            {{ t('common.close') }}
          </NButton>
        </div>
      </div>
    </template>
  </NModal>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import type {appType} from '@/types/app'
import {
  ChatbubbleEllipsesOutline,
  CheckmarkCircleOutline,
  RefreshOutline,
  ShieldCheckmarkOutline
} from '@vicons/ionicons5'

const props = defineProps<{
  showModal: boolean
  resource?: appType.MediaInfo | null
}>()

const emits = defineEmits(['update:showModal', 'retry'])
const {t, locale} = useI18n()
const comments = computed(() => props.resource?.Comments || [])
const meta = computed(() => props.resource?.CommentMeta)
const localeName = computed(() => locale.value === 'zh' ? 'zh-CN' : 'en-US')

const changeShow = (value: boolean) => emits('update:showModal', value)
const retry = () => {
  emits('retry')
  changeShow(false)
}

const formatTimestamp = (value: number) => new Intl.DateTimeFormat(localeName.value, {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
}).format(new Date(value))

const formatCommentTime = (value: number) => formatTimestamp(value * 1000)
const toIso = (value: number) => new Date(value * 1000).toISOString()
</script>

<style scoped>
@media (max-width: 640px) {
  .comments-dialog :deep(.n-card__content) {
    padding-left: 16px;
    padding-right: 16px;
  }

  .comments-scroll {
    max-height: 52vh !important;
  }
}

@media (prefers-reduced-motion: reduce) {
  .comments-dialog *,
  .comments-dialog *::before,
  .comments-dialog *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
