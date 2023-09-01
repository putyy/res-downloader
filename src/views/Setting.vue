<script setup lang="ts">
import {onMounted, ref} from "vue";
import {ipcRenderer} from "electron";
import localStorageCache from "../common/localStorage";
import {ElMessage} from "element-plus";

onMounted(() => {
  saveDir.value = localStorageCache.get("save_dir")
  saveDir.value = !saveDir.value ? "" : saveDir.value
})

const saveDir = ref("")
const selectSaveDir = () => {
  ipcRenderer.invoke('invoke_select_down_dir').then(save_path => {
    if (save_path !== false) {
      saveDir.value = save_path
    }
  })
}

const onSetting = () => {
  localStorageCache.set("save_dir", saveDir.value, -1)
  ElMessage({
    message: "操作成功",
    type: 'success',
  })
}

</script>
<template lang="pug">
el-form
  el-form-item(label="保存位置")
    el-button.select-dir(@click="selectSaveDir") {{saveDir ? saveDir : '选择'}}
  el-form-item
    el-button(type="primary" @click="onSetting") 保存
</template>