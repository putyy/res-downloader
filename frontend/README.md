# 前端开发

Vue 3 + TypeScript + Vite，UI 使用 Naive UI，样式使用 Tailwind CSS。

## 常用命令

在 `frontend` 目录执行：

```bash
npm install
npm run dev
npm run check
npm run build
npm run preview
npm run format
npm run format:check
```

桌面联调从仓库根目录启动 Wails 开发模式。

## 目录说明

```text
src/
├── api/          HTTP API 封装
├── components/   界面组件
├── locales/      中英文界面文案
├── router/       页面路由
├── services/     业务逻辑
├── stores/       Pinia 状态
├── types/        前端协议类型
└── views/        页面
```

`wailsjs/` 由 Wails 生成，请勿手动修改。界面文案需同步维护中英文。
