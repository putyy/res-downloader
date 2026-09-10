---
description: 了解 res-downloader 插件扩展商店的发布要求，包括 GitHub Topic、版本与 Tag、Release、安装包结构及安装校验。
---

# 发布插件到扩展商店

本文面向插件作者和项目维护者。普通用户安装、更新或卸载插件请阅读[插件管理](../guide/plugin-management.md)。

扩展商店使用 GitHub 仓库和 Release 发布插件，官网提供插件索引。开发者在自己的公开仓库中发布插件即可，商店会定期查找符合发布要求的仓库并更新索引。

> `res-downloader-ext` Topic 表示作者主动加入插件生态，不代表官方审核或安全背书。

## 发布要求与建议

要被扩展商店收录并供用户安装，需要满足：

1. GitHub 仓库公开、未归档、不是 Fork，并添加 `res-downloader-ext` Topic；
2. 一个仓库只发布一个插件，`plugin.json` 位于仓库根目录；
3. JavaScript、WASM、页面脚本等运行文件已经提交，不能依赖未提交的本地构建产物；
4. 创建非草稿、非预发布的 GitHub Release，例如 Tag 为 `v1.2.0`；
5. `plugin.json.version` 为 `1.2.0`，与去掉可选 `v` 前缀后的 Tag 完全一致。

发布前建议至少准备一个脱敏 fixture，并通过 `plugin lint` 和 `plugin replay`。这些检查用于帮助作者验证插件行为，fixture 不是商店收录的必要条件。

建议将加速包提交到仓库的 `dist/plugin.zip`，以提升国内用户的下载速度。商店会优先下载该包；加速包不可用时，自动下载对应 Tag 的 GitHub 源码包。不提供加速包也能发布插件，无需额外上传 Release 附件。

## 插件来源与保留 ID

“官方”和“社区”是应用使用的来源标记，判定方式取决于安装入口：

- **扩展商店**：根据索引中的仓库归属判定，`putyy` 账号下的仓库标记为“官方”，其他仓库标记为“社区”。
- **本地 ZIP**：根据 `plugin.json` 中的 `author.url` 判定。地址指向 GitHub 的 `putyy` 账号时标记为“官方”，其他包标记为“社区”。此字段应填写插件仓库地址。

本地 ZIP 的来源标记取自包内填写的作者地址，不能证明文件来自该仓库或经过官方审核。

插件 ID 有以下限制，修改字母大小写也不能绕过这些限制：

- `builtin.`：保留给应用内置功能，任何插件包都不能使用。
- `official.`：仅限应用内嵌的官方插件，以及按上述规则标记为“官方”的商店包或本地 ZIP。
- 社区插件不能使用以上两个前缀。

`plugin lint`、`plugin replay` 和 `plugin pack` 允许检查、回放和打包 `official.` 插件。这些命令不会为插件授予官方身份；安装时，应用还会检查插件来源是否允许使用该 ID。

## 版本与 Tag

商店以最新正式 Release 为准。`plugin.json.version` 必须与去掉可选 `v` 前缀后的 Tag 完全一致。

发布插件内容的更新时，应按以下顺序操作：

1. 更新 Manifest 版本；
2. 提交全部运行文件；如果提供 fixture，也一并提交；
3. 创建新的 Tag 和 Release；
4. 等待商店索引刷新。

不要将已有 Tag 改为指向其他提交。WASM、`dist/plugin.zip` 和其他构建文件必须在创建 Tag 前提交；发布后补交的文件不会进入该版本的下载包，因为 jsDelivr 和 GitHub 源码包都取自 Tag 指向的提交。

## 安装包结构与加速

商店版本要求 `plugin.json` 位于仓库根目录。推荐仓库结构：

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

`tests/` 用于存放插件的 JavaScript 测试，测试文件建议命名为 `*.test.js`。

打包时请注意：

- 打包工具会排除 `.git/`、`dist/`、`tests/` 和输出文件自身，保留 `fixtures/`。
- `plugin.zip` 中的 `plugin.json` 应位于 ZIP 根目录。
- 每次发布前都要重新打包，并在创建 Tag 前提交 `dist/plugin.zip`，确保包内运行文件和版本与该 Tag 一致。

不要在压缩包中放入账号信息、抓取日志、Cookie、Authorization、含真实用户数据的 fixture 或无关的大文件。

## 安装校验

安装时，应用会检查 ZIP 结构、解压大小、`plugin.json`、插件 ID、版本、权限、入口文件和运行代码。下载包中的 `plugin.json` 必须与商店索引从同一 Tag 读取的内容一致。

安装工具会忽略常见的 macOS 压缩元数据，并拒绝符号链接、重复文件、解压路径超出插件目录的文件、大小超限的文件和无效的 `plugin.json`。

安装本地 ZIP 时，应用会显示内容摘要（SHA-256），用于识别插件包。该摘要根据解压后的文件路径和内容计算，不是 ZIP 文件本身的哈希值。

商店索引不提供内容摘要，因此应用只比较本地包与商店条目的插件 ID 和版本。两项信息一致，并不能证明两个包的文件内容完全相同。

## 其他分发方式

开发者可以把 GitHub 为该版本 Tag 自动生成的源码 ZIP 包分享到网盘或其他渠道。用户通过“插件管理 → 从压缩包安装”选择文件后，应用会先展示：

- 插件 ID、名称、作者、版本和 API 版本；
- 申请访问的域名和插件权限；
- 本地 ZIP 的内容摘要；
- 插件 ID 和版本是否与本地缓存的商店条目一致；
- 是否会替换当前已安装版本。

用户确认后才会安装。更新或替换外部插件时，应用会保留现有插件设置和上一个版本，方便更新后回滚。

更新应用内嵌的 JavaScript 插件时，还需满足以下条件：

- 安装包按[来源规则](#插件来源与保留-id)标记为“官方”。社区包不能替换内嵌插件。
- 安装包与内嵌插件使用相同的 ID。

所有更新或替换操作都需要通过版本和权限检查。

## 建议的发布自检清单

- `plugin.json` 位于仓库根目录，ID 与仓库长期对应，且符合[保留 ID 规则](#插件来源与保留-id)；
- 版本符合语义化版本，并与 Release Tag 一致；
- 权限和域名已经缩减到实际需要的范围；
- README 说明支持站点、主要功能、必要设置和已知限制；
- fixture、日志和示例已脱敏；
- 最终提交通过 `go run main.go plugin lint <插件目录>` 与所有 fixture 回放；
- 从 GitHub 自动生成的 Tag 源码 ZIP 安装检查成功；
- 若提供加速包，`go run main.go plugin pack <插件目录>` 成功，生成的 `dist/plugin.zip` 已在创建 Tag 前提交，并完成该包的安装检查；
- 新版本使用新 Tag，没有移动或覆盖旧 Tag；
