<template>
  <NConfigProvider class="h-full" :theme="theme" :locale="zhCN">
    <NaiveProvider>
      <RouterView />
    </NaiveProvider>
    <NGlobalStyle />
    <NModalProvider />
  </NConfigProvider>
</template>

<script setup lang="ts">
import NaiveProvider from '@/components/NaiveProvider.vue'
import {darkTheme, lightTheme, zhCN} from 'naive-ui'
import {useIndexStore} from "@/stores"
import {computed, onMounted} from "vue"
import {useEventStore} from "@/stores/event"
import {appType} from "@/types/app";

const store = useIndexStore()
const eventStore = useEventStore()

const theme = computed(() => {
  if (store.globalConfig.Theme === "darkTheme") {
    document.documentElement.classList.add('dark');
    return darkTheme
  }
  document.documentElement.classList.remove('dark');
  return lightTheme
})

onMounted(async () => {
  await store.init()
  eventStore.init()
  eventStore.addHandle({
    type: "message",
    event: (res: appType.Message)=>{
      switch (res?.code) {
        case 0:
          window?.$message?.error(res.message)
          break
        case 1:
          window?.$message?.success(res.message)
          break
      }
    }
  })

  eventStore.addHandle({
    type: "updateProxyStatus",
    event: (res: any)=>{
      store.updateProxyStatus(res)
    }
  })
})
</script>

<style scoped>
</style>
