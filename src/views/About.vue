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

const str = "使用方法\n" +
    "     1. 打开本软件\n" +
    "     2. 软件首页选择要获取的资源类型（默认选中的视频）\n" +
    "     3. 打开要捕获的源， 如：视频号、网页、小程序等等\n" +
    "     4. 返回软件首页即可看到要下载的资源\n" +
    "常见问题\n" +
    "     1. 无法拦截获取\n" +
    "        手动检测系统代理是否设置正确 本软件代理地址: 127.0.0.1:8899\n" +
    "     2. 关闭软件后无法正常上网\n" +
    "        手动关闭系统代理设置\n" +
    "     3. 视频号抓取流程\n" +
    "        将需要下载的视频发给好友或者文件助手 再打开该视频即可拦截到，通常软件界面对应视频会出现标题描述、对应操作会出现解密下载按钮\n" +
    "        大视频可以复制链接通过其他工具加速下载，然后再通过对应的视频操作项进行\"视频解密\"\n" +

    "实现原理\n" +
    "     通过代理网络抓包拦截响应，筛选出有用的资源，\n" +
    "     同fiddler、charles等抓包软件、浏览器F12打开控制也能达到目的，\n" +
    "     只不过这些软件需要手动进行筛选，对于小白用户上手还是有点难度，本软件对部分资源做了特殊处理，\n" +
    "     更适合大众用户，所以就有了本项目这样的软件。\n"
</script>
<template lang="pug">
div.about
  div 1. 本软件免费、代码已开源，不会以任何形式收取费用。&nbsp;
    el-button(@click="jump(3)") 查看源码
  div 2. m3u8复制的链接如何使用? &nbsp;
    el-button(@click="jump(1)") 在线下载
    el-button(@click="jump(2)") 在线播放
  div 3. 软件如何获取更新?  &nbsp;
    el-button(@click="jump(3)") 获取更新
  div 4. 问题反馈 &nbsp;
    el-button(@click="jump(4)") 点击前往
  div.more
    pre {{str}}

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
  .more{

  }
}
</style>
