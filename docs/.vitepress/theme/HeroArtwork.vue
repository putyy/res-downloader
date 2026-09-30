<script setup lang="ts">
import { computed } from 'vue'
import { useData } from 'vitepress'
import HeroFlipCard from './HeroFlipCard.vue'

const { lang } = useData()
const copy = computed(() => lang.value.startsWith('zh') ? {
  label: '视频、音频、图片汇聚到下载中心的示意图',
  flow: ['发现资源', '即时预览', '保存精彩'],
  audio: '留住此刻的声音',
  image: '收集灵感碎片',
  hint: '按住卡片左右拖动 · 点击或按 Enter 翻面',
} : {
  label: 'Illustration of video, audio and images flowing into the download center',
  flow: ['DISCOVER', 'PREVIEW', 'KEEP'],
  audio: 'Sounds of the moment',
  image: 'A little inspiration',
  hint: 'Drag the card · Click or press Enter to flip',
})

const waveform = [24, 42, 30, 65, 90, 52, 76, 100, 58, 84, 40, 66, 92, 48, 72, 34, 54, 24]
</script>

<template>
  <figure class="resd-artwork" :aria-label="copy.label">
    <div class="resd-art-scene">
      <div class="resd-art-orbit resd-art-orbit-outer" aria-hidden="true" />
      <div class="resd-art-orbit resd-art-orbit-inner" aria-hidden="true" />
      <span class="resd-art-star resd-art-star-one" aria-hidden="true">✦</span>
      <span class="resd-art-star resd-art-star-two" aria-hidden="true">✦</span>
      <svg class="resd-art-connections" viewBox="0 0 500 460" fill="none" aria-hidden="true">
        <path d="M378 103V151Q378 168 357 168H327M96 343H65Q45 343 45 320V259Q45 241 66 241H93" />
        <circle cx="378" cy="103" r="4" />
        <circle cx="96" cy="343" r="4" />
      </svg>

      <div class="resd-art-audio resd-art-card" aria-hidden="true">
        <div class="resd-art-card-heading"><span>AUDIO</span><span>♪</span></div>
        <div class="resd-art-waveform">
          <span v-for="(height, index) in waveform" :key="index" :style="{ height: `${height}%` }" />
        </div>
        <p>{{ copy.audio }}</p>
      </div>

      <HeroFlipCard class="resd-art-window" />

      <div class="resd-art-image resd-art-card" aria-hidden="true">
        <span class="resd-art-image-icon"><svg viewBox="0 0 32 32" width="28" height="28" fill="none" stroke="currentColor" stroke-width="1.6"><rect x="5" y="5" width="22" height="22" rx="5" /><circle cx="12" cy="12" r="2" /><path d="m6 23 7-7 5 5 4-4 5 5" /></svg></span>
        <div><span class="resd-art-card-heading">IMAGE / PNG</span><p>{{ copy.image }}</p></div>
        <span class="resd-art-image-check">✓</span>
      </div>

      <div class="resd-art-flow" aria-hidden="true">
        <template v-for="(step, index) in copy.flow" :key="step">
          <span v-if="index" class="resd-art-flow-arrow">↗</span>
          <span><i>0{{ index + 1 }}</i>{{ step }}</span>
        </template>
      </div>
    </div>
    <figcaption class="resd-art-hint">↔ {{ copy.hint }}</figcaption>
  </figure>
</template>

<style scoped>
.resd-artwork {
  --art-surface: rgba(255, 252, 248, 0.96);
  --art-border: rgba(182, 116, 62, 0.2);
  --art-muted: #877667;
  position: relative;
  container-type: inline-size;
  width: 100%;
  max-width: 520px;
  margin: 0 auto;
  color: var(--vp-c-text-1);
  font-size: 12px;
  line-height: 1.4;
  text-align: left;
}

.dark .resd-artwork {
  --art-surface: rgba(44, 36, 30, 0.97);
  --art-border: rgba(251, 146, 60, 0.23);
  --art-muted: #b6a18f;
}

