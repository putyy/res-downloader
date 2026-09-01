import './assets/css/main.css'

import {createApp} from 'vue'
import {createPinia} from 'pinia'
import i18n from './i18n'

import App from './App.vue'
import router from './router'
import {useIndexStore} from './stores'
import {reportFrontendError} from '@/services/diagnostics'

const pinia = createPinia()
const app = createApp(App)
    .use(router)
    .use(i18n)
    .use(pinia)

window.addEventListener('error', event => {
    void reportFrontendError('window.error', event.error || event.message)
})

window.addEventListener('unhandledrejection', event => {
    void reportFrontendError('unhandledrejection', event.reason)
})

app.mount('#app')
void useIndexStore(pinia).init()
