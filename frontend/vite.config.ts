import {defineConfig} from "vite"
import vue from "@vitejs/plugin-vue"
import {fileURLToPath} from "node:url"
import AutoImport from "unplugin-auto-import/vite"
import Components from "unplugin-vue-components/vite"
import {NaiveUiResolver} from "unplugin-vue-components/resolvers"
import wails from "@wailsio/runtime/plugins/vite"

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [
    vue(),
    // Adds typed custom events from the Wails-generated bindings.
    wails("./bindings"),
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
      "@bindings": fileURLToPath(new URL("./bindings", import.meta.url)),
    },
  },
  build: {
    // Preserve the Vite 3 output target for the desktop WebViews.
    target: ["es2020", "edge88", "firefox78", "chrome87", "safari13"],
    cssMinify: "esbuild",
  },
})
