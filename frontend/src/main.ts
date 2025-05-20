import './assets/css/main.css'

import {createApp} from 'vue'
import {createPinia} from 'pinia'
import i18n from './i18n'

import App from './App.vue'
import router from './router'

createApp(App)
    .use(router)
    .use(i18n)
    .use(createPinia())
    .mount('#app')
