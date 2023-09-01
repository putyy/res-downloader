import {Component, createApp} from 'vue'
import "./style.css"
import VueApp from './App.vue'
import './samples/node-api'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import router from './route'

const app = createApp(VueApp)

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    app.component(key, <Component>component)
}

app.use(router)
  .mount('#app')
  .$nextTick(() => {
    postMessage({ payload: 'removeLoading' }, '*')
  })
