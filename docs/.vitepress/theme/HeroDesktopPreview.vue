<script setup lang="ts">
import { computed, useId } from 'vue'
import { useData, withBase } from 'vitepress'

const { lang } = useData()
const id = useId()
const icon = (name: string) => `#${id}-${name}`
const copy = computed(() => lang.value.startsWith('zh') ? {
  menu: ['获取资源', '下载任务', '插件管理', '系统设置'],
  lowerMenu: ['github', 'English', '关于我们'],
  start: '开启抓取', all: '全部', clear: '清空列表', download: '批量下载',
  headers: ['域名', '类型', '预览', '状态', '描述', '资源大小', '保存路径', '操作'],
  types: ['视频', '音频', '图片', 'm3u8'],
  preview: '预览', ready: '就绪', completed: '完成',
  footer: ['证书下载', '使用文档', '软件源码', '帮助支持', '更新日志'],
} : {
  menu: ['Intercept', 'Downloads', 'Plugins', 'Setting'],
  lowerMenu: ['github', '中文', 'About'],
  start: 'Start Grabbing', all: 'All', clear: 'Clear List', download: 'Batch Download',
  headers: ['Domain', 'Type', 'Preview', 'Status', 'Description', 'Resource Size', 'Save Path', 'Operation'],
  types: ['Video', 'Audio', 'Image', 'M3U8'],
  preview: 'Preview', ready: 'Ready', completed: 'Done',
  footer: ['Certificate Download', 'Documentation', 'Source Code', 'Issues', 'Update Log'],
})

const toolbarWidths = computed(() => lang.value.startsWith('zh')
  ? { clear: 96, download: 96 }
  : { clear: 100, download: 132 })

const menuIcons = ['cloud', 'download', 'plugin', 'settings']
const lowerIcons = ['github', 'language', 'help']
const columnX = [28, 128, 206, 284, 384, 530, 630, 806]
const rows = [
  { domain: 'media.example', name: 'sunset.mp4', size: '38.6 MB', completed: true },
  { domain: 'audio.example', name: 'ambient.mp3', size: '8.2 MB', completed: false },
  { domain: 'img.example', name: 'inspiration.png', size: '2.4 MB', completed: false },
  { domain: 'live.example', name: 'playlist.m3u8', size: '—', completed: false },
]

// Local outline icons keep the miniature independent of the desktop runtime.
const paths: Record<string, string> = {
  cloud: 'M7 19h11a4 4 0 0 0 .6-7.95A7 7 0 0 0 5 9a5 5 0 0 0 2 10Z',
  download: 'M12 3v12m-4-4 4 4 4-4M5 14v6h14v-6',
  plugin: 'M9 4H5v5H3a2 2 0 0 0 0 4h2v6h5v2a2 2 0 0 0 4 0v-2h5v-6h-2a2 2 0 0 1 0-4h2V4h-5V2a2 2 0 0 0-4 0v2Z',
  settings: 'm10 3-1 3-3 1-3 3 2 2-1 4 3 1 2 3 3-1 3 1 2-3 3-1-1-4 2-2-3-3-3-1-1-3Zm2 5a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z',
  github: 'M9 20v-3c-4 1-4-2-5-2m11 5v-4c0-1 0-2-1-2 3 0 5-1 5-5 0-1-1-2-1-3V3l-4 2h-4L6 3v3C5 7 5 8 5 9c0 4 2 5 5 5-1 0-1 1-1 2',
  language: 'M3 5h12M9 2v3m4 0c-1 6-5 9-10 11m2-8c1 4 4 6 8 8m0 5 4-10 4 10m-6-4h4',
  help: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18Zm-3 6a3 3 0 0 1 6 0c0 2-3 2-3 5m0 3v.1',
  trash: 'M4 6h16M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7m4-7v7',
  grid: 'M4 4h5v5H4Zm11 0h5v5h-5ZM4 15h5v5H4Zm11 0h5v5h-5Z',
  search: 'M10 3a7 7 0 1 1 0 14 7 7 0 0 1 0-14Zm5 12 6 6',
  filter: 'M3 5h18M6 11h12m-8 6h4',
}
</script>

