import {defineConfig, loadEnv} from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import {NaiveUiResolver} from 'unplugin-vue-components/resolvers'

// https://vitejs.dev/config/
// export default defineConfig({
//   plugins: [vue()]
// })
export default defineConfig((env) => {
    const viteEnv = loadEnv(env.mode, process.cwd())

    return {
        plugins: [
            vue(),
            AutoImport({
                imports: [
                    'vue',
                    {
                        'naive-ui': [
                            'useDialog',
                            'useMessage',
                            'useNotification',
                            'useLoadingBar'
                        ]
                    }
                ]
            }),
            Components({
                resolvers: [NaiveUiResolver()]
            })
        ],
        resolve: {
            alias: {
                '@': path.resolve(process.cwd(), 'src'),
            },
        },
    }
})
