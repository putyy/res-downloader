<template>
  <div class="h-full relative p-5 overflow-y-auto [&::-webkit-scrollbar]:hidden">
    <NForm
        :model="formValue"
        size="medium"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
        style="--wails-draggable:no-drag"
        class="w-[500px]"
    >
      <NFormItem label="Host" path="Host">
        <NInput v-model:value="formValue.Host" placeholder="127.0.0.1"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          如果不清楚请保持默认，修改后请重启软件
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
          如果不清楚保持默认，修改后请重启软件
        </NTooltip>
      </NFormItem>

      <NFormItem label="上游代理" path="UpstreamProxy">
        <NInput v-model:value="formValue.UpstreamProxy" placeholder="例如: http://127.0.0.1:7890"/>
        <NSwitch v-model:value="formValue.OpenProxy" class="ml-1"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          可结合其他代理工具，用于访问国外网站、以及正常网络无法访问的资源
        </NTooltip>
      </NFormItem>

      <NFormItem label="保存位置" path="SaveDirectory">
        <NInput :value="formValue.SaveDirectory" placeholder="保存位置"/>
        <NButton strong secondary type="primary" @click="selectDir" class="ml-1">选择</NButton>
      </NFormItem>

      <div class="grid grid-cols-2">
        <NFormItem label="文件命名" path="FilenameLen">
          <NInputNumber v-model:value="formValue.FilenameLen" :min="0" :max="9999" placeholder="0"/>
          <NSwitch v-model:value="formValue.FilenameTime" class="ml-1"></NSwitch>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            输入框控制文件命名的长度(不含时间、0为无效)，开关控制文件末尾是否添加时间标识
          </NTooltip>
        </NFormItem>

        <NFormItem label="清晰度" path="Quality">
          <NSelect v-model:value="formValue.Quality" :options="options"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            视频号有效
          </NTooltip>
        </NFormItem>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <NFormItem label="自动拦截" path="AutoProxy">
          <NSwitch v-model:value="formValue.AutoProxy"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            打开软件时自动启用拦截
          </NTooltip>
        </NFormItem>

        <NFormItem label="全量拦截" path="WxAction">
          <NSwitch v-model:value="formValue.WxAction"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            微信视频号是否全量拦截，否：只拦截视频详情
          </NTooltip>
        </NFormItem>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <NFormItem label="下载代理" path="DownloadProxy">
          <NSwitch v-model:value="formValue.DownloadProxy"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            进行下载时使用代理请求
          </NTooltip>
        </NFormItem>

        <NFormItem label="连接数量" path="TaskNumber">
          <NInputNumber v-model:value="formValue.TaskNumber" :min="2" :max="64"/>
          <NTooltip trigger="hover">
            <template #trigger>
              <NIcon size="18" class="ml-1 text-gray-500">
                <HelpCircleOutline/>
              </NIcon>
            </template>
            如不清楚请保持默认，通常CPU核心数*2，用于分片下载
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
          如不清楚请保持默认
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
          定义下载时可使用的header参数，逗号分割
        </NTooltip>
      </NFormItem>

      <NFormItem label="拦截规则" path="MimeMap">
        <NInput
            v-model:value="MimeMap"
            type="textarea"
            rows="11"
            placeholder='{"content-type": { "Type": "分类名称","Suffix": "后缀"}}'
        />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="18" class="ml-1 text-gray-500">
              <HelpCircleOutline/>
            </NIcon>
          </template>
          拦截规则，json格式，不清楚请勿改动
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

const store = useIndexStore()

const options = [
  {
    value: 0,
    label: "默认(推荐)"
  }, {
    value: 1,
    label: "超清"
  }, {
    value: 2,
    label: "高画质"
  }, {
    value: 3,
    label: "中画质"
  }, {
    value: 4,
    label: "低画质"
  }
]

const formValue = ref<appType.Config>(Object.assign({}, store.globalConfig))

const MimeMap = ref(formValue.value.MimeMap ? JSON.stringify(formValue.value.MimeMap, null, 2) : "")

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