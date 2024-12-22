<template>
  <div class="fixed inset-0 flex items-center justify-center bg-black bg-opacity-50 z-50" v-if="isLoading">
    <div class="flex flex-col items-center">
      <svg class="animate-spin h-10 w-10 text-white mb-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
      </svg>
      <span class="text-white">{{ loadingText }}</span>
    </div>
  </div>
</template>

<script lang="ts" setup>
const props = defineProps(["isLoading", "loadingText"])
</script>