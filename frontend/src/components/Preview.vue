<template>
  <NModal
      style="--wails-draggable:no-drag"
      :show="showModal"
      :on-update:show="changeShow"
      preset="card"
      class="w-[720px] h-auto"
      :title="t('index.preview')"
      display-directive="show"
      :on-after-enter="onAfterEnter"
      :on-after-leave="onAfterLeave"
  >
    <div class="flex flex-col justify-center items-center gap-3 w-full h-[80vh] overflow-auto">
      <NAlert v-if="previewError" type="error" :show-icon="false" class="w-full shrink-0 break-all">{{ previewError }}</NAlert>
      <NImage v-if="renderer === 'image'" :src="imageContentURL" object-fit="contain" class="max-w-full max-h-full" @error="onImagePreviewError"/>
      <audio v-else-if="renderer === 'audio'" ref="audioPlayer" class="w-full" controls preload="metadata"/>
      <iframe v-else-if="renderer === 'pdf'" :src="previewURL()" class="w-full h-full border-0" sandbox="allow-same-origin"/>
      <pre v-else-if="renderer === 'text'" class="w-full h-full whitespace-pre-wrap break-words overflow-auto p-3">{{ textContent }}</pre>
      <video v-else class="w-full h-full min-h-0 bg-black" ref="videoPlayer" controls playsinline preload="auto"></video>
    </div>
  </NModal>
</template>

<script setup lang="ts">
import {computed, onUnmounted, ref} from "vue"
import mpegts from "mpegts.js"
import Hls, {ErrorTypes} from "hls.js"
import axios from "axios"
import {useI18n} from 'vue-i18n'
import {resourcePreviewURL} from '@/services/resources'

const {t} = useI18n()
const videoPlayer = ref<HTMLVideoElement | null>(null)
const audioPlayer = ref<HTMLAudioElement | null>(null)
const textContent = ref("")
const imageContentURL = ref("")
const previewError = ref("")
let mpegtsPlayer: mpegts.Player | null = null
let hlsPlayer: Hls | null = null
let hlsMediaRecoveryAttempted = false
let hlsNetworkRecoveryAttempted = false
let nativeHLSFallbackAttempted = false
let imageProxyAttempted = false
let imageObjectURL = ""
let imageLoadMode: 'idle' | 'direct' | 'proxy' = 'idle'
let imageLoadGeneration = 0

const props = defineProps<{
  showModal: boolean
  previewRow: any
}>()
const emits = defineEmits(["update:showModal"])

const renderer = computed(() => props.previewRow?.preview?.renderer || 'video')
const previewMIME = computed(() => props.previewRow?.preview?.mime || '')
const previewTrack = computed(() => {
  const trackID = props.previewRow?.preview?.trackId
  const tracks = Array.isArray(props.previewRow?.tracks) ? props.previewRow.tracks : []
  return tracks.find((track: any) => track?.id === trackID) || (tracks.length === 1 ? tracks[0] : null)
})
const isHLS = computed(() =>
    props.previewRow?.kind === 'stream.hls' ||
    props.previewRow?.metadata?.['stream.protocol'] === 'hls' ||
    previewMIME.value.toLowerCase().includes('mpegurl'),
)

const changeShow = (value: boolean) => emits("update:showModal", value)

const onAfterEnter = () => {
  previewError.value = ""
  if (renderer.value === 'image') {
    loadImagePreview()
    return
  }
  if (renderer.value === 'audio') {
    if (audioPlayer.value) {
      audioPlayer.value.src = previewURL()
      audioPlayer.value.load()
    }
    return
  }
  if (renderer.value === 'text') {
    axios.get(previewURL(), {responseType: 'text'})
        .then(response => textContent.value = String(response.data ?? ''))
        .catch(() => textContent.value = '')
    return
  }
  if (renderer.value !== 'video') return
  if (previewMIME.value.includes('flv') || props.previewRow?.preview?.mode === 'flv') {
    playFlvStream()
    return
  }
  if (isHLS.value) {
    playHLSStream()
    return
  }
  playNativeVideo()
}

const onAfterLeave = () => {
  cleanupVideoPlayback()
  if (audioPlayer.value) {
    audioPlayer.value.pause()
    audioPlayer.value.removeAttribute('src')
    audioPlayer.value.load()
  }
  textContent.value = ""
  resetImagePreview()
  previewError.value = ""
}

const loadImagePreview = () => {
  imageLoadGeneration++
  revokeImageContentURL()
  imageProxyAttempted = false
  const source = previewTrack.value?.url || props.previewRow?.Url || ''
  const processors = previewTrack.value?.processors
  if (source && (!Array.isArray(processors) || processors.length === 0)) {
    imageLoadMode = 'direct'
    imageContentURL.value = source
    return
  }
  loadProxiedImage()
}

const onImagePreviewError = () => {
  if (imageLoadMode === 'idle' || !imageContentURL.value) return
  if (!imageProxyAttempted) {
    loadProxiedImage()
    return
  }
  previewError.value = t('index.preview_load_failed', {message: 'image'})
}

const loadProxiedImage = () => {
  imageProxyAttempted = true
  imageLoadMode = 'proxy'
  const generation = ++imageLoadGeneration
  revokeImageContentURL()
  axios.get(previewURL(), {responseType: 'blob'})
      .then(response => {
        if (generation !== imageLoadGeneration || imageLoadMode !== 'proxy') return
        imageObjectURL = URL.createObjectURL(response.data)
        imageContentURL.value = imageObjectURL
      })
      .catch(error => {
        if (generation !== imageLoadGeneration || imageLoadMode !== 'proxy') return
        const status = error?.response?.status ? `HTTP ${error.response.status}` : error?.message || 'image'
        previewError.value = t('index.preview_load_failed', {message: status})
      })
}

