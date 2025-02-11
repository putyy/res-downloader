<template>
  <div class="h-full relative">
    <NForm
        :model="formValue"
        size="medium"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
        style="--wails-draggable:no-drag"
        class="px-5 py-5"
    >
      <NFormItem label="代理Host" path="Port" size="small">
        <NInput v-model:value="formValue.Host" placeholder="127.0.0.1" style="width:256px"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>如果不清楚保持默认就行，修改后请重启软件</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="代理端口" path="Port" size="small">
        <NInput v-model:value="formValue.Port" placeholder="8899" style="width:256px"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>如果不清楚保持默认就行，修改后请重启软件</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="保存位置" path="SaveDirectory" size="small">
        <NSpace>
          <NInput v-model:value="formValue.SaveDirectory" disabled placeholder="保存位置" style="width:256px"/>
          <NButton strong secondary type="success" @click="selectDir">选择</NButton>
        </NSpace>
      </NFormItem>
      <NFormItem label="文件命名" path="FilenameLen" size="small">
        <NInputNumber v-model:value="formValue.FilenameLen" :min="0" :max="9999" placeholder="0" style="width:256px"/>
        <NSwitch class="pl-1" v-model:value="formValue.FilenameTime" aria-placeholder="随机数">
          <template #checked>
            是
          </template>
          <template #unchecked>
            否
          </template>
        </NSwitch>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>输入框控制文件命名的长度(不含时间、0为无效，此选项有描述信息时有效)，开关控制文件末尾是否添加时间标识</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="主题" path="theme" size="small">
        <NRadio :checked="formValue.Theme === 'lightTheme'" value="lightTheme" name="theme" @change="handleChange">浅色主题</NRadio>
        <NRadio :checked="formValue.Theme === 'darkTheme'" value="darkTheme" name="theme" @change="handleChange">深色主题</NRadio>
      </NFormItem>
      <NFormItem label="自动拦截" path="AutoProxy" size="small">
        <NSwitch v-model:value="formValue.AutoProxy" />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>打开软件时动启用拦截</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="清晰度" path="Quality" size="small">
        <NSelect v-model:value="formValue.Quality" :options="options" class="w-64" />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>视频号有效</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="全量拦截" path="Quality" size="small">
        <NSwitch v-model:value="formValue.WxAction" />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>微信视频号是否全量拦截，否：只拦截视频详情</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="上游代理" path="UpstreamProxy" size="small">
        <NInput v-model:value="formValue.UpstreamProxy" placeholder="例如: http://127.0.0.1:7890" style="width:256px"/>
        <NSwitch class="pl-1" v-model:value="formValue.OpenProxy" />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>可结合其他代理工具，用于访问国外网站、以及正常网络无法访问的资源(格式http://username:password@your.proxy.server:port)</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="下载代理" path="DownloadProxy" size="small">
        <NSwitch v-model:value="formValue.DownloadProxy" />
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>进行下载时使用代理请求</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="连接数" path="TaskNumber" size="small">
        <NInputNumber v-model:value="formValue.TaskNumber" :min="2" :max="64" class="w-64"/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>如不清楚请保持默认，通常CPU核心数*2，用于分片下载</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label="UserAgent" path="UserAgent" size="small">
        <NInput v-model:value="formValue.UserAgent" style="width:256px" placeholder=""/>
        <NTooltip trigger="hover">
          <template #trigger>
            <NIcon size="20" class="pl-1">
              <HelpCircleOutline />
            </NIcon>
          </template>
          <span>如不清楚请保持默认</span>
        </NTooltip>
      </NFormItem>
      <NFormItem label=" " path="UserAgent" size="small">
        <NButton strong secondary type="success" @click="save" class="w-20">保存</NButton>
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

watch(()=>{
  return store.globalConfig.Theme
}, ()=>{
  formValue.value.Theme = store.globalConfig.Theme
})

const handleChange = (e: Event)=>{
  formValue.value.Theme = (e.target as HTMLInputElement).value
}

const selectDir = () => {
  appApi.openDirectoryDialog().then((res: any) => {
    if (res.code === 1) {
      formValue.value.SaveDirectory = res.data.folder
    }
  }).catch((err: any) => {
    window?.$message?.error(err)
  });
}

const save = () => {
  store.setConfig(formValue.value)
  window?.$message?.success("保存成功")
}
</script>