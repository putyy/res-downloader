# 文档工程

res-downloader 文档使用 VitePress，依赖在本目录独立管理。Node.js 版本要求为 22 或更高。

```bash
cd docs
npm ci
npm run dev
```

默认预览地址为 `http://127.0.0.1:8088`。

## 目录

```text
docs/
├── .vitepress/
│   ├── config.mts       # 构建、资源目录和共享站点配置
│   ├── seo.mts          # 页面元信息和多语言关联
│   ├── locales/         # 各语言的导航、搜索和界面文案
│   └── theme/           # 主题入口和样式
├── zh/
│   ├── index.md         # 中文首页
│   ├── guide/           # 中文使用指南
│   └── development/     # 中文开发文档
├── en/                  # 英文首页、使用指南和开发文档（与 zh/ 对应）
├── public/              # 各语言共用的静态资源，原样发布
│   ├── images/
│   └── plugin-sdk/
├── package.json
├── package-lock.json
└── tsconfig.json
```

中文与英文源码分别位于 `zh/` 和 `en/`，目录和文件名一一对应。构建时通过 `rewrites` 将中文页面映射到根路径，英文页面保留 `/en/` 前缀。例如，`zh/index.md` 对应 `/`，`zh/guide/settings.md` 对应 `/guide/settings.html`，英文对应 `/en/` 和 `/en/guide/settings.html`。访问根路径会直接显示中文首页。

语言配置分别位于 `.vitepress/locales/zh.mts` 和 `en.mts`，在 `config.mts` 中以 `root` 和 `en` 注册。新增或修改页面时同步维护两个语言版本。图片和 SDK 文件共用 `public/`，不按语言复制。

完整规则见[多语言文档说明](zh/development/contributing.md#多语言文档)。

## 静态检查

```bash
npm run check
npm run build
```

构建产物位于 `.vitepress/dist/`，不提交到仓库。构建后可运行 `npm run preview` 人工检查。

提交要求和验证说明见[文档贡献指南](zh/development/contributing.md#检查文档修改)。
