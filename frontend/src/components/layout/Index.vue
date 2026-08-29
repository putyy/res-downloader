<template>
  <div class="app-shell w-full h-full transition-all p-0">
    <div class="sakura-decoration" aria-hidden="true">
      <span v-for="index in 12" :key="index"></span>
    </div>
    <div class="h-full overflow-hidden flex justify-center items-center">
      <div id="content" class="z-40 transition flex relative w-full h-full rounded-xl">
        <Sider></Sider>
        <NLayoutContent class="h-full flex-1 bg-[unset]">
          <RouterView v-slot="{ Component, route }">
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
