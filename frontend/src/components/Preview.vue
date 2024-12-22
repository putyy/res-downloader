<template>
  <NModal
      style="--wails-draggable:no-drag"
      :show="showModal"
      :on-update:show="changeShow"
      preset="card"
      class="w-[540px] h-auto"
      title="预览"
      display-directive="show"
      :on-after-enter="onAfterEnter"
      :on-after-leave="onAfterLeave"
  >
    <div class="flex justify-center w-full h-[80vh]">
      <video
          class="video-js vjs-default-skin w-full h-full"
          ref="videoPlayer"
          controls
          preload="auto"
      ></video>
    </div>
  </NModal>
</template>

<script setup lang="ts">
import { ref} from "vue"
import "video.js/dist/video-js.css"
import videojs from "video.js"
import flvjs from "flv.js"
import axios from "axios"
// @ts-ignore
import { getDecryptionArray } from '@/assets/js/decrypt.js'
import type Player from "video.js/dist/types/player"

const videoPlayer = ref<HTMLElement | any>(null)
let player: Player | null = null
let flvPlayer: flvjs.Player | null = null
let sourceBuffer: SourceBuffer | null = null
let isLoading = false
let isOver = false
let startByte = 0
const chunkSize = 5 * 1024 * 1024
let endByte = startByte + chunkSize - 1
let decodeArr: any = null
let mediaSource: MediaSource
let rowUrl = ''

const props = defineProps<{
  showModal: boolean
  previewRow: any
}>()
const emits = defineEmits(["update:showModal"])

const changeShow = (value: boolean) => emits("update:showModal", value)

const onAfterEnter = () => {
  if (props.previewRow.DecodeKey) {
    playVideoWithoutTotalLength()
  } else if (props.previewRow.Classify === "live") {
    playFlvStream()
  } else {
    setupVideoJsPlayer()
  }
}

const onAfterLeave = () => {
  if (props.previewRow.Classify === "live" && flvPlayer) {
    flvPlayer.unload()
    flvPlayer.detachMediaElement()
    flvPlayer.destroy()
    flvPlayer = null
  } else if (player) {
    player.pause()
  }
  if (startByte){
    videoPlayer.value?.pause()
    videoPlayer.value?.removeEventListener("seeking", handleSeeking)
    videoPlayer.value?.removeEventListener("timeupdate", handleTimeupdate)
  }
}

const playFlvStream = () => {
  try {
    if (!flvjs.isSupported() || !videoPlayer.value) return

    flvPlayer = flvjs.createPlayer({ type: "flv", url: props.previewRow.Url })
    flvPlayer.attachMediaElement(videoPlayer.value)
    flvPlayer.load()
    flvPlayer.play()
  }catch (e) {

  }
}

const setupVideoJsPlayer = () => {
  if (!videoPlayer.value) return

  if (!player) {
    player = videojs(videoPlayer.value, {
      controls: true,
      autoplay: false,
      preload: "auto",
    })
  }

  player.src({
    src: props.previewRow.Url,
    type: props.previewRow.ContentType,
    withCredentials: true,
  })
  player.play()
}

const playVideoWithoutTotalLength = () => {
  rowUrl = buildUrlWithParams(props.previewRow.Url)
  mediaSource = new MediaSource()
  videoPlayer.value.src = URL.createObjectURL(mediaSource)
  videoPlayer.value.play()
  isOver = false
  startByte = 0
  endByte = startByte + chunkSize - 1
  decodeArr = getDecryptionArray(props.previewRow.DecodeKey)
  sourceBuffer = null
  mediaSource.addEventListener("sourceopen", () => {
    sourceBuffer = mediaSource.addSourceBuffer('video/mp4; codecs="avc1.42E01E, mp4a.40.2"')
    downloadChunk()
  })

  videoPlayer.value.addEventListener("seeking", handleSeeking)
  videoPlayer.value.addEventListener("timeupdate", handleTimeupdate)
}

const buildUrlWithParams = (url: string) => {
  const parsedUrl = new URL(url)
  const queryParams = parsedUrl.searchParams
  if (queryParams.has("encfilekey") && queryParams.has("token")) {
    return `${parsedUrl.origin}${parsedUrl.pathname}?encfilekey=${queryParams.get("encfilekey")}&token=${queryParams.get("token")}`
  }
  return url
}

const handleSeeking = () => {
  console.log('handleSeeking')
  const currentTime = videoPlayer.value.currentTime
  const bufferedEnd = videoPlayer.value.buffered.end(videoPlayer.value.buffered.length - 1)

  if (currentTime > bufferedEnd && !isLoading && !isOver) {
    downloadChunk()
  }
}

const handleTimeupdate = () => {
  if (videoPlayer.value.buffered.length > 0) {
    const bufferedEnd = videoPlayer.value.buffered.end(videoPlayer.value.buffered.length - 1);
    const timeToEnd = bufferedEnd - videoPlayer.value.currentTime;

    // 如果剩余播放时间不足10秒，加载更多数据
    if (timeToEnd < 10 && !isLoading && !isOver) {
      downloadChunk()
    }
  }
}

const downloadChunk = () => {
  if (sourceBuffer?.updating) return;

  isLoading = true
  try {
    axios.get(rowUrl, { headers: { Range: `bytes=${startByte}-${endByte}` }, responseType: "arraybuffer" })
        .then(response => {
          let chunk = new Uint8Array(response.data)

          // 解密前 13702 字节
          for (let i = 0; i < chunk.byteLength && startByte + i < decodeArr.length; i++) {
            chunk[i] ^= decodeArr[startByte + i]
          }

          // 更新字节范围，准备请求下一个分片
          startByte = endByte + 1
          endByte = startByte + chunkSize - 1

          if (sourceBuffer && !sourceBuffer.updating) {
            sourceBuffer.appendBuffer(chunk);
          } else {
            console.error("SourceBuffer is updating, cannot append buffer right now.");
          }
          isLoading = false
          if (response.data.byteLength === 0) {
            isOver = true
            mediaSource?.endOfStream()
          }
        })
        .catch(() => {
          isLoading = false
          isOver = true
        })
  }catch (e) {
    isLoading = false
    isOver = true
  }
}

</script>