<template>
  <div class="flex pb-2 flex-col h-full min-w-[80px] border-r border-slate-100 dark:border-slate-900">
    <Screen v-if="envInfo.platform!=='darwin'"></Screen>
    <div class="w-full flex flex-row items-center justify-center pt-5" :class="envInfo.platform==='darwin' ? 'pt-8' : 'pt-2'">
      <img class="w-12 h-12 cursor-pointer" src="@/assets/image/logo.png" alt="res-downloader logo" @click="handleFooterUpdate('github')"/>
    </div>
    <main class="flex-1 flex-grow-1 mb-5 overflow-auto flex flex-col pt-1 items-center h-full" v-if="is">
      <NScrollbar :size="1">
        <NLayout has-sider>
          <NLayoutSider
              :bordered="false"
              show-trigger
              collapse-mode="width"
              :on-after-enter="() => { showAppName = true }"
              :on-after-leave="() => { showAppName = false }"
              :collapsed-width="70"
              :collapsed="collapsed"
              :width="140"
              :native-scrollbar="false"
              :inverted="inverted"
              :on-update:collapsed="collapsedChange"
              class="bg-inherit"
          >
            <NMenu
                :inverted="inverted"
                :collapsed-width="70"
                :collapsed-icon-size="22"
                :options="menuOptions"
                :value="menuValue"
                @update:value="handleUpdateValue"
            />
          </NLayoutSider>
        </NLayout>
        <NLayoutFooter position="absolute" :inverted="inverted" class="bg-inherit">
          <NMenu
              :inverted="inverted"
              :collapsed-width="70"
              :collapsed-icon-size="22"
              :options="footerOptions"
              :value="menuValue"
              @update:value="handleFooterUpdate"
          />
        </NLayoutFooter>
      </NScrollbar>
    </main>
  </div>
  <Footer v-model:showModal="showAppInfo"/>
</template>

<script lang="ts" setup>
import {MenuOption} from "naive-ui"
import {NIcon} from "naive-ui"
import {computed, h, onMounted, ref, watch} from "vue"
import {useRoute, useRouter} from "vue-router"
import {
  CloudOutline,
  SettingsOutline,
  HelpCircleOutline,
  MoonOutline,
  SunnyOutline,
  LanguageSharp,
  LogoGithub
} from "@vicons/ionicons5"
import {useIndexStore} from "@/stores"
import Footer from "@/components/Footer.vue"
import Screen from "@/components/Screen.vue"
import {BrowserOpenURL} from "../../../wailsjs/runtime"
import {useI18n} from "vue-i18n"

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const inverted = ref(false)
const collapsed = ref(false)
const showAppName = ref(false)
const showAppInfo = ref(false)
const menuValue = ref(route.fullPath.substring(1))
const store = useIndexStore()
const is = ref(false)

const envInfo = store.envInfo

const globalConfig = computed(() => {
  return store.globalConfig
})

const theme = computed(() => {
  return store.globalConfig.Theme === "darkTheme" ? renderIcon(SunnyOutline) : renderIcon(MoonOutline)
})

watch(() => route.path, (newPath, oldPath) => {
  menuValue.value = route.fullPath.substring(1)
})

onMounted(()=>{
  const collapsedCache = localStorage.getItem("collapsed");
  if (collapsedCache) {
    collapsed.value = JSON.parse(collapsedCache).collapsed
  }
  is.value = true
})

const renderIcon = (icon: any) => {
  return () => h(NIcon, null, {default: () => h(icon)})
}

const menuOptions = ref([
  {
    label: computed(() => t("menu.index")),
    key: 'index',
    icon: renderIcon(CloudOutline),
  },
  {
    label: computed(() => t("menu.setting")),
    key: 'setting',
    icon: renderIcon(SettingsOutline),
  },
])

const footerOptions = ref([
  {
    label: "github",
    key: 'github',
    icon: renderIcon(LogoGithub),
  },
  {
    label: computed(() => t("menu.locale")),
    key: 'locale',
    icon: renderIcon(LanguageSharp),
  },
  {
    label: computed(() => t("menu.theme")),
    key: 'theme',
    icon: theme,
  },
  {
    label: computed(() => t("menu.about")),
    key: 'about',
    icon: renderIcon(HelpCircleOutline),
  },
])

const handleUpdateValue = (key: string, item?: MenuOption) => {
  menuValue.value = key
  return router.push({path: "/" + key})
}
const handleFooterUpdate = (key: string, item?: MenuOption) => {
  if (key === "about") {
    showAppInfo.value = true
    return
  }

  if (key === "github") {
    BrowserOpenURL("https://github.com/putyy/res-downloader")
    return
  }

  if (key === "theme") {
    if (globalConfig.value.Theme === "darkTheme") {
      store.setConfig({Theme: "lightTheme"})
      return
    }
    store.setConfig({Theme: "darkTheme"})

    return
  }

  if (key === "locale") {
    if (globalConfig.value.Locale === "zh") {
      store.setConfig({Locale: "en"})
      return
    }
    store.setConfig({Locale: "zh"})
    return
  }

  menuValue.value = key
  return router.push({path: "/" + key})
}

const collapsedChange = (value: boolean)=>{
  console.log("collapsedChange",value)
  collapsed.value = value
  localStorage.setItem("collapsed", JSON.stringify({collapsed: value}))
}
</script>