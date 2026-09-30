<script setup lang="ts">
import { computed, ref } from 'vue'
import { useData } from 'vitepress'
import HeroDesktopPreview from './HeroDesktopPreview.vue'

const { lang } = useData()
const copy = computed(() => lang.value.startsWith('zh') ? {
  control: '资源预览和桌面界面示意：按住左右拖动，或点击、按 Enter 或空格翻面',
  video: '一段值得珍藏的风景',
  downloading: '保存到你的桌面',
} : {
  control: 'Resource preview and desktop illustration: drag horizontally, click, or press Enter or Space to flip',
  video: 'A view worth keeping',
  downloading: 'Saving to your desktop',
})

const angle = ref(0)
const dragging = ref(false)
const showingBack = computed(() => {
  const turn = ((angle.value % 360) + 360) % 360
  return turn > 90 && turn < 270
})

// Capture on the stationary hit area so an edge-on card stays draggable.
let pointerId: number | null = null
let startX = 0
let startAngle = 0
let degreesPerPixel = 1
let moved = false
let suppressClick = false

function startDrag(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0 || pointerId !== null) return
  const target = event.currentTarget as HTMLButtonElement
  target.focus({ preventScroll: true })
  target.setPointerCapture(event.pointerId)
  pointerId = event.pointerId
  startX = event.clientX
  startAngle = angle.value
  degreesPerPixel = 300 / Math.max(target.offsetWidth, 1)
  moved = false
  suppressClick = false
  dragging.value = true
}

function moveDrag(event: PointerEvent) {
  if (event.pointerId !== pointerId) return
  if (event.pointerType === 'mouse' && !(event.buttons & 1)) {
    finishDrag(event)
    return
  }
  const distance = event.clientX - startX
  if (Math.abs(distance) > 5) moved = true
  if (moved) angle.value = startAngle + Math.max(-180, Math.min(180, distance * degreesPerPixel))
}

function finishDrag(event: PointerEvent) {
  if (event.pointerId !== pointerId) return
  suppressClick = moved || event.type !== 'pointerup'
  pointerId = null
  dragging.value = false
  angle.value = Math.round(angle.value / 180) * 180
  const target = event.currentTarget as HTMLButtonElement
  if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId)
}

function flip(event: MouseEvent) {
  // Ignore the synthesized click after a drag, but allow keyboard/AT clicks.
  if (suppressClick && event.detail !== 0) {
    suppressClick = false
    return
  }
  if (dragging.value) return
  suppressClick = false
  angle.value = Math.round(angle.value / 180) * 180 + 180
}

function cancelOnBlur(event: FocusEvent) {
  if (pointerId === null) return
  const target = event.currentTarget as HTMLButtonElement
  const capturedPointer = pointerId
  pointerId = null
  dragging.value = false
  suppressClick = true
  angle.value = Math.round(angle.value / 180) * 180
  if (target.hasPointerCapture(capturedPointer)) target.releasePointerCapture(capturedPointer)
}
</script>

<template>
  <button
    type="button"
    class="resd-flip-card"
    :class="{ 'is-dragging': dragging }"
    :aria-label="copy.control"
    :aria-pressed="showingBack"
    :data-back="showingBack"
    @pointerdown="startDrag"
    @pointermove="moveDrag"
    @pointerup="finishDrag"
    @pointercancel="finishDrag"
    @lostpointercapture="finishDrag"
    @blur="cancelOnBlur"
    @click="flip"
    @dragstart.prevent
  >
    <span class="resd-flip-zoom">
      <span class="resd-flip-rotor" :style="{ transform: `rotateY(${angle}deg)` }" aria-hidden="true">
        <span class="resd-flip-face resd-flip-front">
          <span class="resd-window-bar">
            <span class="resd-window-dots"><i /><i /><i /></span>
            <span>res-downloader</span>
            <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 3v12m-4-4 4 4 4-4M5 15v5h14v-5" /></svg>
          </span>
          <span class="resd-video">
            <svg class="resd-landscape" viewBox="0 0 360 156" preserveAspectRatio="xMidYMid slice" fill="none">
              <circle cx="270" cy="44" r="24" fill="#ffe0ab" />
              <path d="m-20 133 98-79 97 93 76-71 131 87H-20Z" fill="#e6833f" />
              <path d="m-20 164 113-59 65 41 80-33 144 65H-20Z" fill="#b9542b" />
              <path d="m-20 167 119-26 92 28 191-34v49H-20Z" fill="#7c3924" />
            </svg>
            <span class="resd-video-type">VIDEO / MP4</span>
            <span class="resd-play"><svg viewBox="0 0 24 24" width="21" height="21" fill="currentColor"><path d="m9 5 11 7-11 7Z" /></svg></span>
            <span class="resd-duration">02:48</span>
          </span>
          <span class="resd-download">
            <strong>{{ copy.video }}</strong>
            <span class="resd-download-meta"><span>{{ copy.downloading }}</span><span>72<small>%</small></span></span>
            <span class="resd-progress"><span /></span>
            <span class="resd-formats"><span>MP4</span><span>MP3</span><span>PNG</span><span>M3U8</span><span>+ …</span></span>
          </span>
        </span>

        <span class="resd-flip-face resd-flip-back">
          <HeroDesktopPreview />
        </span>
      </span>
    </span>
  </button>
