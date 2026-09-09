---
description: 参与 res-downloader：反馈问题、改进文档、开发插件和贡献代码，了解 VitePress 文档的本地预览、静态构建与部署。
---

# 参与贡献

欢迎通过反馈问题、改进文档、开发插件或提交代码参与 res-downloader。

## 反馈问题

提交 [GitHub Issue](https://github.com/putyy/res-downloader/issues) 时，请尽量提供：

- 操作系统和应用版本；
- 相关插件及其版本；
- 可以重复执行的操作步骤；
- 实际结果和期望结果；
- 已脱敏的错误提示或日志。

请勿公开 Cookie、Authorization、账号、管理员密码、带私人签名的下载地址或其他敏感数据。

## 改进文档

普通用户文档应优先说明“在哪里操作、选项有什么作用、遇到问题怎么办”，避免加入不影响使用的内部实现细节。修正文案、补充截图、完善安装步骤和帮助翻译都可以直接提交 Pull Request。

### 本地预览文档

文档站点使用 VitePress，依赖独立放在 `docs/`，不与桌面应用的 `frontend/` 共用。安装 Node.js 22 或更高版本后，在仓库根目录执行：

```bash
cd docs
npm ci
npm run dev
```

默认监听 `127.0.0.1:8088`，浏览器打开 <http://127.0.0.1:8088> 即可预览。修改 Markdown 后页面自动更新，按 `Ctrl+C` 停止服务。依赖安装完成后，站点的样式、脚本和搜索不依赖外部 CDN。

需要更换端口或允许局域网访问时，在 `docs/` 执行：

```bash
npm run dev -- --port 9000
npm run dev -- --host 0.0.0.0
```

### 构建与部署文档

在 `docs/` 执行静态检查和构建：

```bash
npm run check
npm run build
```

构建产物位于 `docs/.vitepress/dist/`，每篇文档生成包含正文的独立 HTML，同时生成站点地图和本地搜索索引。文档路径使用 `.html` 后缀，目录首页使用 `/`，例如 `/zh/guide/installation.html` 和 `/en/development/plugin-sdk.html`。

部署时将 `.vitepress/dist/` 的内容发布到静态服务器的网站根目录，不能继续直接发布 `docs/` 源目录。服务器需要支持目录默认文件 `index.html`，并让不存在的路径返回 HTTP 404；可以使用生成的 `404.html` 作为错误页，不要将所有路径回退到首页。线上无需运行 Node.js。

网站地址配置为 `https://res.putyy.com/`。如果更换域名，需要同步修改 `.vitepress/config.mts` 中的 `hostname` 和 `public/robots.txt` 中的站点地图地址。本配置按域名根目录部署，不包含旧 Docsify hash 链接的跳转兼容。

人工检查构建产物时，在 `docs/` 执行：

```bash
npm run preview
```

发布前需人工检查桌面与手机布局、浅色与深色模式、中英文搜索、语言切换、文内锚点、代码复制，以及插件 SDK 文件下载。发布后检查子页面直接访问和刷新、404 状态码，并在搜索平台提交 `https://res.putyy.com/sitemap.xml`。

### 文档结构与资源

- `docs/` 是文档工程根目录；`zh/` 和 `en/` 分别保存中英文页面，其中 `index.md` 为语言首页，`guide/` 为使用指南，`development/` 为架构、贡献与插件开发文档。根目录 `index.md` 将访问者跳转到 `/zh/`。工程入口说明位于 `docs/README.md`，不会生成站点页面。
- `.vitepress/config.mts` 管理构建、共享资源与语言注册；`.vitepress/locales/zh.mts` 和 `en.mts` 分别管理中英文导航、页面目录、搜索和界面文案；主题样式位于 `.vitepress/theme/custom.css`。
- 每篇文档在 frontmatter 中维护 `description`；标题默认取首个一级标题，也可以用 `title` 覆盖。构建时据此生成每页标题、摘要、canonical 和分享信息。
- Markdown 之间使用相对 `.md` 链接，跨分类时使用 `../guide/` 或 `../development/`。移动页面时同步更新导航、首页入口、中英文 README、示例及项目协作规则中的路径。
- 图片、图标、`robots.txt`、`.nojekyll` 和 SDK 文件统一维护在 `public/`，由 Vite 原样发布，无需额外的复制脚本。页面通过 `/images/show.png`、`/plugin-sdk/plugin-v1.schema.json` 等根路径引用公共资源，各语言共用同一份文件。仓库 README 则使用 `docs/public/` 下的相对路径。
- `node_modules/`、`.vitepress/cache/` 和 `.vitepress/dist/` 不提交；`package-lock.json`、`public/` 和所有页面源码需要提交。

### 多语言文档

中文和英文分别注册为 `locales.zh` 与 `locales.en`，页面放在 `docs/zh/` 和 `docs/en/`，使用 `/zh/` 与 `/en/` 地址。网站根路径通过静态 HTML 跳转到中文首页；根入口不进入搜索索引或站点地图，其 canonical 指向 `/zh/`。静态服务器也可以为 `/` 配置到 `/zh/` 的 HTTP 重定向。

增加一种语言时：

1. 在 `docs/<语言代码>/` 下添加首页及译文，保留与中文相同的相对目录和文件名。例如 `en/guide/getting-started.md` 对应 `zh/guide/getting-started.md`。
2. 参考 `.vitepress/locales/zh.mts` 和 `en.mts` 新建语言配置，维护 `label`、`lang`、站点描述、导航、侧边栏、页脚和界面文案。导航与首页按钮指向带语言前缀的地址，例如 `/en/guide/getting-started`。
3. 在 `config.mts` 中导入配置，注册到 `locales`，并将搜索文案注册到 `themeConfig.search.options.locales` 的同名键。搜索索引按语言生成；分词配置共用，确保构建和浏览器搜索使用同一规则。
4. 翻译各页面的 `title`、`description`、正文与链接文字，并同步文内锚点。语言内的文档链接使用相对路径；图片和公开 SDK 文件仍引用根路径，不复制到语言目录中。

语言菜单会跳转到当前页面的对应译文。只注册完整准备好对应页面和导航目标的语言，避免切换后出现 404。正文不会自动翻译，也不会自动使用中文填充缺失译文；新增或修改文档时应同步维护中英文版本。

`.vitepress/seo.mts` 使用页面所属语言生成 canonical、分享标题和语言标签，仅在同一篇文档实际存在多个译文时输出 `hreflang`，默认语言为中文。站点地图也按实际页面关联不同语言。新增语言后需执行静态构建，并人工检查语言切换、导航、搜索、直达链接和页面显示。

## 开发插件

新增站点适配时，优先开发独立插件，不要把站点判断写入通用下载器。请从[插件开发指南](plugins.md)和[示例项目](https://github.com/putyy/res-downloader/tree/master/examples/plugins)开始，并提交至少一个不含隐私数据的离线 fixture。

公开插件可以按照[扩展商店发布说明](extension-store.md)发布，无需把插件代码合并到主项目。

## 贡献代码

Bug 修复和小型文档更新可以直接提交 Pull Request。新功能、架构调整或大型重构建议先创建 Issue 讨论，以免实现方向与项目规划不一致。

完整的 PR 范围、标题格式和提交前检查要求，以仓库根目录的 [CONTRIBUTING.md](https://github.com/putyy/res-downloader/blob/master/CONTRIBUTING.md) 为准。

### 客户端前端开发

客户端前端依赖独立维护在 `frontend/`，需要 **Node.js 22.12 或更高版本**；Windows、macOS 发布流程和 Linux 构建镜像统一使用 Node.js 22。在仓库根目录执行：

```bash
cd frontend
npm ci
npm run check
npm run build
```

`check` 使用 `vue-tsc -b` 检查 Vue 页面及 Vite 配置；`build` 会先执行同样的检查，再生成 `frontend/dist/`。Wails 构建使用 `npm ci` 按锁文件安装依赖。更新依赖时应一起提交 `package.json` 与 `package-lock.json`。

`tsconfig.json` 只组织项目引用，页面与浏览器类型放在 `tsconfig.app.json`，构建配置与 Node 类型放在 `tsconfig.node.json`。`auto-imports.d.ts` 和 `components.d.ts` 由 Vite 插件生成并提交到仓库，便于全新检出时直接检查类型；构建导致声明变化时，也应一起提交。IDE 应使用 `frontend/node_modules/typescript` 中的 TypeScript 5.9；文档工程保持独立依赖。

Vite 配置显式保留升级前的 JavaScript 编译目标，并继续使用 esbuild 压缩 CSS；Tailwind 保持 3.4。这些设置用于降低构建工具升级对现有 WebView 的影响，不代表所有旧系统都已通过兼容性验证。

客户端交互需要人工验证：在支持的 Windows、macOS 和 Linux 系统中启动应用，检查首页与设置页、语言和主题切换、证书授权、插件管理、资源列表、下载任务，以及图片、音视频、HLS / FLV 预览。开发时还需检查 `wails dev` 的页面更新和 Go 绑定调用。

### Windows 安装脚本

安装目录校验与卸载保护位于 `build/windows/installer/safety.nsh`，由 `project.nsi` 引用。不要修改 Wails 构建时会重新生成的 `wails_tools.nsh` 来实现这些保护。

编译 Fixed WebView2 安装包时，NSIS 自动调用 `go run ./runtimefiles`，使用现有 Go 工具链将运行库的准确文件清单嵌入卸载程序。普通版不会调用生成器；用户安装或卸载时也不会运行它。生成器位于 `build/windows/installer/runtimefiles/`，只使用 Go 标准库，可通过 `-DRESD_GO=<Go可执行文件路径>` 指定构建工具。

卸载前会交叉检查注册表中的安装路径与安装标记，并拒绝目录链接。删除范围限定为安装包内置的文件清单，非空目录、用户数据和未知运行库残留会保留；普通版升级也不会整目录清理旧版 Fixed WebView2。旧版卸载程序不会自动获得这些保护，需要通过新安装包覆盖更新。

修改卸载逻辑后，请静态编译普通版 amd64、arm64 和 Fixed WebView2 分支，并在可还原快照的 Windows 虚拟机中手动验证：正常及静默卸载保留额外文件；移动卸载程序、删除安装标记或注册表记录时不删除任何文件；系统目录、目录联接和符号链接受到拒绝；程序占用文件时保留重试所需记录。使用临时测试目录和无价值占位文件，不要在真实系统目录或用户资料目录复现误删。
