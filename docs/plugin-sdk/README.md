# 插件 SDK v1

本目录提供插件开发时使用的公开协议辅助文件，会随 `docs/` 一起发布到文档站点。它们用于编辑器提示和外部工具集成，不参与 res-downloader 的运行时加载或命令行校验。

## 文件

- [Manifest Schema](/plugin-sdk/plugin-v1.schema.json ':ignore')：`plugin.json` 的 JSON Schema，覆盖权限、匹配规则、页面脚本、资源类型、设置、声明式提取器、WASM 处理器和本地操作。
- [`plugin-v1.d.ts`](/plugin-sdk/plugin-v1.d.ts ':ignore')：JavaScript 插件 API 的 TypeScript 声明，覆盖 Observation、运行时 API、资源、刷新结果和下载计划。

在支持 JSON Schema 的编辑器中，可以把 `plugin.json` 与 Schema 地址关联：

```text
https://res.putyy.com/plugin-sdk/plugin-v1.schema.json
```

JavaScript 项目可以把 `plugin-v1.d.ts` 下载到开发目录，并通过编辑器配置或 `/// <reference path="./plugin-v1.d.ts" />` 启用类型提示。运行时仍执行普通 JavaScript，不要求使用 TypeScript 构建。

应用实际加载插件时，以 Go 后端的协议模型和校验逻辑为准。提交插件前仍应在 `res-downloader` 源码根目录运行：

```text
go run main.go plugin lint <插件目录>
go run main.go plugin replay <插件目录> <fixture 文件>
```

完整的 Manifest、权限、钩子、资源模型和 WASM ABI 说明见 [插件开发](../plugins.md)。

新增或调整插件协议时，应同步更新 Schema、类型声明、示例、fixture 和插件开发文档。

JSON Schema 负责字段提示，无法证明插件包的分发来源。本地和社区插件的 `builtin.` / `official.` 保留前缀规则由 `plugin lint` 和安装器执行；只有宿主内嵌插件以及固定官方账号发布、由商店索引确认来源的插件可以使用 `official.`。
