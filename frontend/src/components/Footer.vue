<template>
  <NModal
      :show="showModal"
      :on-update:show="changeShow"
      style="--wails-draggable:no-drag"
      preset="card"
      class="w-[640px]"
      title="关于"
  >
    <div class="rounded p-5">
      <div class="flex flex-col">
        <div class="flex flex-row">
          <div>
            <div class="flex flex-row items-center">
              <div class="text-4xl font-bold">{{ store.appInfo.AppName }}</div>
              <div class="text-xs pl-5 text-slate-400">
                Version {{ store.appInfo.Version }}
              </div>
            </div>
            <div class="text-slate-400 w-80 pt-2 pb-4">
              {{ store.appInfo.Description }}
            </div>
          </div>
          <div class="pl-8">
            <img src="@/assets/image/logo.png" alt="Logo" class="h-28 w-28"/>
          </div>
        </div>
      </div>
      <div class="flex flex-col">
        <div class="text-2xl font-bold text-emerald-600">
          支持市面上几乎所有的网络应用
        </div>
        <div class="grid grid-cols-5 gap-2 text-sm m-4 text-slate-400">
          <span>抖音</span>
          <span>快手</span>
          <span>小红书</span>
          <span>视频号</span>
          <span>小程序</span>
          <span>公众号</span>
          <span>酷狗音乐</span>
          <span>QQ音乐</span>
          <span>QQ微视</span>
          <span>......</span>
        </div>
      </div>
      <div class="flex w-full text-sm justify-between pt-8 text-slate-400">
        <div>{{ store.appInfo.Copyright }}</div>
        <div class="flex">
          <button class="pl-4" @click="toWebsite('https://s.gowas.cn/d/4089')">论坛</button>
          <button class="pl-4" @click="toWebsite('https://github.com/putyy/res-downloader')">软件源码</button>
          <button class="pl-4" @click="toWebsite('https://github.com/putyy/res-downloader/issues')">帮助支持</button>
          <button class="pl-4" @click="toWebsite('https://github.com/putyy/res-downloader/releases')">更新日志</button>
        </div>
      </div>
    </div>
  </NModal>
</template>

<script lang="ts" setup>
import {useIndexStore} from "@/stores"
import {BrowserOpenURL} from "../../wailsjs/runtime";
const store = useIndexStore()
const props = defineProps(["showModal"])
const emits = defineEmits(["update:showModal"])

const changeShow = (value: boolean) => {
  emits('update:showModal', value)
}

const toWebsite = (url: string) => {
  BrowserOpenURL(url)
}
</script>