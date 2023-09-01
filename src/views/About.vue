<script lang="ts" setup>
import {ipcRenderer} from 'electron'
const jump = (scene: number)=>{
  switch (scene) {
    case 1:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://m3u8-down.gowas.cn/"
      })
      break;
    case 2:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://www.baidu.com/s?wd=m3u8%E5%9C%A8%E7%BA%BF"
      })
      break;
    case 3:
      ipcRenderer.invoke('invoke_open_default_browser', {
        url: "https://github.com/putyy/res-downloader"
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
div.about
  div 1. 本软件免费、源代码已经开源，不会以任何形式收取费用。&nbsp;
    el-button(@click="jump(3)") 查看源码
  div 2. m3u8复制的链接如何使用? &nbsp;
    el-button(@click="jump(1)") 在线下载
    el-button(@click="jump(2)") 在线播放
  div 3. 软件如何获取更新?  &nbsp;
    el-button(@click="jump(3)") 获取更新
  div 4. 问题反馈 &nbsp;
    el-button(@click="jump(4)") 点击前往
</template>

<style lang="less">
.about{
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  text-align: left;
  font-size: 20px;
  color: #ec8d13;
  .problem{
    padding: .3rem;
    white-space: pre-wrap;
  }
}
</style>