<template>
  <svg class="resd-desktop-preview" viewBox="0 0 1120 720" aria-hidden="true" focusable="false">
    <defs>
      <symbol v-for="(path, name) in paths" :id="`${id}-${name}`" :key="name" viewBox="0 0 24 24">
        <path :d="path" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
      </symbol>
    </defs>

    <rect width="1120" height="720" class="desktop-background" />
    <rect width="154" height="720" class="desktop-sidebar" />
    <path d="M154 0v720" class="desktop-border" />
    <g class="desktop-traffic-lights">
      <circle cx="20" cy="19" r="6" fill="#ff5f57" />
      <circle cx="41" cy="19" r="6" fill="#febc2e" />
      <circle cx="62" cy="19" r="6" fill="#28c840" />
    </g>
    <image :href="withBase('/images/logo.png')" x="53" y="51" width="48" height="48" />
    <g v-for="(item, index) in copy.menu" :key="item" :transform="`translate(8 ${124 + index * 48})`" class="desktop-nav" :class="{ active: index === 0 }">
      <rect width="138" height="42" rx="8" />
      <use :href="icon(menuIcons[index])" x="13" y="11" width="20" height="20" />
      <text x="43" y="26">{{ item }}</text>
    </g>
    <g class="desktop-collapse">
      <circle cx="153" cy="319" r="10" />
      <path d="m155 315-4 4 4 4" />
    </g>
    <g v-for="(item, index) in copy.lowerMenu" :key="item" :transform="`translate(21 ${570 + index * 46})`" class="desktop-lower-nav">
      <use :href="icon(lowerIcons[index])" width="20" height="20" />
      <text x="30" y="15">{{ item }}</text>
    </g>

    <g transform="translate(174 20)" class="desktop-toolbar">
      <g class="desktop-control">
        <rect width="112" height="32" rx="6" />
        <text x="56" y="21" text-anchor="middle">{{ copy.start }}</text>
      </g>
      <g transform="translate(120)" class="desktop-control">
        <rect width="86" height="32" rx="6" />
        <rect x="7" y="6" width="49" height="20" rx="3" class="desktop-filter-value" />
        <text x="13" y="21">{{ copy.all }}</text>
        <path d="m44 13 6 6m0-6-6 6m23-4 4 4 4-4" class="desktop-control-mark" />
      </g>
      <g transform="translate(214)" class="desktop-control danger">
        <rect :width="toolbarWidths.clear" height="32" rx="6" />
        <use :href="icon('trash')" x="10" y="8" width="16" height="16" />
        <text x="34" y="21">{{ copy.clear }}</text>
      </g>
      <g :transform="`translate(${222 + toolbarWidths.clear})`" class="desktop-control accent">
        <rect :width="toolbarWidths.download" height="32" rx="6" />
        <use :href="icon('download')" x="10" y="8" width="16" height="16" />
        <text x="34" y="21">{{ copy.download }}</text>
      </g>
      <g :transform="`translate(${230 + toolbarWidths.clear + toolbarWidths.download})`" class="desktop-control info">
        <rect width="34" height="32" rx="6" />
        <use :href="icon('grid')" x="8" y="7" width="18" height="18" />
      </g>
    </g>

    <g transform="translate(174 74)">
      <rect width="924" height="574" class="desktop-table-surface" />
      <rect width="924" height="44" class="desktop-table-header" />
      <rect x="8" y="15" width="13" height="13" rx="2" class="desktop-checkbox" />
      <text v-for="(header, index) in copy.headers" :key="header" :x="columnX[index] + 8" y="27" class="desktop-header-text">{{ header }}</text>
      <use :href="icon('search')" x="83" y="15" width="15" height="15" class="desktop-muted" />
      <use :href="icon('filter')" x="184" y="15" width="14" height="14" class="desktop-muted" />
      <use :href="icon('search')" x="485" y="15" width="15" height="15" class="desktop-muted" />
      <use :href="icon('help')" x="885" y="15" width="15" height="15" class="desktop-muted" />
      <path d="M0 44h924" class="desktop-border" />

      <g v-for="(row, index) in rows" :key="row.name" :transform="`translate(0 ${44 + index * 48})`" class="desktop-row">
        <path d="M0 48h924" class="desktop-border" />
        <rect x="8" y="18" width="13" height="13" rx="2" class="desktop-checkbox" />
        <text x="36" y="29">{{ row.domain }}</text>
        <text x="136" y="29">{{ copy.types[index] }}</text>
        <g transform="translate(214 11)" class="desktop-tag">
          <rect width="62" height="26" rx="4" />
          <text x="31" y="18" text-anchor="middle">{{ copy.preview }}</text>
        </g>
        <g transform="translate(292 11)" class="desktop-tag">
          <rect width="84" height="26" rx="4" />
          <text x="42" y="18" text-anchor="middle">{{ row.completed ? copy.completed : copy.ready }}</text>
        </g>
        <text x="392" y="29">{{ row.name }}</text>
        <text x="538" y="29">{{ row.size }}</text>
        <text v-if="row.completed" x="638" y="29" class="desktop-saved-path">~/Downloads/sunset.mp4</text>
        <g transform="translate(818 10)" class="desktop-row-actions">
          <g class="accent"><circle cx="14" cy="14" r="14" /><use :href="icon('download')" x="6" y="6" width="16" height="16" /></g>
          <g transform="translate(34)" class="danger"><circle cx="14" cy="14" r="14" /><use :href="icon('trash')" x="6" y="6" width="16" height="16" /></g>
          <g transform="translate(68)" class="muted"><circle cx="14" cy="14" r="14" /><use :href="icon('grid')" x="7" y="7" width="14" height="14" /></g>
        </g>
      </g>
    </g>

    <g class="desktop-footer">
      <text x="636" y="702" text-anchor="middle">
        <tspan v-for="(item, index) in copy.footer" :key="item" :dx="index ? 16 : 0">{{ item }}</tspan>
      </text>
    </g>
  </svg>
