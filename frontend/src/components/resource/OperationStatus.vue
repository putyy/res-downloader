<template>
  <NTooltip>
    <template #trigger>
      <NButton text size="small" @click="router.push({path: '/operations', query: {executionId: execution.executionId}})">
        <span class="text-xs" :class="execution.state === 'failed' ? 'text-red-500' : 'text-app-muted'">
          {{ executionMessage(execution, t) }}<span v-if="!execution.syncUnavailable && execution.progress != null"> · {{ Math.floor(execution.progress) }}%</span>
        </span>
      </NButton>
    </template>
    {{ t("operations.details") }}
  </NTooltip>
</template>
<script setup lang="ts">
import {NButton, NTooltip} from "naive-ui"
import {useI18n} from "vue-i18n"
import {useRouter} from "vue-router"
import type {OperationExecution} from "@/types/operations"
import {executionMessage} from "@/services/operations"
defineProps<{execution: OperationExecution}>()
const {t} = useI18n()
const router = useRouter()
</script>
