# 前端开发

桌面界面使用 Vue 3、TypeScript、Vite、Naive UI、Pinia 和 Vue Router，样式由 Tailwind CSS 与局部组件样式共同维护。

## 常用命令

在 `frontend` 目录执行：

```bash
npm install
npm run dev
npm run build
npm run preview
```

日常联调通常从项目根目录运行 Wails 开发命令，由 Wails 启动前端开发服务器并注入桌面运行时。

## 目录说明

```text
src/
├── api/          HTTP API 封装
├── components/   通用组件及资源、任务、插件、设置组件
├── locales/      中英文界面文案
├── router/       页面路由
├── services/     资源和任务等前端领域逻辑
├── stores/       Pinia 状态
├── types/        前端协议类型
└── views/        获取资源、下载任务、插件管理和系统设置页面
```

`wailsjs` 由 Wails 生成。后端绑定或运行时发生变化时应重新生成对应文件，不要把它当作手写业务代码维护。

## 静态检查

`npm run build` 会先运行 `vue-tsc --noEmit`，再由 Vite 生成生产资源。提交前还应检查中英文文案键是否保持一致。
