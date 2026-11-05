<template>
  <div class="relative isolate overflow-hidden text-app-text bg-app-background w-full h-full transition-all p-0">
    <div class="h-full overflow-hidden flex justify-center items-center">
      <div id="content" class="z-40 transition flex relative w-full h-full rounded-xl">
        <Sider></Sider>
        <NLayoutContent class="app-workspace relative bg-app-background h-full min-w-0 flex-1 [--wails-draggable:no-drag]">
          <!-- Pages reserve 20px of top padding. Keep dragging inside that blank
               space, away from the native resize edge and page controls. -->
          <div aria-hidden="true" class="absolute top-1.5 left-0 right-1.5 z-50 h-3.5 [--wails-draggable:drag]"></div>
          <RouterView v-slot="{Component, route}">
            <KeepAlive>
              <component v-if="route.meta.keepAlive" :is="Component" :key="route.name"></component>
            </KeepAlive>
            <component v-if="!route.meta.keepAlive" :is="Component" :key="route.fullPath"></component>
          </RouterView>
        </NLayoutContent>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import Sider from "./Sider.vue"
</script>