.resd-art-scene { position: relative; width: 100%; aspect-ratio: 500 / 490; }
.resd-art-orbit { position: absolute; border: 1px solid var(--art-border); border-radius: 50%; }
.resd-art-orbit-outer { inset: 2% 4% 6%; transform: rotate(-22deg); }
.resd-art-orbit-inner { inset: 13% 15% 17%; border-style: dashed; opacity: 0.65; animation: resd-art-orbit-spin 50s linear infinite; }
.resd-art-orbit-inner::after {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #fb923c;
  box-shadow: 0 0 0 4px rgba(251, 146, 60, 0.12), 0 0 14px rgba(251, 146, 60, 0.5);
  transform: translate(-50%, -50%);
}
.resd-art-star { position: absolute; color: #fb923c; line-height: 1; }
.resd-art-star-one { top: 12%; left: 14%; font-size: 36px; transform: rotate(15deg); }
.resd-art-star-two { right: 4%; bottom: 17%; font-size: 23px; }
.resd-art-connections { position: absolute; inset: 0; width: 100%; height: 100%; stroke: #fb923c; stroke-width: 1.5; }
.resd-art-connections path { stroke-dasharray: 4 6; }
.resd-art-connections circle { fill: #fb923c; stroke: none; }
.resd-art-card { border: 1px solid var(--art-border); background: var(--art-surface); box-shadow: 0 16px 40px -16px rgba(103, 60, 25, 0.28); }
.resd-art-window { position: absolute; z-index: 2; top: 26%; left: 12%; width: 76%; transform: rotate(-3deg); }
.resd-art-window.is-dragging, .resd-art-window[data-back='true'] { z-index: 4; }
.resd-art-orbit, .resd-art-star, .resd-art-connections, .resd-art-audio, .resd-art-image, .resd-art-flow { pointer-events: none; }
.resd-art-audio { position: absolute; z-index: 3; top: 4%; right: 7%; width: 40%; padding: 13px 15px 12px; border-radius: 12px; transform: rotate(7deg); animation: resd-art-float 7s ease-in-out infinite; }
.resd-art-card-heading { display: flex; justify-content: space-between; color: var(--art-muted); font: 9px/1.4 var(--vp-font-family-mono); letter-spacing: 0.09em; }
.resd-art-waveform { display: flex; align-items: center; justify-content: space-between; gap: 3px; height: 32px; margin: 10px 0; }
.resd-art-waveform span { flex: 1; max-width: 4px; border-radius: 4px; background: #fb923c; }
.resd-art-card p { margin: 0; font-size: 10px; line-height: 1.5; }
.resd-art-image { position: absolute; z-index: 3; left: 2%; bottom: 0; display: flex; align-items: center; gap: 10px; max-width: 80%; padding: 12px 15px 12px 10px; border-radius: 12px; transform: rotate(4deg); animation: resd-art-float 7s ease-in-out -3s infinite; }
.resd-art-image-icon { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 8px; color: #fb923c; background: var(--vp-c-brand-soft); }
.resd-art-image p { margin-top: 4px; }
.resd-art-image-check { margin-left: 10px; color: #fb923c; }
.resd-art-flow { position: absolute; right: 3%; bottom: -6%; display: flex; align-items: center; gap: 10px; color: var(--vp-c-text-2); font-size: 9px; letter-spacing: 0.03em; }
.resd-art-flow i { margin-right: 5px; color: #fb923c; font: normal 9px/1 var(--vp-font-family-mono); }
.resd-art-flow-arrow { color: var(--art-muted); }
figcaption { margin: 40px 3% 0 0; text-align: right; color: var(--art-muted); font-size: 10px; letter-spacing: 0.08em; }
.resd-art-hint { color: var(--vp-c-text-2); letter-spacing: 0; }

@keyframes resd-art-float {
  0%, 100% { translate: 0 0; }
  50% { translate: 0 -7px; }
}

@keyframes resd-art-orbit-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (max-width: 639px) {
  .resd-art-scene { aspect-ratio: 1 / 1.1; }
  .resd-art-window { top: 25%; }
  .resd-art-audio { padding: 10px 12px; }
  .resd-art-waveform { height: 25px; margin: 7px 0; }
  .resd-art-image { bottom: -3%; padding: 8px 10px; }
  .resd-art-image-icon { width: 34px; height: 34px; }
  .resd-art-flow { bottom: -9%; gap: 6px; }
  figcaption { margin-top: 42px; }
}

@media (prefers-reduced-motion: reduce) {
  .resd-art-audio, .resd-art-image, .resd-art-orbit-inner { animation: none; }
}
</style>
