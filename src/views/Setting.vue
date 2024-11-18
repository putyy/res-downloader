<script setup lang="ts">
import {onMounted, ref} from "vue"
import {ipcRenderer} from "electron"
import localStorageCache from "../common/localStorage"
import {ElMessage} from "element-plus"

const formData = ref({
  save_dir: "",
  quality: "-1",
  proxy: "",
  port: "8899",
})
const proxy_old = ref("")
const port_old = ref("")

const qualityOptions = ref([
  {
    value: '-1',
    label: '默认(推荐)'
  }, {
    value: '0',
    label: '高画质'
  }, {
    value: '1',
    label: '中画质'
  }, {
    value: '2',
    label: '低画质'
  }
])

onMounted(() => {
  const cache = localStorageCache.get("resd_config")
  if (cache) {
    formData.value = JSON.parse(cache)
  }
  proxy_old.value = formData.value.proxy
  port_old.value = formData.value.port
})

const selectSaveDir = () => {
  ipcRenderer.invoke('invoke_select_down_dir').then(save_dir => {
    console.log("save_dir", save_dir)
    if (save_dir !== false) {
      formData.value.save_dir = save_dir
    }
  })
}

const onSetting = () => {
  localStorageCache.set("resd_config", JSON.stringify(formData.value))
  ipcRenderer.invoke('invoke_set_config', Object.assign({}, formData.value))
  if (proxy_old.value != formData.value.proxy || port_old.value != formData.value.port){
    ipcRenderer.invoke('invoke_window_restart')
  }

  ElMessage({
    message: "保存成功",
    type: 'success',
  })
}

</script>
<template lang="pug">
el-form(style="max-width: 600px")
  el-form-item(label="代理端口")
    el-input(v-model="formData.port" placeholder="默认: 8899" )
  el-form-item(label="保存位置")
    div(style="display:flex;flex-direction: row;align-items: center;")
      el-input(v-model="formData.save_dir" placeholder="请选择" disabled )
      el-button(style="margin-left: 10px;" type="primary" @click="selectSaveDir") 选择
  el-form-item(label="视频号画质")
    el-select(v-model="formData.quality" placeholder="请选择")
      el-option( v-for="item in qualityOptions"
        :key="item.value"
        :label="item.label"
        :value="item.value")
  el-form-item(label="特殊代理")
    el-input(v-model="formData.proxy" placeholder="例如: http://127.0.0.1:7890 修改此项需重启本软件，如不清楚用途请勿设置。" )
  el-form-item
    el-button(type="primary" @click="onSetting") 保存
</template>
