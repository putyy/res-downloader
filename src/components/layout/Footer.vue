<script setup lang="ts">
import {ipcRenderer} from 'electron'
import pkg from '../../../package.json'
const v = pkg.version
const jump = (url: string)=>{
  ipcRenderer.invoke('invoke_open_default_browser', {
    url: url
  })
}
</script>
<template lang="pug">
div.line
  a.item 当前版本: {{v}}
  a.item(@click="jump('https://s.gowas.cn/d/4089-quan-ping-tai-zi-yuan-xia-zai-ruan-jian')") 获取更新&问题反馈
  a.item(@click="jump('https://github.com/putyy/res-downloader')") 软件源码
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
