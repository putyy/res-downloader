import './assets/css/main.css'

import {createApp} from 'vue'
import {createPinia} from 'pinia'
import i18n from './i18n'

import App from './App.vue'
import router from './router'
import {useIndexStore} from './stores'

const pinia = createPinia()
const app = createApp(App)
    .use(router)
    .use(i18n)
    .use(pinia)

async function bootstrap() {
    await useIndexStore(pinia).init()
    app.mount('#app')
}

bootstrap().catch(error => {
    console.error('Failed to initialize application:', error)
})
