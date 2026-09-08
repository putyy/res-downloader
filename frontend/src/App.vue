<template>
  <NConfigProvider
      class="h-full"
      :theme="activeTheme.naiveTheme"
      :theme-overrides="activeTheme.overrides"
      :locale="uiLocale"
  >
    <NaiveProvider>
      <StartupScreen v-if="store.startupState !== 'ready'"/>
      <template v-else>
        <RouterView/>
        <CertificateSetupGuide/>
      </template>
    </NaiveProvider>
    <NGlobalStyle/>
    <NModalProvider/>
  </NConfigProvider>
</template>

<script setup lang="ts">
import NaiveProvider from '@/components/NaiveProvider.vue'
import {enUS, zhCN} from 'naive-ui'
import {useIndexStore} from "@/stores"
import {computed, watch} from "vue"
import {useEventStore} from "@/stores/event"
import type {appType} from "@/types/app"
import {useI18n} from 'vue-i18n'
import {resolveAppTheme} from '@/themes'
import CertificateSetupGuide from '@/components/settings/CertificateSetupGuide.vue'
import StartupScreen from '@/components/StartupScreen.vue'

const store = useIndexStore()
const eventStore = useEventStore()
const {locale} = useI18n()

const activeTheme = computed(() => resolveAppTheme(store.globalConfig.Theme))

watch(activeTheme, (theme) => {
  document.documentElement.classList.toggle('dark', theme.dark)
  document.documentElement.dataset.appTheme = theme.id
  document.documentElement.dataset.menuInset = String(theme.menuInset)
  Object.entries(theme.cssVars).forEach(([name, value]) => {
    document.documentElement.style.setProperty(name, value)
  })
}, {immediate: true})

watch(() => store.globalConfig.Locale, value => {
  locale.value = value
  document.documentElement.lang = value === 'zh' ? 'zh-CN' : 'en'
}, {immediate: true})

const uiLocale = computed(() => store.globalConfig.Locale === 'zh' ? zhCN : enUS)

let eventsInitialized = false
const initializeEvents = () => {
  if (eventsInitialized) return
  eventsInitialized = true
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
}

watch(() => store.startupState, state => {
  if (state === 'ready') initializeEvents()
}, {immediate: true})
</script>
