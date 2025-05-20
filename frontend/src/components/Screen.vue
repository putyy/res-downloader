<template>
  <div class="flex justify-around px-2 pt-3">
    <button
        class="w-4 h-4 rounded-full bg-[#ff5c57] flex items-center justify-center transition-all duration-200 hover:bg-[#e0443e] group"
        @click="closeWindow"
        :title="t('components.screen_close')"
    >
      <svg class="w-[0.7rem] h-[0.7rem] text-[#2c2c2c] opacity-0 group-hover:opacity-100 transition-opacity duration-150" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M6 6L18 18M6 18L18 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
    </button>
    <button
        class="w-4 h-4 rounded-full bg-[#ffbc38] flex items-center justify-center transition-all duration-200 hover:bg-[#e0a824] group"
        @click="minimizeWindow"
        :title="t('components.screen_minimize')"
    >
      <svg class="w-[0.7rem] h-[0.7rem] text-[#2c2c2c] opacity-0 group-hover:opacity-100 transition-opacity duration-150" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M4 12H20" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
    </button>
    <button
        class="w-4 h-4 rounded-full bg-[#28c840] flex items-center justify-center transition-all duration-200 hover:bg-[#1ea230] group"
        @click="maximizeWindow"
        :title="isMaximized ? t('components.screen_restore') : t('components.screen_maximize')"
    >
      <svg v-if="isMaximized" class="w-[0.5rem] h-[0.5rem] text-[#2c2c2c] opacity-0 group-hover:opacity-100 transition-opacity duration-150" t="1729223337944" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="2168">
        <path d="M965.6 512.8L564 512c-28.8 0-52 23.2-52 51.2l1.6 401.6c0 28.8 16.8 35.2 36.8 15.2l430.4-431.2c20-20 12.8-36-15.2-36zM510.4 58.4c0-28.8-16.8-35.2-36.8-15.2L44 474.4c-20 20-12.8 36.8 15.2 36.8l401.6 0.8c28.8 0 51.2-23.2 51.2-51.2l-1.6-402.4z" fill="#2c2c2c" p-id="2169"/>
      </svg>
      <svg v-else class="w-[0.5rem] h-[0.5rem] text-[#2c2c2c] opacity-0 group-hover:opacity-100 transition-opacity duration-150" t="1729223289826" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1881">
        <path d="M966.2236448 410.53297813l0 496.32142187a56.61582187 56.61582187 0 0 1-56.59306667 56.61582187l-496.32142293-1e-8a56.59306667 56.59306667 0 0 1-39.9815104-96.68835519L869.53528854 370.57422187a56.61582187 56.61582187 0 0 1 96.71111146 39.95875626z m-905.67111147 200.15786666L60.55253333 114.39217812a56.61582187 56.61582187 0 0 1 56.59306667-56.61582292l496.29866667 0a56.59306667 56.59306667 0 0 1 39.98151146 96.68835626l-496.2076448 496.20764374a56.61582187 56.61582187 0 0 1-96.68835519-39.9815104z" fill="#373C43" p-id="1882"/>
      </svg>
    </button>
  </div>
</template>

<script lang="ts" setup>
import {ref} from "vue"
import {Quit, WindowFullscreen, WindowMinimise, WindowUnfullscreen} from "../../wailsjs/runtime"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const isMaximized = ref(false)

const closeWindow = () => {
  Quit()
}
const minimizeWindow = () => {
  WindowMinimise()
}
const maximizeWindow = () => {
  isMaximized.value = !isMaximized.value;
  if (isMaximized.value) {
    WindowFullscreen()
  } else {
    WindowUnfullscreen()
  }
}
</script>

<style scoped>
</style>
