<template>
  <NTooltip trigger="hover">
    <template #trigger>
      <span class="inline-flex items-center gap-1">
        <span v-if="color" class="inline-block h-2 w-2 rounded-full" :style="{backgroundColor: color}"/>
        {{ label }}
      </span>
    </template>
    <div>{{ kind || primaryType }}</div>
    <div v-if="traits.length" class="mt-1 max-w-[280px] text-xs">{{ traits.join(' · ') }}</div>
  </NTooltip>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  label: string
  primaryType?: string
  kind?: string
  traits?: string[]
  color?: string
}>(), {primaryType: '', kind: '', traits: () => [], color: ''})
</script>
