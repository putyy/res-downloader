<script setup lang="ts">
import { useData } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import HeroArtwork from './HeroArtwork.vue'

const { lang } = useData()
const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #home-hero-info-before>
      <div class="resd-hero-eyebrow">
        <span class="resd-hero-spark" aria-hidden="true">✦</span>
        {{ lang.startsWith('zh') ? '开源工具，自由探索' : 'OPEN SOURCE. OPEN POSSIBILITIES.' }}
      </div>
    </template>
    <template #home-hero-actions-after>
      <div class="resd-hero-platforms">
        <span>{{ lang.startsWith('zh') ? '为你的桌面而来' : 'AT HOME ON YOUR DESKTOP' }}</span>
        <span class="resd-hero-platform-list">Windows <i aria-hidden="true">/</i> macOS <i aria-hidden="true">/</i> Linux</span>
      </div>
    </template>
    <template #home-hero-image>
      <HeroArtwork />
    </template>
  </Layout>
</template>