</template>

<style scoped>
.resd-flip-card {
  display: block;
  padding: 0;
  border: 0;
  border-radius: 16px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  perspective: 1100px;
  cursor: grab;
  touch-action: pan-y pinch-zoom;
  user-select: none;
  -webkit-user-select: none;
  -webkit-touch-callout: none;
}

.resd-flip-card.is-dragging { z-index: 4; cursor: grabbing; }
.resd-flip-card:focus, .resd-flip-card:focus-visible { outline: none; }
.resd-flip-zoom { display: block; transform: scale(1); transform-style: preserve-3d; transition: transform 420ms cubic-bezier(0.22, 0.8, 0.3, 1); }
[data-back='true'] .resd-flip-zoom { transform: scale(1.26); }
.resd-flip-rotor { position: relative; display: grid; transform-style: preserve-3d; transition: transform 420ms cubic-bezier(0.22, 0.8, 0.3, 1); }
.is-dragging .resd-flip-rotor { transition: none; }
.resd-flip-face { display: block; grid-area: 1 / 1; min-width: 0; overflow: hidden; border: 1px solid var(--art-border); border-radius: 16px; background: var(--art-surface); box-shadow: 0 16px 40px -16px rgba(103, 60, 25, 0.28); backface-visibility: hidden; -webkit-backface-visibility: hidden; pointer-events: none; }
.resd-flip-back { position: absolute; inset: auto 0; top: 50%; border-color: #dfe5e1; transform: translateY(-50%) rotateY(180deg); }
.dark .resd-flip-back { border-color: #3d3f37; }
.resd-window-bar { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 8px; height: clamp(26px, 7.2cqw, 36px); padding: 0 14px; font: 10px/1 var(--vp-font-family-mono); color: var(--art-muted); }
.resd-window-dots { display: flex; gap: 4px; }
.resd-window-dots i { width: 5px; height: 5px; border-radius: 50%; background: #fb923c; opacity: 0.4; }
.resd-window-dots i:first-child { opacity: 1; }
.resd-video { display: block; position: relative; height: clamp(76px, 29cqw, 148px); margin: 0 9px; overflow: hidden; border-radius: 9px; background: #f5b477; color: #fff9ed; }
.resd-landscape { position: absolute; width: 100%; height: 100%; }
.resd-video-type, .resd-duration { position: absolute; font: 9px/1 var(--vp-font-family-mono); letter-spacing: 0.06em; }
.resd-video-type { top: 12px; left: 12px; color: #6f3a20; }
.resd-duration { bottom: 10px; right: 12px; }
.resd-play { position: absolute; inset: 0; display: grid; place-items: center; width: 43px; height: 43px; margin: auto; border: 1px solid rgba(255, 255, 255, 0.6); border-radius: 50%; background: rgba(255, 249, 237, 0.18); }
.resd-download { display: block; padding: 13px 16px 15px; }
.resd-download > strong { font-size: clamp(11px, 2.8cqw, 14px); font-weight: 600; }
.resd-download-meta { display: flex; justify-content: space-between; align-items: baseline; margin-top: 7px; color: var(--art-muted); font-size: 10px; }
.resd-download-meta > span:last-child { color: #fb923c; font: 18px/1 var(--vp-font-family-mono); }
.resd-download-meta small { margin-left: 2px; font-size: 10px; }
.resd-progress { display: block; height: 4px; margin-top: 8px; border-radius: 4px; background: var(--vp-c-default-soft); overflow: hidden; }
.resd-progress > span { display: block; height: 100%; width: 72%; border-radius: inherit; background: linear-gradient(90deg, #fb923c, #fdba74); }
.resd-formats { display: flex; gap: 6px; margin-top: 12px; font: 8px/1 var(--vp-font-family-mono); color: var(--art-muted); }
.resd-formats span { padding: 4px 5px; border: 1px solid var(--art-border); border-radius: 4px; }

@media (max-width: 639px) {
  .resd-window-bar { height: 30px; }
  .resd-download { padding: 10px 12px 12px; }
}

@media (prefers-reduced-motion: reduce) {
  .resd-flip-zoom { transition: none; }
  .resd-flip-rotor { transform: none !important; transition: none; }
  .resd-flip-face { transform: none; backface-visibility: visible; -webkit-backface-visibility: visible; }
  .resd-flip-back { visibility: hidden; transform: translateY(-50%); }
  [data-back='true'] .resd-flip-front { visibility: hidden; }
  [data-back='true'] .resd-flip-back { visibility: visible; }
}
</style>
