<template>
  <NModal
      :show="showModal"
      :on-update:show="changeShow"
      style="--wails-draggable:no-drag"
      preset="card"
      class="w-[640px]"
      :title="t('footer.title')"
  >
    <div class="rounded p-5">
      <div class="flex flex-col">
        <div class="flex flex-row">
          <div>
            <div class="flex flex-row items-end">
              <div class="text-4xl font-bold">{{ store.appInfo.AppName }}</div>
              <div class="text-xs pl-5 text-slate-400">
                Version {{ store.appInfo.Version }}
              </div>
            </div>
            <div class="text-slate-400 w-80 pt-2 pb-4">
              {{ t('footer.description') }}
            </div>
          </div>
          <div class="pl-8">
            <img src="@/assets/image/logo.png" alt="Logo" class="h-28 w-28"/>
          </div>
        </div>
      </div>
      <div class="flex flex-col">
        <div class="text-2xl font-bold text-emerald-600">
          {{ t('footer.support') }}
        </div>
        <div class="grid grid-cols-5 gap-2 text-sm m-4 text-slate-400">
          <span v-for="item in t('footer.application').split(',')">{{ item }}</span>
        </div>
      </div>
      <div class="flex w-full text-sm justify-between pt-8 text-slate-400">
        <div>{{ store.appInfo.Copyright }}</div>
        <div class="flex">
          <button class="pl-4" @click="toWebsite('https://s.gowas.cn/d/4089')">{{ t('footer.forum') }}</button>
          <button class="pl-4" @click="toWebsite(certUrl)">{{ t('footer.cert') }}</button>
          <button class="pl-4" @click="toWebsite('https://github.com/putyy/res-downloader')">{{ t('footer.source_code') }}</button>
          <button class="pl-4" @click="toWebsite('https://github.com/putyy/res-downloader/issues')">{{ t('footer.help') }}</button>
          <button class="pl-4" @click="toWebsite('https://github.com/putyy/res-downloader/releases')">{{ t('footer.update_log') }}</button>
        </div>
      </div>
    </div>
  </NModal>
</template>

<script lang="ts" setup>
import {useIndexStore} from "@/stores"
import {BrowserOpenURL} from "../../wailsjs/runtime"
import {computed} from "vue"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const store = useIndexStore()
const props = defineProps(["showModal"])
const emits = defineEmits(["update:showModal"])
const certUrl = computed(()=>{
  return store.baseUrl + "/api/cert"
})
const changeShow = (value: boolean) => {
  emits('update:showModal', value)
}

const toWebsite = (url: string) => {
  BrowserOpenURL(url)
}
</script>