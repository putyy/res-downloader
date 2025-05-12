<template>
  <NConfigProvider class="h-full" :theme="theme" :locale="uiLocale">
    <NaiveProvider>
      <RouterView/>
      <ShowLoading :isLoading="loading"/>
      <Password v-model:showModal="showPassword" @submit="handlePassword"/>
    </NaiveProvider>
    <NGlobalStyle/>
    <NModalProvider/>
  </NConfigProvider>
</template>

<script setup lang="ts">
import NaiveProvider from '@/components/NaiveProvider.vue'
import {darkTheme, lightTheme, zhCN, enUS} from 'naive-ui'
import {useIndexStore} from "@/stores"
import {computed, onMounted, ref} from "vue"
import {useEventStore} from "@/stores/event"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import ShowLoading from "@/components/ShowLoading.vue"
import Password from "@/components/Password.vue"
import {useI18n} from 'vue-i18n'

const store = useIndexStore()
const eventStore = useEventStore()
const loading = ref(false)
const showPassword = ref(false)
const {t, locale} = useI18n()

const theme = computed(() => {
  if (store.globalConfig.Theme === "darkTheme") {
    document.documentElement.classList.add('dark');
    return darkTheme
  }
  document.documentElement.classList.remove('dark');
  return lightTheme
})

const uiLocale = computed(() => {
  locale.value = store.globalConfig.Locale
  if (store.globalConfig.Locale === "zh") {
    return zhCN
  }
  return enUS
})

onMounted(async () => {
  await store.init()
  loading.value = true
  handleInstall().then((is: boolean)=>{
    loading.value = false
  })


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

const handleInstall = async () => {
  const res = await appApi.install()
  if (res.code === 1) {
    store.globalConfig.AutoProxy && store.openProxy()
    return true
  }

  window.$message?.error(res.message, {duration: 5000})

  if (store.envInfo.platform === 'windows' && res.message.includes('Access is denied')) {
    window.$message?.error('首次启用本软件，请使用鼠标右键选择以管理员身份运行')
  } else if (['darwin', 'linux'].includes(store.envInfo.platform)) {
    showPassword.value = true
  }
  return false
}

const handlePassword = async (password: string, isCache: boolean) => {
  const res = await appApi.setSystemPassword({password, isCache})
  if (res.code === 0) {
    window.$message?.error(res.message)
    return
  }
  handleInstall().then((is: boolean)=>{
    if (is) {
      showPassword.value = false
    }
  })
}
</script>