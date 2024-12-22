<template>
<div class="w-full h-full transition-all p-0">
  <div class="h-full overflow-hidden flex justify-center items-center">
      <div id="content" class="z-40 transition flex relative w-full h-full rounded-xl">
        <Sider></Sider>
        <NLayoutContent class="h-full flex-1 bg-[unset]">
          <RouterView v-slot="{ Component, route }">
            <keep-alive v-if="route.meta.keepAlive">
              <component :is="Component"></component>
            </keep-alive>
              <component v-else :is="Component"></component>
          </RouterView>
        </NLayoutContent>
      </div>
  </div>
</div>
</template>
<script lang="ts" setup>
import Sider from "./Sider.vue"
</script>
