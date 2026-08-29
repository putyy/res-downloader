<template>
  <NConfigProvider
      class="h-full"
      :theme="activeTheme.naiveTheme"
      :theme-overrides="activeTheme.overrides"
      :locale="uiLocale"
  >
    <NaiveProvider>
      <RouterView/>
      <CertificateSetupGuide/>
    </NaiveProvider>
    <NGlobalStyle/>
    <NModalProvider/>
  </NConfigProvider>
</template>

<script setup lang="ts">
import NaiveProvider from '@/components/NaiveProvider.vue'
import {enUS, zhCN} from 'naive-ui'
import {useIndexStore} from "@/stores"
import {computed, onMounted, watch} from "vue"
import {useEventStore} from "@/stores/event"
import type {appType} from "@/types/app"
import {useI18n} from 'vue-i18n'
import {resolveAppTheme} from '@/themes'
import CertificateSetupGuide from '@/components/settings/CertificateSetupGuide.vue'

const store = useIndexStore()
const eventStore = useEventStore()
const {locale} = useI18n()

const activeTheme = computed(() => resolveAppTheme(store.globalConfig.Theme))

watch(activeTheme, (theme) => {
  document.documentElement.classList.toggle('dark', theme.dark)
  document.documentElement.dataset.appTheme = theme.id
}, {immediate: true})

const uiLocale = computed(() => {
  locale.value = store.globalConfig.Locale
  if (store.globalConfig.Locale === "zh") {
    return zhCN
  }
  return enUS
})

onMounted(() => {
  eventStore.init()
  eventStore.addHandle({
    type: "message",
    event: (res: appType.Message) => {
      switch (res?.code) {
        case 0:
          window.$message?.error(res.message)
          break
        case 1:
          window.$message?.success(res.message)
          break
      }
    }
  })
})
</script>
