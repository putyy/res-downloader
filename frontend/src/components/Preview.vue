<template>
  <NModal
      style="--wails-draggable:no-drag"
      :show="showModal"
      :on-update:show="changeShow"
      preset="card"
      class="w-[540px] h-auto"
      :title="t('index.preview')"
      display-directive="show"
      :on-after-enter="onAfterEnter"
      :on-after-leave="onAfterLeave"
  >
    <div class="flex flex-col gap-2 w-full h-[80vh]">
      <div v-if="isHevc" class="px-3 py-2 text-sm text-yellow-900 bg-yellow-100 border border-yellow-300 rounded">
        当前直播为 HEVC/H.265，内置播放器可能无法解码。你可以：
        <button class="ml-2 px-2 py-1 bg-yellow-300 hover:bg-yellow-400 rounded" @click="openInBrowser">用浏览器打开</button>
        <button class="ml-2 px-2 py-1 bg-gray-200 hover:bg-gray-300 rounded" @click="copyLink">复制直链</button>
      </div>
      <video
          class="video-js vjs-default-skin w-full h-full"
          ref="videoPlayer"
          controls
          preload="auto"
          autoplay
          muted
          playsinline
          webkit-playsinline
          x-webkit-airplay="allow"
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
import {useI18n} from 'vue-i18n'
import {useIndexStore} from "../stores"
import mpegts from "mpegts.js"
import {BrowserOpenURL} from "../../wailsjs/runtime"

const {t} = useI18n()
const store = useIndexStore()
const videoPlayer = ref<HTMLElement | any>(null)
let player: Player | null = null
let flvPlayer: flvjs.Player | null = null
let mpPlayer: any | null = null
let sourceBuffer: SourceBuffer | null = null
let isLoading = false
let isOver = false
let startByte = 0
const chunkSize = 5 * 1024 * 1024
let endByte = startByte + chunkSize - 1
let decodeArr: any = null
let mediaSource: MediaSource
let rowUrl = ''
const isHevc = ref(false)
let lastPlayUrl = ''

// helpers used by HEVC提示条
const openInBrowser = () => {
  if (lastPlayUrl) {
    BrowserOpenURL(lastPlayUrl)
  }
}
const copyLink = async () => {
  try {
    if (lastPlayUrl) {
      await navigator.clipboard.writeText(lastPlayUrl)
      console.log('[Preview] link copied:', lastPlayUrl)
      try { window.alert?.('直链已复制到剪贴板') } catch {}
    }
  } catch (e) {
    console.warn('[Preview] copy failed', e)
  }
}

const props = defineProps<{
  showModal: boolean
  previewRow: any
}>()
const emits = defineEmits(["update:showModal"])

const changeShow = (value: boolean) => emits("update:showModal", value)

const onAfterEnter = () => {
  console.log('[Preview] onAfterEnter, row=', props.previewRow)
  isHevc.value = false
  if (props.previewRow.DecodeKey) {
    playVideoWithoutTotalLength()
  } else if (props.previewRow.Classify === "live") {
    console.log('[Preview] classify=live, use playFlvStream')
    playFlvStream()
  } else {
    console.log('[Preview] classify not live, use video.js, type=', props.previewRow.ContentType)
    setupVideoJsPlayer()
  }
}

const onAfterLeave = () => {
  if (props.previewRow.Classify === "live" && flvPlayer) {
    flvPlayer.unload()
    flvPlayer.detachMediaElement()
    flvPlayer.destroy()
    flvPlayer = null
  }
  if (props.previewRow.Classify === "live" && mpPlayer) {
    try { mpPlayer.unload(); mpPlayer.detachMediaElement(); mpPlayer.destroy() } catch {}
    mpPlayer = null
  } else if (player) {
    player.pause()
  }
  isHevc.value = false
  if (startByte){
    videoPlayer.value?.pause()
    videoPlayer.value?.removeEventListener("seeking", handleSeeking)
    videoPlayer.value?.removeEventListener("timeupdate", handleTimeupdate)
  }
}

