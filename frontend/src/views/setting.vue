<template>
  <div class="h-full relative p-5 overflow-y-auto [&::-webkit-scrollbar]:hidden" :key="renderKey">
    <NForm
        :model="formValue"
        size="medium"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
        style="--wails-draggable:no-drag"
        class="w-[700px]"
    >
      <NFormItem label="Host" path="Host">
        <NInput v-model:value="formValue.Host" placeholder="127.0.0.1"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          {{ t("setting.restart_tip") }}
        </NTooltip>
      </NFormItem>

      <NFormItem label="Port" path="Port">
        <NInput v-model:value="formValue.Port" placeholder="8899"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          {{ t("setting.restart_tip") }}
        </NTooltip>
      </NFormItem>

      <NFormItem :label="t('setting.upstream_proxy')" path="UpstreamProxy">
        <NInput v-model:value="formValue.UpstreamProxy" placeholder="http://127.0.0.1:7890"/>
        <NSwitch v-model:value="formValue.OpenProxy" class="ml-1"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          {{ t("setting.upstream_proxy_tip") }}
        </NTooltip>
      </NFormItem>

      <NFormItem :label="t('setting.save_dir')" path="SaveDirectory">
        <NInput :value="formValue.SaveDirectory" :placeholder="t('setting.save_dir')"/>
        <NButton strong secondary type="primary" @click="selectDir" class="ml-1">{{ t('common.select') }}</NButton>
      </NFormItem>

      <div class="grid grid-cols-2">
        <NFormItem :label="t('setting.filename_rules')" path="FilenameLen">
          <NInputNumber v-model:value="formValue.FilenameLen" :min="0" :max="9999" placeholder="0"/>
          <NSwitch v-model:value="formValue.FilenameTime" class="ml-1"></NSwitch>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            {{ t("setting.filename_rules_tip") }}
          </NTooltip>
        </NFormItem>

        <NFormItem :label="t('setting.quality')" path="Quality">
          <NSelect v-model:value="formValue.Quality" :options="options"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            {{ t("setting.quality_tip") }}
          </NTooltip>
        </NFormItem>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <NFormItem :label="t('setting.auto_proxy')" path="AutoProxy">
          <NSwitch v-model:value="formValue.AutoProxy"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            {{ t("setting.auto_proxy_tip") }}
          </NTooltip>
        </NFormItem>

        <NFormItem :label="t('setting.full_intercept')" path="WxAction">
          <NSwitch v-model:value="formValue.WxAction"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            {{ t("setting.full_intercept_tip") }}
          </NTooltip>
        </NFormItem>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <NFormItem :label="t('setting.download_proxy')" path="DownloadProxy">
          <NSwitch v-model:value="formValue.DownloadProxy"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            {{ t("setting.download_proxy_tip") }}
          </NTooltip>
        </NFormItem>

        <NFormItem :label="t('setting.connections')" path="TaskNumber">
          <NInputNumber v-model:value="formValue.TaskNumber" :min="2" :max="64"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            {{ t("setting.connections_tip") }}
          </NTooltip>
        </NFormItem>
      </div>

      <NFormItem label="UserAgent" path="UserAgent">
        <NInput v-model:value="formValue.UserAgent" placeholder="默认UserAgent"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          {{ t("setting.user_agent_tip") }}
        </NTooltip>
      </NFormItem>

      <NFormItem label="Headers" path="Headers">
        <NInput v-model:value="formValue.UseHeaders" placeholder="User-Agent,Referer,Authorization,Cookie"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          {{ t("setting.use_headers_tip") }}
        </NTooltip>
      </NFormItem>

      <NFormItem :label="t('setting.mime_map')" path="MimeMap">
        <NInput
            v-model:value="MimeMap"
            type="textarea"
            rows="11"
            placeholder='{"video/mp4": { "Type": "video","Suffix": ".mp4"}}'
        />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          {{ t("setting.mime_map_tip") }}
        </NTooltip>
      </NFormItem>
    </NForm>
  </div>
</template>

<script lang="ts" setup>
import {HelpCircleOutline} from "@vicons/ionicons5"
import {ref, watch} from "vue"
import {useIndexStore} from "@/stores"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {computed} from "vue"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const store = useIndexStore()

const options = computed(() =>
    t("setting.quality_value")
        .split(",")
        .map((value, index) => ({ value: index, label: value }))
)

const formValue = ref<appType.Config>(Object.assign({}, store.globalConfig))

const MimeMap = ref(formValue.value.MimeMap ? JSON.stringify(formValue.value.MimeMap, null, 2) : "")
const renderKey = ref(999)

watch(formValue.value, () => {
  store.setConfig(formValue.value)
}, {deep: true})

watch(MimeMap, () => {
  store.setConfig({
    MimeMap: JSON.parse(MimeMap.value)
  })
})

watch(() => {
  return store.globalConfig.Theme
}, () => {
  formValue.value.Theme = store.globalConfig.Theme
})

watch(() => store.globalConfig.Locale, () => {
  formValue.value.Locale = store.globalConfig.Locale
  renderKey.value++
})

const selectDir = () => {
  appApi.openDirectoryDialog().then((res: any) => {
    if (res.code === 1) {
      formValue.value.SaveDirectory = res.data.folder
    }
  }).catch((err: any) => {
    window?.$message?.error(err)
  });
}
</script>