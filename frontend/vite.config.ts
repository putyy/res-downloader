import {defineConfig} from "vite"
import vue from "@vitejs/plugin-vue"
import {fileURLToPath} from "node:url"
import AutoImport from "unplugin-auto-import/vite"
import Components from "unplugin-vue-components/vite"
import {NaiveUiResolver} from "unplugin-vue-components/resolvers"

export default defineConfig({
  server: {
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [
    vue(),
    AutoImport({
      dts: "auto-imports.d.ts",
      imports: [
        "vue",
        {
          "naive-ui": ["useDialog", "useMessage", "useNotification", "useLoadingBar"],
        },
      ],
    }),
    Components({
      dts: "components.d.ts",
      resolvers: [NaiveUiResolver()],
    }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    // Keep the output compatible with the desktop WebViews.
    target: ["es2020", "edge88", "firefox78", "chrome87", "safari13"],
    cssMinify: "esbuild",
  },
})
