<script setup lang="ts">
import {ref, provide} from 'vue'
import Sidebar from './Sidebar.vue'
import Footer from './Footer.vue'

const sidebarCollapse = ref(false)
provide('sidebarCollapse', sidebarCollapse)

</script>
<template lang="pug">
el-container
  Sidebar
  el-container
    el-main
      router-view(v-slot="{ Component, route }")
        keep-alive(v-if="route.meta.keepAlive")
          component(:is="Component")
        component(v-else :is="Component")
    el-footer
      Footer
</template>

<style lang="less" scoped>
.el-header {
  padding: 0;
}
.el-main {
  text-align: center;
}
</style>
