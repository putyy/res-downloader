<template>
  <div
      class="min-h-6"
      @click="handleOnClick"
  >
    <n-input
        v-if="isEdit"
        ref="inputRef"
        :value="inputValue"
        @update:value="v => inputValue = v"
        @change="handleChange"
        @blur="handleChange"
    />

    <n-tooltip
        v-else
        trigger="hover"
        placement="top"
    >
      <template #trigger>
        <div class="ellipsis-2">{{ inputValue }}</div>
      </template>
      <div class="ellipsis-2">{{ inputValue }}</div>
    </n-tooltip>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import type { InputInst } from 'naive-ui'

interface OnUpdateValue {
  (value: string): void
}

const props = defineProps<{
  value: string | number
  onUpdateValue?: OnUpdateValue
}>()

const isEdit = ref(false)
const inputRef = ref<InputInst | null>(null)
const inputValue = ref(String(props.value))

watch(
    () => props.value,
    v => inputValue.value = String(v)
)

function handleOnClick() {
  isEdit.value = true
  nextTick(() => inputRef.value?.focus())
}

function handleChange() {
  props.onUpdateValue?.(String(inputValue.value))
  isEdit.value = false
}
</script>