const playFlvStream = () => {
  try {
    if (!videoPlayer.value) return

    const headersStr = props.previewRow.OtherData?.headers || ""
    const base = (window as any).$baseUrl || (store as any).baseUrl?.value || 'http://127.0.0.1:8899'
    console.log('[Preview] baseUrl=', base)
    const proxiedUrl = `${base}/api/preview?url=${encodeURIComponent(props.previewRow.Url)}${headersStr?`&headers=${encodeURIComponent(headersStr)}`:''}`
    console.log('[Preview] proxiedUrl=', proxiedUrl)
    lastPlayUrl = `${base}/api/play?url=${encodeURIComponent(props.previewRow.Url)}${headersStr?`&headers=${encodeURIComponent(headersStr)}`:''}`

    // WKWebView/Safari 有时不支持 MSE，直接在系统浏览器打开我们已验证的播放页
    const flvSupported = !!flvjs.isSupported?.() || (typeof flvjs.isSupported === 'function' && flvjs.isSupported())
    const mpegtsSupported = !!(mpegts && mpegts.isSupported && mpegts.isSupported())
    console.log('[Preview] support -> flv:', flvSupported, 'mpegts:', mpegtsSupported)

    // 先发一个 GET 探测请求（短超时+no-store），确保 Network 可见
    try {
      const ctrl = new AbortController()
      const tid = setTimeout(() => ctrl.abort(), 3000)
      fetch(proxiedUrl + `&t=${Date.now()}`,
        { method: 'GET', cache: 'no-store', signal: ctrl.signal } as any)
        .then(r => {
          clearTimeout(tid)
          console.log('[Preview] PROBE GET /api/preview status=', r.status)
        }, e => {
          clearTimeout(tid)
          console.warn('[Preview] PROBE GET error', e)
        })
    } catch (e) { console.warn('[Preview] PROBE exception', e) }
    if (!flvSupported && !mpegtsSupported) {
      const playUrl = `${base}/api/play?url=${encodeURIComponent(props.previewRow.Url)}${headersStr?`&headers=${encodeURIComponent(headersStr)}`:''}`
      BrowserOpenURL(playUrl)
      console.log('[Preview] neither flv nor mpegts supported, open browser:', playUrl)
      return
    }

    let started = false
    let settled = false
    const markStarted = (tag: string) => {
      if (settled) return
      settled = true
      started = true
      console.log('[Preview] started by', tag)
    }
    const tryFlv = () => {
      if (settled) return
      if (!flvSupported) return fallbackToBrowser()
      try {
        flvPlayer = flvjs.createPlayer(
          { type: "flv", isLive: true, url: proxiedUrl },
          { enableWorker: true, enableStashBuffer: false, stashInitialSize: 32 } as any
        )
        flvPlayer.attachMediaElement(videoPlayer.value)
        flvPlayer.on(flvjs.Events.ERROR, (type: any, detail: any, info: any) => {
          console.warn('[Preview] flv error:', type, detail, info)
          if (!settled) {
            try { flvPlayer?.unload(); flvPlayer?.detachMediaElement(); flvPlayer?.destroy(); } catch {}
            fallbackToBrowser()
          }
        })
        // 尝试读取媒体信息
        try { (flvPlayer as any).on((flvjs as any).Events.MEDIA_INFO, (info: any) => {
          console.log('[Preview] flv MEDIA_INFO', info)
          const vc = (info && (info.videoCodec || info.videoCodecName)) || ''
          if (typeof vc === 'string' && vc.toLowerCase().startsWith('hvc')) {
            isHevc.value = true
          }
        }) } catch {}
        flvPlayer.on(flvjs.Events.STATISTICS_INFO, (stat: any) => console.log('[Preview] flv STAT', stat))
        flvPlayer.load()
        console.log('[Preview] flv.js created, waiting events...')
        const onOk = () => markStarted('flv-media')
        videoPlayer.value.addEventListener('loadedmetadata', onOk, { once: true })
        videoPlayer.value.addEventListener('playing', onOk, { once: true })
        const pr: any = (flvPlayer as any).play?.()
        if (pr && pr.catch) pr.catch(() => {})
      } catch (e) {
        console.warn('[Preview] flv.js init error -> browser', e)
        fallbackToBrowser()
      }
    }

    const fallbackToBrowser = () => {
      if (settled) return
      settled = true
      BrowserOpenURL(lastPlayUrl)
      console.log('[Preview] fallback -> open browser:', lastPlayUrl)
    }
    const tryMpegts = () => {
      if (settled) return
      if (!mpegtsSupported) return fallbackToBrowser()
      try {
        const mp = mpegts.createPlayer(
          { type: "flv", isLive: true, url: proxiedUrl },
          {
            enableWorker: true,
            enableStashBuffer: false,
            stashInitialSize: 32,
            autoCleanupSourceBuffer: true,
            liveBufferLatencyChasing: true,
            deferLoadAfterSourceOpen: true,
            seekType: 'range',
          } as any
        )
        mpPlayer = mp
        mp.attachMediaElement(videoPlayer.value)
        mp.load()
        console.log('[Preview] mpegts.js created, waiting events...')
        mp.on(mpegts.Events.ERROR, (type: any, detail: any) => {
          console.warn('[Preview] mpegts error:', type, detail)
          if (!settled) {
            try { mp.unload(); mp.detachMediaElement(); mp.destroy() } catch {}
            mpPlayer = null
            tryFlv()
          }
        })
        mp.on(mpegts.Events.MEDIA_INFO, (info: any) => {
          console.log('[Preview] mpegts MEDIA_INFO', info)
          const vc = (info && (info.videoCodec || info.videoCodecName)) || ''
          if (typeof vc === 'string' && vc.toLowerCase().startsWith('hvc')) {
            isHevc.value = true
          }
        })
        mp.on(mpegts.Events.STATISTICS_INFO, (stat: any) => console.log('[Preview] mpegts STAT', stat))
        const onOk = () => markStarted('mpegts-media')
        videoPlayer.value.addEventListener('loadedmetadata', onOk, { once: true })
        videoPlayer.value.addEventListener('playing', onOk, { once: true })
        // safety timer
        setTimeout(() => {
          if (!settled) {
            console.warn('[Preview] mpegts timeout, try flv')
            try { mp.unload(); mp.detachMediaElement(); mp.destroy() } catch {}
            mpPlayer = null
            tryFlv()
          }
        }, 8000)
        const pr = (videoPlayer.value as any).play?.()
        if (pr && pr.catch) pr.catch(() => {})
      } catch (e) {
        console.warn('[Preview] mpegts.js init error -> try flv', e)
        tryFlv()
      }
    }
    // 优先 mpegts.js，若失败再尝试 flv.js，最后打开浏览器
    try {
      // 先试 mpegts
      if (mpegtsSupported) {
        tryMpegts()
      } else if (flvSupported) {
        tryFlv()
      } else {
        fallbackToBrowser()
      }
    } catch (e) {
      console.warn('[Preview] flv.js init error -> fallback mpegts', e)
      tryMpegts()
    }

    // 如果最终也没有成功，会在 tryMpegts 内部 fallbackToBrowser
    // 通用视频元素事件日志
    try {
      videoPlayer.value.muted = true
      videoPlayer.value.addEventListener('error', (ev: any) => {
        const ve = videoPlayer.value as HTMLVideoElement
        console.warn('[Preview] video error', ve?.error?.code, ve?.error?.message)
      }, { once: true })
    } catch {}

  }catch (e) {
    console.error('[Preview] playFlvStream outer error', e)
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