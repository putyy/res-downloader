<script setup lang="ts">
import {ipcRenderer} from 'electron'
import pkg from '../../../package.json'
const v = pkg.version
const jump = (scene: number)=>{
  switch (scene) {
    case 1:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://github.com/putyy/res-downloader"
      })
      break;
    case 2:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://s.gowas.cn"
      })
      break;
    case 3:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://i.gowas.cn"
      })
      break;
    case 4:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://github.com/putyy/res-downloader/issues"
      })
      break;
  }
}
</script>
<template lang="pug">
div.line
  a.item 当前版本: {{v}}
  a.item 站长邮箱: gowas.work@gmail.com
  a.item(@click="jump(1)") 获取更新
  a.item(@click="jump(2)") 云盘资源
  a.item(@click="jump(3)") 图片压缩
  a.item(@click="jump(4)") 问题反馈
</template>

<style lang="less" scoped>
.line{
  display: flex;
  flex-direction: row;
  justify-content: center;
  .item{
    padding: 0 5px;
    cursor: pointer;
  }
}
</style>