</template>

<style scoped>
/* Match lightTheme/darkTheme in frontend/src/themes/index.ts. */
.resd-desktop-preview {
  --desktop-background: #f3f5f4;
  --desktop-surface: #ffffff;
  --desktop-surface-muted: #edf1ef;
  --desktop-border: #dfe5e1;
  --desktop-text: #27332e;
  --desktop-muted: #616e67;
  --desktop-accent: #357458;
  --desktop-accent-soft: #e5efe9;
  --desktop-sidebar: #252d2a;
  --desktop-sidebar-text: #b9c5be;
  --desktop-sidebar-accent: #c2e6d1;
  --desktop-sidebar-active: #354b40;
  --desktop-danger: #d03050;
  --desktop-danger-soft: #fbeef1;
  --desktop-info: #2080f0;
  display: block;
  width: 100%;
  height: auto;
  aspect-ratio: 1120 / 720;
  font-family: 'PingFang SC', 'Microsoft YaHei', sans-serif;
  font-size: 13px;
  color: var(--desktop-text);
  fill: currentColor;
}

.dark .resd-desktop-preview {
  --desktop-background: #1b1c19;
  --desktop-surface: #242521;
  --desktop-surface-muted: #2d2e29;
  --desktop-border: #3d3f37;
  --desktop-text: #eee9df;
  --desktop-muted: #b9b2a5;
  --desktop-accent: #8fc6a4;
  --desktop-accent-soft: #2a3b31;
  --desktop-sidebar: #11120f;
  --desktop-sidebar-text: #beb8ac;
  --desktop-sidebar-accent: #eee9df;
  --desktop-sidebar-active: rgba(255, 255, 255, 0.12);
  --desktop-danger: #e88080;
  --desktop-danger-soft: #3d2928;
  --desktop-info: #70c0e8;
}

.desktop-background { fill: var(--desktop-background); }
.desktop-sidebar { fill: var(--desktop-sidebar); }
.desktop-border { fill: none; stroke: var(--desktop-border); stroke-width: 1; }
.desktop-nav, .desktop-lower-nav { color: var(--desktop-sidebar-text); font-size: 14px; }
.desktop-nav > rect { fill: transparent; }
.desktop-nav.active { color: var(--desktop-sidebar-accent); }
.desktop-nav.active > rect { fill: var(--desktop-sidebar-active); }
.desktop-collapse circle { fill: var(--desktop-sidebar-active); stroke: var(--desktop-sidebar-text); stroke-opacity: 0.2; }
.desktop-collapse path { fill: none; stroke: var(--desktop-sidebar-text); stroke-width: 1.4; }
.desktop-control > rect { fill: var(--desktop-surface); stroke: var(--desktop-border); }
.desktop-control > .desktop-filter-value { fill: var(--desktop-surface-muted); stroke: none; }
.desktop-control-mark { fill: none; stroke: var(--desktop-muted); stroke-width: 1.2; }
.accent { color: var(--desktop-accent); }
.danger { color: var(--desktop-danger); }
.info { color: var(--desktop-info); }
.muted, .desktop-muted { color: var(--desktop-muted); }
.desktop-control.accent > rect, .desktop-tag > rect, .desktop-row-actions .accent > circle { fill: var(--desktop-accent-soft); stroke: none; }
.desktop-control.danger > rect, .desktop-row-actions .danger > circle { fill: var(--desktop-danger-soft); stroke: none; }
.desktop-table-surface { fill: var(--desktop-surface); }
.desktop-table-header, .desktop-row-actions .muted > circle { fill: var(--desktop-surface-muted); }
.desktop-header-text { fill: var(--desktop-muted); font-weight: 500; font-size: 12px; }
.desktop-checkbox { fill: var(--desktop-surface); stroke: var(--desktop-border); }
.desktop-tag { color: var(--desktop-accent); font-size: 12px; }
.desktop-saved-path { fill: var(--desktop-accent); font-size: 12px; }
.desktop-footer { color: var(--desktop-muted); font-size: 12px; }
</style>
