<template>
  <NTooltip>
    <template #trigger>
      <div class="px-1 py-1 min-w-0 cursor-default" role="status">
        <div class="text-xs truncate" :class="failed ? 'text-red-500' : 'text-app-muted'">
          {{ t(command.syncUnavailable ? 'index.page_command_sync_label' : `index.page_command_${command.state}`) }}<span v-if="!command.syncUnavailable && command.progress != null"> · {{ Math.floor(command.progress) }}%</span>
        </div>
        <NProgress v-if="active" type="line" :percentage="command.progress ?? 0"
                   :processing="true" :show-indicator="false" :height="3" class="mt-1" />
      </div>
    </template>
    {{ pageCommandMessage(command, t) }}
  </NTooltip>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {NProgress, NTooltip} from 'naive-ui'
import {useI18n} from 'vue-i18n'
import type {appType} from '@/types/app'
import {pageCommandMessage} from '@/services/pageCommands'
const props = defineProps<{command: appType.PageCommandStatus}>()
const {t} = useI18n()
const active = computed(() => !props.command.syncUnavailable && ['pending', 'running'].includes(props.command.state))
const failed = computed(() => props.command.state === 'failed')
</script>
