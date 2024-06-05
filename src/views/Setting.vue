<script setup lang="ts">
import {onMounted, ref} from "vue"
import {ipcRenderer} from "electron"
import localStorageCache from "../common/localStorage"
import {ElMessage} from "element-plus"

const saveDir = ref("")
const upstream_proxy = ref("")
const upstream_proxy_old = ref("")

onMounted(() => {
  saveDir.value = localStorageCache.get("save_dir") ? localStorageCache.get("save_dir") : ""
  upstream_proxy.value = localStorageCache.get("upstream_proxy") ? localStorageCache.get("upstream_proxy") : ""
  upstream_proxy_old.value = upstream_proxy.value
})

const selectSaveDir = () => {
  ipcRenderer.invoke('invoke_select_down_dir').then(save_path => {
    if (save_path !== false) {
      saveDir.value = save_path
    }
  })
}

const onSetting = () => {
  localStorageCache.set("save_dir", saveDir.value, -1)
  localStorageCache.set("upstream_proxy", upstream_proxy.value, -1)
  if (upstream_proxy_old.value != upstream_proxy.value){
    ipcRenderer.invoke('invoke_window_restart')
  }
  ElMessage({
    message: "操作成功",
    type: 'success',
  })
}

</script>
<template lang="pug">
el-form
  el-form-item(label="保存位置")
    el-button(@click="selectSaveDir") {{saveDir ? saveDir : '选择'}}
  el-form-item(label="代理服务")
    el-input(v-model="upstream_proxy" placeholder="例如: http://127.0.0.1:7890 修改此项需重启本软件" )
  el-form-item
    el-button(type="primary" @click="onSetting") 保存
</template>
