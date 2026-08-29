# 发布插件到扩展商店

本文面向插件作者和项目维护者。普通用户安装、更新或卸载插件请阅读[插件管理](plugin-management.md)。

扩展商店使用 GitHub 仓库和 Release 发布插件，官网只提供索引。开发者不需要向中心插件仓库提交代码；符合约定的公开仓库会被定期发现。

> `res-downloader-ext` Topic 表示作者主动加入插件生态，不代表官方审核或安全背书。

## 发布要求与建议

一个可安装插件需要满足：

1. GitHub 仓库公开、未归档、不是 Fork，并添加 `res-downloader-ext` Topic；
2. 一个仓库只发布一个插件，`plugin.json` 位于仓库根目录；
3. JavaScript、WASM、页面脚本等运行文件已经提交，不能依赖未提交的本地构建产物；
4. 建议至少包含一个脱敏 fixture，并在发布前通过 `plugin lint` 和 `plugin replay`；
5. 创建非草稿、非预发布的 GitHub Release，例如 Tag 为 `v1.2.0`；
6. `plugin.json.version` 为 `1.2.0`，与去掉可选 `v` 前缀后的 Tag 完全一致。

`putyy` 账号下的仓库在商店中标记为“官方”，其他仓库标记为“社区”。社区插件 ID 不能以 `builtin.` 或 `official.` 开头，大小写变体也不允许；只有官方商店条目和宿主内嵌插件可以使用 `official.` 前缀。

建议在仓库中提交固定路径 `dist/plugin.zip`，用于提升国内用户的下载速度。商店也会保留 GitHub Release Tag 对应的源码包作为回退，不需要额外上传 Release Asset。

## 版本与 Tag

商店以最新正式 Release 为准。`plugin.json.version` 必须与去掉可选 `v` 前缀后的 Tag 完全一致。

发布任何内容变化都应：

1. 更新 Manifest 版本；
2. 提交全部运行文件和 fixture；
3. 创建新的 Tag 和 Release；
4. 等待商店索引刷新。

不要覆盖或移动已有 Tag，也不要在 Release 后补交 WASM、`dist/plugin.zip` 或其他构建文件。jsDelivr 和 GitHub 源码包都以 Tag 对应的提交为准。

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

在 `res-downloader` 源码根目录运行以下命令，为插件目录生成固定路径的加速包：

```bash
go run main.go plugin pack <插件目录>
```

`tests/` 用于插件开发者自己的 JavaScript 测试，测试文件建议命名为 `*.test.js`。打包器会排除 `.git/`、`dist/`、`tests/` 和输出文件自身；`fixtures/` 仍会保留。`plugin.zip` 中的 `plugin.json` 应位于 ZIP 根目录。创建 Tag 前必须提交最新的 `dist/plugin.zip`。

商店会优先使用 `dist/plugin.zip` 加速安装；该文件不可用时会自动使用 GitHub 源码包。

安装器会忽略常见的 macOS 压缩元数据，并拒绝符号链接、重复文件、越界路径、超限文件和无效 Manifest。不要在压缩包中放入账号、抓取日志、Cookie、Authorization、真实用户 fixture 或无关的大文件。

## 安装校验

扩展商店不发布内容 SHA-256。客户端仍会验证 ZIP 结构、解压大小、Manifest、插件 ID、版本、权限、入口文件和运行时代码。下载包中的 Manifest 必须与索引从同一 Tag 读取的 `plugin.json` 一致。

本地 ZIP 安装仍可显示其规范化内容摘要，用于用户自行识别文件；商店只判断本地包的插件 ID 和版本是否与条目一致，不表示文件内容完全相同。

## 其他分发方式

开发者可以把 GitHub 自动生成的同一 Tag ZIP 分享到网盘或其他渠道。用户通过“插件管理 → 从压缩包安装”选择文件后，应用会先展示：

- 插件 ID、名称、作者、版本和 API 版本；
- 申请访问的域名和 capability；
- 本地 ZIP 的规范化内容摘要；
- 插件 ID 和版本是否与本地缓存的商店条目一致；
- 是否会替换当前已安装版本。

确认后才会安装。替换外部插件会保留现有插件设置，并保留上一个版本用于一次回滚。官方商店版本可以更新同 ID 的内嵌 JavaScript 插件；社区或本地 ZIP 不能覆盖 `official.` 插件。

## 发布检查清单

- `plugin.json` 位于仓库根目录，ID 与仓库长期对应，且不使用保留前缀；
- 版本符合语义化版本，并与 Release Tag 一致；
- 权限和域名已经缩减到实际需要的范围；
- README 说明支持站点、主要功能、必要设置和已知限制；
- fixture、日志和示例已脱敏；
- 最终提交通过 `go run main.go plugin lint <插件目录>` 与所有 fixture 回放；
- `go run main.go plugin pack <插件目录>` 成功，且生成文件已经在创建 Tag 前提交；
- 从 `dist/plugin.zip` 和 GitHub 自动生成的源码 ZIP 安装检查成功；
- 新版本使用新 Tag，没有移动或覆盖旧 Tag；