const revokeImageContentURL = () => {
  if (imageObjectURL) {
    URL.revokeObjectURL(imageObjectURL)
    imageObjectURL = ""
  }
  imageContentURL.value = ""
}

const resetImagePreview = () => {
  imageLoadGeneration++
  imageLoadMode = 'idle'
  imageProxyAttempted = false
  revokeImageContentURL()
}

const playFlvStream = () => {
  const features = mpegts.getFeatureList()
  if (!mpegts.isSupported() || !features.mseLivePlayback || !videoPlayer.value) {
    previewError.value = t('index.preview_unsupported')
    return
  }
  mpegtsPlayer = mpegts.createPlayer({
    type: "flv",
    isLive: true,
    url: previewURL(),
  }, {
    enableStashBuffer: false,
    lazyLoad: false,
    liveBufferLatencyChasing: true,
  })
  mpegtsPlayer.on(mpegts.Events.ERROR, (_type, detail) => {
    if (detail === mpegts.ErrorDetails.MEDIA_CODEC_UNSUPPORTED) {
      previewError.value = t('index.preview_unsupported')
      return
    }
    previewError.value = t('index.preview_load_failed', {message: detail || 'FLV'})
  })
  mpegtsPlayer.attachMediaElement(videoPlayer.value)
  mpegtsPlayer.load()
  playMedia(videoPlayer.value)
}

const playHLSStream = () => {
  if (!videoPlayer.value) return
  const media = videoPlayer.value
  const source = previewURL()
  hlsMediaRecoveryAttempted = false
  hlsNetworkRecoveryAttempted = false
  nativeHLSFallbackAttempted = false

  // Prefer hls.js in WebView. Video.js may choose WebKit's native HLS path and
  // fail with MEDIA_ERR_SRC_NOT_SUPPORTED without falling back to its VHS engine.
  if (Hls.isSupported()) {
    hlsPlayer = new Hls({
      enableWorker: true,
      lowLatencyMode: true,
    })
    hlsPlayer.on(Hls.Events.MANIFEST_PARSED, () => playMedia(media))
    hlsPlayer.on(Hls.Events.ERROR, (_event, data) => {
      if (!data.fatal || !hlsPlayer) return
      if (data.type === ErrorTypes.NETWORK_ERROR && !hlsNetworkRecoveryAttempted) {
        hlsNetworkRecoveryAttempted = true
        hlsPlayer.startLoad()
        return
      }
      if (data.type === ErrorTypes.MEDIA_ERROR && !hlsMediaRecoveryAttempted) {
        hlsMediaRecoveryAttempted = true
        hlsPlayer.recoverMediaError()
        return
      }
      const status = data.response?.code ? `HTTP ${data.response.code}` : data.details
      hlsPlayer.destroy()
      hlsPlayer = null
      if (playNativeHLS(media, source)) return
      previewError.value = t('index.preview_load_failed', {message: status})
    })
    hlsPlayer.loadSource(source)
    hlsPlayer.attachMedia(media)
    return
  }

  if (playNativeHLS(media, source)) return
  previewError.value = t('index.preview_unsupported')
}

const playNativeHLS = (media: HTMLVideoElement, source: string) => {
  if (nativeHLSFallbackAttempted || !media.canPlayType('application/vnd.apple.mpegurl')) return false
  nativeHLSFallbackAttempted = true
  media.onerror = () => {
    const code = media.error?.code
    previewError.value = t('index.preview_load_failed', {message: code ? `MEDIA_ERR_${code}` : 'HLS'})
  }
  media.src = source
  media.load()
  playMedia(media)
  return true
}

const playNativeVideo = () => {
  if (!videoPlayer.value) return
  videoPlayer.value.onerror = () => {
    const code = videoPlayer.value?.error?.code
    previewError.value = t('index.preview_load_failed', {message: code ? `MEDIA_ERR_${code}` : 'media'})
  }
  videoPlayer.value.src = previewURL()
  videoPlayer.value.load()
  playMedia(videoPlayer.value)
}

const playMedia = (media: HTMLMediaElement) => {
  media.play().catch(error => {
    // Autoplay restrictions still leave the native play button available.
    if (error?.name !== 'NotAllowedError' && error?.name !== 'AbortError') {
      previewError.value = t('index.preview_load_failed', {message: error?.message || error?.name || 'media'})
    }
  })
}

const cleanupVideoPlayback = () => {
  if (hlsPlayer) {
    hlsPlayer.destroy()
    hlsPlayer = null
  }
  if (mpegtsPlayer) {
    mpegtsPlayer.unload()
    mpegtsPlayer.detachMediaElement()
    mpegtsPlayer.destroy()
    mpegtsPlayer = null
  }
  if (videoPlayer.value) {
    videoPlayer.value.onerror = null
    videoPlayer.value.pause()
    videoPlayer.value.removeAttribute('src')
    videoPlayer.value.load()
  }
}

onUnmounted(() => {
  cleanupVideoPlayback()
  resetImagePreview()
})

const previewURL = () => {
  return resourcePreviewURL(props.previewRow)
}
</script>