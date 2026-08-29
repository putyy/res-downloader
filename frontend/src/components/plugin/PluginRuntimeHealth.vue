<template>
  <NAlert v-if="visible" class="mt-3" :type="paused ? 'warning' : 'info'" :show-icon="false">
    {{ t('plugin.runtime_health', {errors: health.totalErrors || 0, slow: health.slowCalls || 0}) }}
    <span v-if="paused"> · {{ t('plugin.runtime_paused') }}</span>
    <div v-if="health.lastError" class="ellipsis-2 mt-1 break-all text-xs" :title="health.lastError">
      {{ health.lastError }}
    </div>
  </NAlert>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import type {appType} from '@/types/app'

const props = defineProps<{ health?: appType.PluginRuntimeHealth }>()
const {t} = useI18n()
const health = computed(() => props.health ?? {})
const paused = computed(() => (health.value.pausedUntil || 0) > Date.now())
const visible = computed(() => !!health.value.totalErrors || !!health.value.slowCalls || paused.value)
</script>
