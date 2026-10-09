---
description: 了解 res-downloader 插件扩展商店的发布要求，包括 GitHub Topic、版本与 Tag、Release、安装包结构及安装校验。
---

# 扩展商店

本文面向插件作者和项目维护者。普通用户安装、更新或卸载插件请阅读[插件管理](../guide/plugin-management.md)。

扩展商店使用 GitHub 仓库和 Release 发布插件，官网提供插件索引。开发者在自己的公开仓库中发布插件即可，商店会定期查找符合发布要求的仓库并更新索引。

> `res-downloader-ext` Topic 表示作者主动加入插件生态，不代表官方审核或安全背书。

## 发布要求与建议

要被扩展商店收录并供用户安装，需要满足：

1. GitHub 仓库公开、未归档、不是 Fork，并添加 `res-downloader-ext` Topic；
2. 一个仓库只发布一个插件，`plugin.json` 位于仓库根目录；
3. JavaScript、WASM、页面脚本等运行文件已经提交，不能依赖未提交的本地构建产物；
4. 创建非草稿、非预发布的 GitHub Release，例如 Tag 为 `v1.0.0`；
5. `plugin.json.version` 为 `1.0.0`，与去掉可选 `v` 前缀后的 Tag 完全一致。

fixture 不是商店收录的必要条件，但建议准备脱敏数据验证插件行为，详见[发布自检](#建议的发布自检清单)。

## 插件来源与保留 ID

“官方”和“社区”是应用使用的来源标记，判定方式取决于安装入口：

- **扩展商店**：根据索引中的仓库归属判定，`putyy` 账号下的仓库标记为“官方”，其他仓库标记为“社区”。
- **本地 ZIP**：根据 `plugin.json` 中的 `author.url` 判定。地址指向 GitHub 的 `putyy` 账号时标记为“官方”，其他包标记为“社区”。此字段应填写插件仓库地址。

本地 ZIP 的来源标记取自包内填写的作者地址，不能证明文件来自该仓库或经过官方审核。

插件 ID 的保留前缀忽略大小写：

- `builtin.`：保留给应用内置功能，任何插件包都不能使用。
- `official.`：仅限应用内嵌的官方插件，以及按上述规则标记为“官方”的商店包或本地 ZIP。

`plugin lint`、`plugin replay` 和 `plugin pack` 允许处理 `official.` 插件，但不授予官方身份；安装时仍会校验来源与 ID。

## 版本与 Tag

商店以最新正式 Release 为准，版本与 Tag 须符合上述[发布要求](#发布要求与建议)。

发布插件内容的更新时，应按以下顺序操作：

1. 更新 Manifest 版本；
2. 提交全部运行文件及提供的 fixture；若提供 `dist/plugin.zip`，需重新打包后一并提交；
3. 创建新的 Tag 和 Release；
4. 等待商店索引刷新。

每个版本使用新 Tag，不要移动已有 Tag。下载包取自 Tag 指向的提交，发布后补交的 WASM、加速包或其他构建文件不会进入该版本。

## 使用 GitHub Actions 自动发布

官方插件通过各自的 `.github/workflows/release.yml` 调用主仓库 `master` 上的共享发布流程。

按上述步骤更新版本、打包并提交源码与 `dist/plugin.zip` 后，推送对应的 `v主版本.次版本.补丁版本` 标签。Actions 校验版本与 ZIP 内容后，自动创建正式 Release 并附上安装包。

## 安装包结构与加速

建议提供 `dist/plugin.zip` 加速包，提升国内用户的下载速度。商店优先下载该包，不可用时回退到对应 Tag 的 GitHub 源码 ZIP。加速包可选，无需额外上传 Release 附件。

推荐仓库结构：

```text
仓库根目录/
├── plugin.json
├── main.js
├── fixtures/
├── tests/
├── decrypt.wasm
└── dist/
    └── plugin.zip
```

在 `res-downloader` 源码根目录运行以下命令，生成 `<插件目录>/dist/plugin.zip`：

```bash
go run main.go plugin pack <插件目录>
```

`plugin.zip` 中的 `plugin.json` 应位于 ZIP 根目录。测试目录约定和打包排除规则见[插件目录与加载](plugins.md#插件目录与加载)，`fixtures/` 会保留在包中。

不要在压缩包中放入账号信息、抓取日志、Cookie、Authorization、含真实用户数据的 fixture 或无关的大文件。

## 安装校验

安装时，应用会检查 ZIP 结构、解压大小、`plugin.json`、插件 ID、版本、权限、入口文件和运行代码。下载包中的 `plugin.json` 必须与商店索引从同一 Tag 读取的内容一致。

安装工具会忽略常见的 macOS 压缩元数据，并拒绝符号链接、重复文件、解压路径超出插件目录的文件、大小超限的文件和无效的 `plugin.json`。

安装本地 ZIP 时，应用会显示内容摘要（SHA-256），用于识别插件包。该摘要根据解压后的文件路径和内容计算，不是 ZIP 文件本身的哈希值。

商店索引不提供内容摘要，因此应用只比较本地包与商店条目的插件 ID 和版本。两项信息一致，并不能证明两个包的文件内容完全相同。

## 其他分发方式

开发者可将对应 Tag 的 GitHub 源码 ZIP 分享到网盘等渠道。用户通过“插件管理 → 从压缩包安装”导入，确认插件信息、权限、内容摘要及替换提示后安装。

更新或替换外部插件会保留现有设置和上一个版本，方便回滚。

更新应用内嵌的 JavaScript 插件时，还需满足以下条件：

- 安装包按[来源规则](#插件来源与保留-id)标记为“官方”。社区包不能替换内嵌插件。
- 安装包与内嵌插件使用相同的 ID。

所有更新或替换操作都需要通过版本和权限检查。

## 建议的发布自检清单

- 仓库符合发布规则，ID 与仓库长期对应，版本符合语义化版本及 Tag 要求，权限和域名限于实际需要；
- README 说明支持站点、主要功能、必要设置和已知限制，fixture、日志和示例已脱敏；
- 最终提交通过 `plugin lint` 和所有 fixture 的 `plugin replay`；
- 从对应 Tag 的 GitHub 源码 ZIP 安装检查成功；若提供加速包，也需验证其内容与 Tag 一致且可正常安装。
