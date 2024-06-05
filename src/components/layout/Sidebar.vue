<script setup lang="ts">
import {inject, onMounted, ref, watch} from 'vue'
import localStorageCache from "../../common/localStorage"

const appName = "爱享素材"
const sidebarCollapse = ref(inject('sidebarCollapse'))
const defaultActive = ref("/index")

onMounted(() => {
  let lastRoute = localStorageCache.get('last-route')
  defaultActive.value = lastRoute ? lastRoute : "/index"
})
</script>
<template lang="pug">
div.sidebar
  el-menu(class="menu" :collapse="sidebarCollapse" :default-active="defaultActive" router)
    div.logo
      img(src="../../assets/logo.png" width="32" height="32")
      span(v-show="!sidebarCollapse") {{appName}}
    el-menu-item(key="1" index="/index")
      el-icon
        VideoCamera
      span 嗅探
    el-menu-item(key="2" index="/about")
      el-icon
        Share
      span 帮助
    el-menu-item(key="99" index="/setting")
      el-icon
        Setting
      span 设置
</template>

<style lang="less" scoped>
.menu-icon {
  max-width: 1rem;
}

.sidebar {
  height: 100vh;
  box-shadow: .2rem 0 .6rem 0 rgba(0, 0, 0, 0.1);
  .menu {
    border-right: unset;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    .logo {
      margin-top: 1rem;
      display: flex;
      flex-direction: row;
      align-items: center;
      padding: 0 1rem;
      overflow: hidden;
      img {
        vertical-align: middle;
      }
      span {
        color: #eab728;
      }
    }
  }
  .menu:not(.el-menu--collapse) {
    width: 10rem;
  }
}
</style>
