<script setup lang="ts">
import {ipcRenderer} from 'electron'
import pkg from '../../../package.json'
const v = pkg.version
const jump = (scene: number)=>{
  switch (scene) {
    case 1:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://s.gowas.cn/d/4089-quan-ping-tai-zi-yuan-xia-zai-ruan-jian"
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
        url: "https://s.gowas.cn/d/4089-quan-ping-tai-zi-yuan-xia-zai-ruan-jian"
      })
      break;
    case 5:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://www.ais.do/ivi/rr2GaZ"
      })
      break;
    case 6:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://github.com/putyy/res-downloader"
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
  a.item(@click="jump(4)") 问题反馈
div.line
  a.item 推荐:
  a.item(@click="jump(5)") Ai助手(免费)
  a.item(@click="jump(2)") 网盘资源
  a.item(@click="jump(3)") 图片无损压缩
  a.item(@click="jump(6)") 软件源码
</template>

<style lang="less" scoped>
.line{
  display: flex;
  flex-direction: row;
  justify-content: center;
  flex-wrap: wrap;
  .item{
    padding: 0 5px;
    cursor: pointer;
  }
}
</style>
