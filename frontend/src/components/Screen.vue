<template>
  <div class="flex justify-around px-2 pt-3">
    <button class="w-3 h-3 rounded-full flex items-center justify-center p-[2px] bg-[#fd5b5b]" @click="closeWindow">
      <svg t="1729232659670" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="11784" width="200" height="200">
        <path d="M617.92 516.096l272 272-101.824 101.824-272-272-272 272-101.856-101.824 272-272-275.008-275.04L241.056 139.2l275.04 275.04 275.04-275.04 101.824 101.824-275.04 275.04z" fill="#2c2c2c" p-id="11785"/>
      </svg>
    </button>
    <button class="w-3 h-3 rounded-full bg-yellow-500 flex items-center justify-center p-[2px]" @click="minimizeWindow">
      <svg t="1729223652686" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="9954" width="200" height="200">
        <path d="M892 472c22.1 0 40 17.9 40 40 0 21.9-17.6 39.6-39.3 40H132c-22.1 0-40-17.9-40-40 0-21.9 17.6-39.6 39.3-40H892z" fill="#111111" p-id="9955"/>
        <path d="M892 472c22.1 0 40 17.9 40 40 0 21.9-17.6 39.6-39.3 40H132c-22.1 0-40-17.9-40-40 0-21.9 17.6-39.6 39.3-40H892z" fill="#111111" p-id="9956"/>
      </svg>
    </button>
    <button class="w-3 h-3 rounded-full bg-green-500 flex items-center justify-center" :class="isMaximized ? 'p-[2px]' : 'p-[3px]'" @click="maximizeWindow">
      <svg v-if="isMaximized" t="1729223337944" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="2168" width="200" height="200">
        <path d="M965.6 512.8L564 512c-28.8 0-52 23.2-52 51.2l1.6 401.6c0 28.8 16.8 35.2 36.8 15.2l430.4-431.2c20-20 12.8-36-15.2-36zM510.4 58.4c0-28.8-16.8-35.2-36.8-15.2L44 474.4c-20 20-12.8 36.8 15.2 36.8l401.6 0.8c28.8 0 51.2-23.2 51.2-51.2l-1.6-402.4z" fill="#2c2c2c" p-id="2169"/>
      </svg>
      <svg v-else t="1729223289826" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="1881" width="200" height="200">
        <path d="M966.2236448 410.53297813l0 496.32142187a56.61582187 56.61582187 0 0 1-56.59306667 56.61582187l-496.32142293-1e-8a56.59306667 56.59306667 0 0 1-39.9815104-96.68835519L869.53528854 370.57422187a56.61582187 56.61582187 0 0 1 96.71111146 39.95875626z m-905.67111147 200.15786666L60.55253333 114.39217812a56.61582187 56.61582187 0 0 1 56.59306667-56.61582292l496.29866667 0a56.59306667 56.59306667 0 0 1 39.98151146 96.68835626l-496.2076448 496.20764374a56.61582187 56.61582187 0 0 1-96.68835519-39.9815104z" fill="#373C43" p-id="1882"/>
      </svg>
    </button>
  </div>
</template>

<script lang="ts" setup>
import {ref} from "vue"
import {Quit, WindowFullscreen, WindowMinimise, WindowUnfullscreen} from "../../wailsjs/runtime"

const isMaximized = ref(false);

const closeWindow = () => {
  Quit()
}
const minimizeWindow = () => {
  WindowMinimise()
}
const maximizeWindow = () => {
  console.log("maximizeWindow")
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
