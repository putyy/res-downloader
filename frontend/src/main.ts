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

const store = useIndexStore(pinia)

const bootstrap = async () => {
    try {
        // Resolve saved/system language before the startup screen's first render.
        await store.loadConfig()
    } catch (error) {
        // Mount with the English fallback; init retries and exposes startup errors.
        void reportFrontendError('startup.config', error)
    }
    app.mount('#app')
    void store.init()
}

void bootstrap()
