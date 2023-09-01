<script lang="ts" setup>
import {onMounted, ref} from "vue"
import {ElMessage} from "element-plus"
import {ipcRenderer} from 'electron'
import {onUnmounted} from "@vue/runtime-core"
import localStorageCache from "../common/localStorage"

const parseUrls = ref([
  "https://www.8090g.cn/jiexi/?url=",
  "https://jx.m3u8.tv/jiexi/?url=",
  "https://www.playm3u8.cn/jiexi.php?url=",
  "https://www.8090.la/8090/?url=",
  "https://jx.xmflv.com/?url=",
  "https://www.8090g.cn/?url=",
  "https://dm.xmflv.com:4433/?url=",
])

const useParseUrl = ref("")
const playUrl = ref("")
const iframeSrc = ref("")
const descText = ref(
    "支持各大视频付费、VIP电影电视剧解析免费观看: 爱奇艺、优酷、腾讯、乐视、土豆、芒果等\r\n若视频播放异常或时长不对，请尝试【更换线路】或【退出软件重新打开】即可解决！\r\n如有线路不行，请把页面拉倒最下方，发邮件给站长！"
)

onMounted(() => {
  useParseUrl.value = parseUrls.value[0]
  let dataCache = localStorageCache.get("res-vip-parse-data")
  if (dataCache) {
    useParseUrl.value = dataCache.useParseUrl
    playUrl.value = dataCache.playUrl
    // iframeSrc.value = useParseUrl.value + encodeURI(playUrl.value)
  }
})

const parsePlay = () => {
  if (!playUrl) {
    ElMessage({
      message: "请填写播放地址",
      type: 'warning',
    })
    return
  }
  iframeSrc.value = useParseUrl.value + encodeURI(playUrl.value)
}

const parseFullPlay = () => {
  if (!playUrl) {
    ElMessage({
      message: "请填写播放地址",
      type: 'warning',
    })
    return
  }
  ipcRenderer.invoke('invoke_resources_preview', {url: useParseUrl.value + encodeURI(playUrl.value)}).catch(() => {
  })
}

onUnmounted(() => {
  localStorageCache.set("res-vip-parse-data", {useParseUrl: useParseUrl.value, playUrl: playUrl.value}, -1)
})

</script>
<template lang="pug">
el-main.play-box
  iframe.iframe(:src="iframeSrc")
  el-form
    el-form-item(label="线路选择:")
      el-select(v-model="useParseUrl")
        el-option(v-for="(v, k) in parseUrls" :value="v" :label="'线路'+(k+1)")
    el-form-item(label="播放地址:")
      el-input(v-model="playUrl" type="textarea" placeholder="爱奇艺、优酷、腾讯、芒果、乐视、土豆")
    el-form-item
      el-button(type="primary" @click="parsePlay()") 立即播放
      el-button(type="primary" @click="parseFullPlay()") 全屏播放
  el-row.desc {{descText}}
</template>
<style scoped lang="less">
.play-box {
  width: 100%;
  height: 100%;

  .iframe {
    top: 0;
    bottom: 0;
    left: 0;
    border: 0;
    background-color: #211f1f;
    width: 100%;
    height: 80%;
  }
}

.desc {
  color: red;
  font-size: 20px;
  white-space: pre-wrap;
  text-align: left;
}
</style>