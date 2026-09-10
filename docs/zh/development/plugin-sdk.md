---
description: 下载 res-downloader 插件 SDK v1 的 Manifest JSON Schema 与 TypeScript 类型声明，为插件开发配置编辑器提示。
---

# 插件 SDK v1

插件开发使用的公开协议辅助文件统一维护在 `docs/public/plugin-sdk/`，构建时原样发布，各语言文档共用。它们用于编辑器提示和外部工具集成，不参与 res-downloader 的运行时加载或命令行校验。

## 文件

- [Manifest Schema](/plugin-sdk/plugin-v1.schema.json)：`plugin.json` 的 JSON Schema，覆盖权限、匹配规则、页面脚本、资源类型、设置、声明式提取器、WASM 处理器和资源操作。
- [`plugin-v1.d.ts`](/plugin-sdk/plugin-v1.d.ts)：JavaScript 插件 API 的 TypeScript 声明，覆盖 Observation、运行时 API、资源、页面命令、刷新结果和下载计划。

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

完整的 Manifest、权限、钩子、资源模型和 WASM ABI 说明见 [插件开发](plugins.md)。

新增或调整插件协议时，应同步更新 Schema、类型声明、示例、fixture 和插件开发文档。

JSON Schema 负责字段提示，无法证明插件包的分发来源。`plugin lint`、`plugin replay` 和 `plugin pack` 也不授予官方身份；它们允许开发阶段处理 `official.` 插件。安装时的来源判定、`builtin.` / `official.` 保留前缀限制见[插件来源与保留 ID](extension-store.md#插件来源与保留-id)。
