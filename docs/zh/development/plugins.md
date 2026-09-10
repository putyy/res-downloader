---
description: res-downloader 插件开发指南，涵盖 Manifest、权限、JavaScript API、页面脚本、资源模型、下载计划、WASM 和打包发布。
---

# 插件开发

本文面向插件开发者，说明插件目录、Manifest、权限、运行时 API、资源模型、下载计划、WASM 处理器、调试测试和发布流程。普通用户安装或管理插件请阅读[插件管理](../guide/plugin-management.md)。

res-downloader 的插件协议通过版本号区分。应用将捕获到的请求和响应信息传给插件，插件按约定的数据格式返回识别出的资源。插件不能直接访问 Go 对象、文件系统、Shell，也不能自行发起任意网络请求。

项目仓库中的 `plugins/` 是本地插件开发工作区，方便克隆和调试独立插件仓库。该目录下的插件源码默认不会提交到宿主仓库，也不会被宿主自动加载或打进应用；示例插件位于 `examples/plugins/`，随应用发布的内嵌插件位于 `internal/plugin/bundled/`。

开发插件前建议依次阅读“选择插件类型 → 快速开始 → Manifest → 权限 → 资源模型 → JavaScript 钩子或声明式插件 → 校验和离线回放”。页面脚本、下载计划和 WASM 属于按需使用的高级能力。

当前支持：

- `declarative`：用 JSON/YAML 和受限 JSON Path 从单个 JSON 响应提取单轨资源。
- `javascript`：处理复杂 JSON、跨请求关联、自定义下载计划和资源刷新。
- 插件自带 WASM：在下载输入或输出阶段执行私有解密/转换算法。
- 资源操作：处理本地文件，或把插件自定义参数发送给匹配的页面脚本。
- 宿主下载能力：普通 HTTP、HLS，以及配置 FFmpeg 后的音视频合并、转封装、音频提取和直播录制。

## 选择插件类型

| 场景 | 建议 |
| --- | --- |
| 一个 JSON 响应直接包含标题和下载地址 | `declarative` |
| 需要条件判断、遍历复杂对象或多个清晰度 | `javascript` |
| 视频、音频来自不同请求，需要按作品 ID 合并 | `javascript` + 关联接口 |
| 链接会过期，需要下载前重新解析 | `javascript` + `refreshResource` |
| 下载后需要私有解密或字节转换 | JavaScript + 插件 WASM |
| 需要从页面运行环境取得播放器数据 | JavaScript + 页面脚本；仅在普通观察接口无法完成时使用 |

优先选择最简单的实现。能通过响应 JSON 提取时，不要申请页面脚本、响应修改或 FFmpeg 高级权限。

## 快速开始

### 1. 复制示例

从仓库根目录执行：

```bash
cp -R examples/plugins/javascript-basic ./plugins/com.example.my-plugin
```

简单 JSON API 可以复制 `declarative-basic`；WASM 示例位于 `wasm-xor`。也可以通过项目 CLI 交互创建 JavaScript 脚手架：

```bash
go run main.go plugin create
```

在各项提示中直接回车，会使用默认父目录 `./plugins`，并将插件 ID 和显示名称设为 `com.example.my-plugin`。最终创建的目录为 `./plugins/com.example.my-plugin`。也可以直接指定参数：

```bash
go run main.go plugin create ./plugins/com.example.my-plugin com.example.my-plugin "Example Video"
```

脚手架包含 `plugin.json`、`main.js`、`README.md`、`.gitignore` 和空的 `fixtures/` 目录。

插件默认包含“启用日志”设置 `enableLog`，类型为 `boolean`、默认值为 `false`，手动创建插件时也应声明该设置，并提供中英文本地化名称。

### 2. 修改 Manifest 和入口

至少修改插件 `id`、名称、版本、允许访问的域名和匹配规则。插件 ID 建议使用反向域名格式，并保持长期稳定，例如 `com.example.video`。社区插件不能使用宿主保留的 `builtin.` 或 `official.` 前缀，大小写变体同样会被拒绝。官方插件及本地 ZIP 的判定方式见[插件来源与保留 ID](extension-store.md#插件来源与保留-id)。

JavaScript 插件可在 `main.js` 中实现 `onObservation`，先完成“匹配一个响应并输出一个资源”的最小流程，再逐步加入关联、刷新或下载处理。

### 3. 准备脱敏 fixture

将一次代表性的请求/响应保存为 fixture，并删除 Cookie、Authorization、账号、签名和真实内容。fixture 应保留插件判断所必需的最小字段。

### 4. 静态校验和离线回放

```bash
go run main.go plugin lint ./plugins/com.example.my-plugin
go run main.go plugin replay ./plugins/com.example.my-plugin ./plugins/com.example.my-plugin/fixtures/video.json
```

`lint` 校验目录、Manifest、入口文件和权限关系；`replay` 在不启动代理的情况下执行 fixture。

### 5. 打包并安装

```bash
go run main.go plugin pack ./plugins/com.example.my-plugin
```

默认生成 `<插件目录>/dist/plugin.zip`。如需保存到其他位置，可以在命令末尾指定输出路径。

打包时会排除以下内容：

- 目录：`.git/`、`.idea/`、`.vscode/`、`dist/`、`tests/`；
- 文件：输出的 ZIP 本身，以及 `.gitignore`、`.DS_Store`、`README.md`、`LICENSE`。

这些是打包器自身的规则，不读取 `.gitignore`。是否将 `dist/plugin.zip` 提交到插件仓库，由 Git 的规则决定。

测试时，在应用“插件管理”中选择生成的 ZIP 安装。开发期间也可以把插件目录放入 res-downloader 数据目录的 `plugins` 子目录，再点击“重新加载”。

本地 ZIP 的来源标记、保留 ID 和替换条件见[插件来源与保留 ID](extension-store.md#插件来源与保留-id)及[其他分发方式](extension-store.md#其他分发方式)。

开发者自己的 JavaScript 测试统一放在仓库根目录的 `tests/`，测试文件建议命名为 `*.test.js`。该目录不会进入安装包；用于 `plugin replay` 的脱敏数据仍放在 `fixtures/`。

## 开发时必须注意

- **最小权限**：只声明实际需要的域名和 capability，避免使用 `*` 域名。
- **最小 Body**：缩小 `match` 范围，不读取 Body 时明确设置 `readBody: false`，并为 `bodyLimit` 选择够用的最小值。
- **敏感数据**：不要记录或上报 Cookie、Authorization、账号、管理员密码和长期凭据；不需要持久化的 Header 使用 `nonPersistentHeaders`。
- **可靠关联**：多请求资源必须使用作品 ID、轨道 ID 或明确别名关联，不能按请求到达顺序猜测。
- **失败可理解**：地址过期时返回明确的刷新状态；数据不完整时保持资源为不可下载状态，不要生成看似成功的错误文件。
- **页面消息不可信**：页面脚本与网站代码处于同一环境，所有桥接消息都要校验类型、长度和业务字段。
- **合法使用**：插件不得用于绕过访问控制或未获授权的 DRM；fixture 和日志不得包含他人的私密数据。

## 插件目录与加载

每个插件使用一个独立目录：

```text
plugins/
└── com.example.video/
    ├── plugin.json
    ├── main.js
    ├── decrypt.wasm
    ├── fixtures/
    │   └── video.json
    └── tests/
        └── main.test.js
```

Manifest 可使用 `plugin.json`、`plugin.yaml` 或 `plugin.yml`。ZIP 中的 Manifest 必须位于压缩包根目录或唯一顶层文件夹内。入口和资源文件必须使用插件目录内的相对路径；符号链接、越界路径和超限文件会被拒绝。

插件在应用启动时加载一次，开发期间可点击“重新加载”；启停插件和保存插件设置也会刷新当前插件集合。目录名建议与 Manifest 的 `id` 相同。

官方插件随应用升级覆盖。需要二次开发时应复制为新的插件 ID。

## Manifest

Manifest 文件可使用 `plugin.json`、`plugin.yaml` 或 `plugin.yml`：

使用 JSON 时，可以通过 [Manifest Schema 和编辑器类型声明](plugin-sdk.md)获得字段提示。完成修改后，运行 `plugin lint` 检查配置和入口文件；安装时仍会按插件来源进行校验。

```yaml
id: com.example.video
name: Example Video
author:
  name: Example Developer
  url: https://example.com
version: 1.0.0
apiVersion: 1
runtime: javascript
entry: main.js
priority: 100

locales:
  zh:
    name: 示例视频插件
    description: 从示例站点发现视频资源。
  en:
    name: Example Video
    description: Discovers videos from the example site.

permissions:
  domains: [api.example.com, "*.cdn.example.com"]
  capabilities:
    - observe-response
    - read-response-body
    - emit-resource
  bodyLimit: 4194304

match:
  - stage: response
    host: api.example.com
    path: /api/videos/*
    readBody: true
    contentTypes: [application/json, "application/json;*"]

resourceKinds:
  - id: media.video
    icon: video
    color: "#2080f0"
    locales:
      zh: {name: 视频}
      en: {name: Video}

settingsSchema:
  type: object
  properties:
    enableLog:
      type: boolean
      default: false
      x-locales:
        zh: {name: 启用日志, description: 记录插件调试日志，排查问题时开启。}
        en: {name: Enable logging, description: Record plugin debug logs for troubleshooting.}
    minimumSize:
      type: number
      default: 0
      x-locales:
        zh: {name: 最小大小, description: 小于该字节数的资源不会被插件输出。}
        en: {name: Minimum size, description: Resources smaller than this byte count are ignored.}
```

主要字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `id` | 是 | 最长 64 个字符，只能包含字母、数字、点、短横线和下划线；保留前缀忽略大小写，使用条件见[插件来源与保留 ID](extension-store.md#插件来源与保留-id) |
| `name` | 是 | 无本地化文案时显示的插件名称 |
| `author` | 否 | `name` 显示在插件卡片；`url` 只能是 HTTP/HTTPS 地址 |
| `version` | 是 | 语义化版本，例如 `1.2.0` |
| `apiVersion` | 是 | 当前必须为 `1` |
| `runtime` | 是 | `javascript` 或 `declarative` |
| `entry` | JavaScript 必填 | 插件目录内的 `.js` 入口，最大 1 MiB |
| `priority` | 否 | 数值越大越先执行；同一个请求可以由多个插件处理 |
| `permissions` | 是 | 允许访问的域名、能力和 Body 上限 |
| `match` | 是 | 请求或响应匹配规则；空数组表示匹配权限域名内的所有允许阶段 |
| `locales` | 否 | 插件名称和描述的多语言文案 |
| `resourceKinds` | 否 | 插件提供的站点细分类和展示文案 |
| `settingsSchema` | 否 | 插件设置的结构、默认值和表单提示 |
| `pageScripts` | 否 | 由宿主注入匹配 HTML 页面的脚本 |
| `extractors` | 声明式必填 | 从 JSON Body 提取资源的规则 |
| `processors` | 否 | 插件自带的 WASM 处理器 |
| `actions` | 否 | 由宿主渲染的本地文件处理或页面命令操作 |
| `requires` | 否 | 可选宿主工具要求，例如 `ffmpeg: ">=6.0"` |

`match` 中的 `host`、`path` 和完整 `url` 支持 `*` 通配符；`method` 忽略大小写。`contentTypes` 匹配响应 Content-Type，`readBody` 决定命中该规则时是否需要 Body。只依赖 URL 或响应头时应明确设置 `readBody: false`。

`resourceKinds` 会出现在首页抓取类型中，用户既可以按稳定的 `primaryType` 宽泛筛选，也可以按插件的 `kind` 精确筛选。

`settingsSchema` 支持 `string`、`number`、`integer`、`boolean`、`object`、`array` 和 `enum` 的基础校验。基础类型会在插件管理页生成表单；复杂结构仍可使用高级 JSON 编辑。

当前支持中文（`zh`）和英文（`en`）。插件的 `locales`、设置的 `x-locales` 和枚举的 `x-enumLabels` 统一使用这两个语言键，并提供对应文案。

设置属性可以使用 `x-locales` 提供本地化名称和说明；枚举可以使用 `x-enumLabels` 提供选项文案：

```yaml
quality:
  type: string
  enum: [default, high, low]
  default: default
  x-locales:
    zh: {name: 下载画质, description: 选择插件请求资源时使用的画质。}
    en: {name: Download quality, description: Select the quality requested by the plugin.}
  x-enumLabels:
    zh: {default: 默认, high: 高清, low: 低清}
    en: {default: Default, high: High, low: Low}
```

## 权限

| capability | 作用 | 前置条件或注意事项 |
| --- | --- | --- |
| `observe-request` | 接收请求 URL、Header 等元数据 | 仅限 `permissions.domains` 和 `match` 命中范围 |
| `read-request-body` | 读取匹配请求的 Body | 需要 `observe-request`，仅在确有必要时申请 |
| `intercept-request` | 返回本地合成响应 | 需要 `observe-request`，会改变页面行为 |
| `observe-response` | 接收响应状态、Header 等元数据 | 常规资源发现的基础权限 |
| `read-response-body` | 读取匹配响应的 Body | 需要 `observe-response`，受 `bodyLimit` 限制 |
| `modify-response` | 修改匹配响应 | 需要 `observe-response`，截断 Body 不可修改 |
| `emit-resource` | 上报资源 | 输出仍会经过宿主校验 |
| `process-download` | 调用本插件声明的 WASM 处理器 | 不能选择任意本地文件或模块 |
| `media.basic` | 使用 mux、remux、音频提取等受控媒体操作 | 需要用户配置兼容的 FFmpeg |
| `media.ffmpeg` | 使用 FFmpeg 参数数组高级接口 | 不经过 Shell；需要 FFmpeg |
| `media.ffmpeg.network` | 允许 FFmpeg 读取插件提供的 HLS/直播地址 | 下载地址必须是合法的 HTTP/HTTPS URL；这是敏感权限 |
| `inject-page-script` | 在匹配 HTML 页面中注入脚本 | 目标域名必须被 TLS 拦截且允许安全注入 |
| `page-bridge` | 页面脚本与插件运行时交换 JSON 消息，或接收用户触发的 `page-command` | 需要 `inject-page-script` |
| `capture-response-body` | 缓存浏览器实际读取的 Range 响应，或接收页面脚本捕获的媒体分片 | 需要 `observe-response`；页面分片还需要 `inject-page-script` 和 `page-bridge` |
| `enqueue-download` | 页面消息上报资源后自动创建下载任务 | 需要 `page-bridge` 和 `emit-resource`；属于会写入下载目录的敏感权限 |

Body 只有在域名、规则和读取权限同时满足时才会进入插件。超过 `bodyLimit` 后快照会标记 `truncated`，截断响应不能被插件修改。

每次 JavaScript 钩子调用都在独立的运行环境中执行。主要限制如下：

| 项目 | 限制 |
| --- | --- |
| 脚本大小 | 最大 1 MiB |
| 单次执行时间 | 最多 5 秒，包含运行环境初始化、钩子执行和返回结果的转换 |
| 含排队的总时间 | 最多 10 秒，包含等待其他调用结束的时间 |
| 并发调用数 | 每个插件最多同时执行 4 次 |

请求取消时，应用也会中断对应的 JavaScript 执行。已经超时但尚未退出的调用，仍计入并发数量，直到它实际结束。应用会记录耗时较长的调用，并在连续失败时暂停插件执行。

插件运行环境不提供 Node.js、`fetch`、文件系统或系统命令 API，也不能等待 Promise。需要在多个请求之间保留关联信息时，使用应用提供的关联接口；JavaScript 全局变量不会在不同调用之间保留。

## 页面脚本和双向消息桥

JavaScript 插件通过 `pageScripts` 声明要注入网页的脚本。应用的 MITM 代理负责注入，目标域名需要开启 TLS 拦截。

页面脚本注入不经过 `onObservation`，不受该钩子的 `bodyLimit` 和 5 秒执行时间限制。注入时机目前只支持 `document-start`；`frames` 可设为 `top`（默认，仅顶层页面）或 `all`（包括子框架）。

```json
{
  "permissions": {
    "domains": ["www.example.com"],
    "capabilities": ["inject-page-script", "page-bridge"]
  },
  "pageScripts": [{
    "id": "runtime-hook",
    "entry": "page/inject.js",
    "match": [{"host": "www.example.com", "path": "/watch*"}],
    "runAt": "document-start",
    "frames": "top",
    "bridge": true
  }]
}
```

每个入口必须是插件目录内的普通 `.js` 文件，最大 256 KiB。每个插件最多声明 8 个页面脚本。

应用只检查 HTML 开头的 256 KiB，并优先使用页面已有的 CSP nonce。只有响应是未压缩的 `text/html`、能找到 `<head>`，且页面 CSP 允许内联脚本时才会注入。条件不满足时保留原响应，不会删除或放宽网站 CSP。

页面入口运行在异步函数中，并获得局部 `pageApi`：

```javascript
pageApi.onMessage(function (message) {
  if (message.type === "probe") runProbe(message.url)
})

var result = await pageApi.send({type: "player-ready", data: collectPlayerData()})
```

声明了 `capture-response-body` 后，页面脚本可以使用当前页面的会话令牌，将二进制媒体分片写入应用的捕获缓存（Capture Store）：

```javascript
await pageApi.capture.start("video:123:video")
await pageApi.capture.write("video:123:video", arrayBufferOrTypedArray)
await pageApi.capture.complete("video:123:video")
// 取消或失败时：await pageApi.capture.abort("video:123:video")
```

这四个接口的作用分别是：

- `start`：开始捕获；已有同名缓存时会先清空。
- `write`：追加数据，按各次调用完成的顺序写入。
- `complete`：标记捕获完成，之后 `capture-file` 才能读取。
- `abort`：取消捕获，立即删除尚未完成的缓存。

页面脚本负责按播放顺序排列分片并去重，应用不解析网站自己的流媒体协议。单个分片最大 32 MiB；每个页面会话最多同时使用 4 个捕获键，累计写入最多 16 GiB。

写入时会检查页面 Origin、会话令牌和插件权限。页面只能访问当前插件允许的捕获缓存，不能借此访问任意文件或其他插件的缓存。

启用桥后，页面到插件使用同源 POST，插件到页面使用 SSE。内部地址由代理直接响应，不会发往目标网站。页面每次加载获得独立 `pageSessionId`；页面关闭、插件重载、禁用或卸载后会话失效。

插件使用同步顶层钩子处理页面消息：

```javascript
function onPageMessage(message, context, api) {
  if (message.type !== "player-ready") return {ok: false, error: "unsupported message"}

  api.page.send(context.pageSessionId, {type: "probe", url: message.data.url})
  api.upsert(/* ResourceCandidate */)
  return {ok: true, data: {accepted: true}}
}
```

声明了 `enqueue-download` 的插件可以在页面消息结果中返回 `autoDownload: true`。宿主只会对同一结果中通过校验、成功发布且具有稳定 `groupKey` 的资源创建任务；单次最多 4 个。未声明权限、资源不完整或下载计划无效时不会入队：

```javascript
return {
  ok: true,
  resources: [resource],
  autoDownload: true
}
```

`context` 提供 `pageSessionId`、`scriptId`、`pageUrl`、`origin` 和当前插件设置 `settings`。

- `settings` 只传给应用中的 `onPageMessage`，不会直接交给网页，也不会出现在 `api.page.sessions()` 的结果中。网页需要设置时，插件可在初始化消息的回复中返回必要的非敏感选项，例如是否显示按钮。
- `pageUrl` 是创建会话时的地址。单页应用切换内容后，页面脚本需要重新确认当前作品或资源，不能只依赖这个地址。
- 声明 `page-bridge` 权限后，插件还可调用 `api.page.broadcast(filter, message)` 和 `api.page.sessions(filter)`。

普通消息和回复必须是 JSON，单条最大 64 KiB。较大的媒体数据使用 `pageApi.capture.write` 传输。每个插件最多保留 32 个活动页面会话；各会话还受消息队列、连接数量和请求频率限制。

在 Manifest 中声明 `page-command` 资源操作后，用户可以从应用资源列表向指定的页面脚本发送命令。应用使用资源记录中已保存的 `action.data`，不接受前端另传的自定义参数。消息格式如下：

```ts
interface PageCommandMessage {
  protocol: 1
  type: "resource-action"
  requestId: string
  actionId: string
  resource: {id: string; groupKey?: string}
  data?: Record<string, unknown>
}
```

应用会为每条命令生成随机 `requestId`。页面返回处理结果时，需要带上同一个 `requestId`。

发送前，应用会清理已过期的空闲页面会话。接收命令的页面必须同时满足：

- 属于当前插件，并使用该操作的 `pageScript` 所指定的脚本；
- 脚本设置了 `bridge: true`；
- 页面与应用之间仍有 SSE 连接。

没有符合条件的页面、消息超过 64 KiB，或所有目标页面的消息队列都已满时，发送失败。发送成功后，本地 API 返回 `requestId`、`pageScriptId` 和 `delivered`；其中 `delivered` 是命令已加入消息队列的页面会话数。

进入队列不代表页面已经收到或执行了命令，页面仍可能断开连接。处理结果需要由页面带上 `requestId` 返回。同一命令可能发给多个页面，因此每个页面都要核对资源 ID，并用 `requestId` 防止重复执行。

页面脚本与网站代码在同一个网页 JavaScript 环境中运行，网站代码可能读取或模拟桥接请求，因此插件必须校验收到的页面消息。

默认情况下，消息桥不允许网页访问文件、Shell、数据库、下载器或其他插件。声明 `enqueue-download` 后，插件可以为本次刚上报且通过校验的资源创建下载任务，但网页仍不能指定文件路径或任意控制下载任务。

## 资源模型

插件上报的是一个逻辑资源。普通文件通常只有一个轨道；音视频分离站点可以包含 `video`、`audio`、`subtitle` 等多个轨道和多个清晰度版本。

```javascript
api.emit({
  groupKey: "video:" + payload.id,
  kind: "media.video",
  primaryType: "video",
  traits: ["multiTrack", "mergeRequired"],
  title: payload.title,
  coverUrl: payload.cover,
  tracks: [{
    id: "video-1080p",
    role: "video",
    executor: "http-file",
    url: payload.videoUrl,
    mime: "video/mp4",
    extension: ".mp4",
    quality: "1080p",
    width: 1920,
    height: 1080,
    headers: {Referer: observation.request.url},
    // 只影响写入 resources.db/tasks.db 的副本，本次会话仍可下载和预览。
    nonPersistentHeaders: ["Cookie"]
  }],
  requiredTracks: ["video"],
  capabilities: ["download", "preview", "open", "copy"],
  preview: {
    renderer: "video",
    mode: "proxy",
    mime: "video/mp4",
    trackId: "video-1080p"
  },
  metadata: {
    "example.assetId": payload.id,
    author: payload.author
  }
})
```

资源字段：

- `primaryType`：稳定主类型，仅包含 `video`、`audio`、`image`、`document`、`archive`、`collection`、`other`。
- `kind`：必填的插件细分类，不受核心枚举限制。
- `traits`：可组合特征，如 `encrypted`、`multiTrack`、`segmented`、`streaming`、`live`、`gallery`、`mergeRequired`；插件私有特征使用 `plugin.id:name`。
- `groupKey`：同一插件内用于识别同一资源的固定标识。需要合并多个请求提供的资源信息时必填。
- `parentGroupKey`：可选的父资源 `groupKey`。设置后当前资源作为子项显示在父记录的展开区域；父项下载会批量下载其所有可下载子项。
- `parentId`：核心解析并持久化的父资源 ID。插件不能自行指定；插件输出中的该字段会被核心清空，避免跨插件挂接资源。
- `dedupeKey`：可选的自定义去重键；通常让核心从 `pluginId + groupKey` 或主轨 URL 生成。
- `tracks`：需要分别下载的轨道。`id` 在当前资源内唯一，`role` 表示视频、音频等用途，`executor` 默认为 `http-file`。
- `requiredTracks`：资源进入 `ready` 状态所需的角色；缺少任一角色时为 `partial`，前端不会开放下载。
- `capabilities`：决定界面上可用的操作按钮，当前通用值为 `download`、`preview`、`open`、`copy`。
- `preview`：选择受信任的前端渲染器以及用于预览的轨道。插件不能注入前端代码。
- `coverUrl`：可选封面地址；不要把大段 Base64 图片直接放入资源。
- `technical`：可选 MIME、容器、编码和时长信息，用于展示或命名。
- `lifecycle.expiresAt`：可选的毫秒时间戳，表示链接预计过期时间。
- `metadata`：站点私有字段应使用命名空间；通用 `author` 可供文件名模板使用。
- `actions`：引用 Manifest 中声明的宿主操作，例如使用 WASM 处理本地文件，或向页面发送命令；动态参数保存在操作的 `data` 中。

资源输出还需满足以下约束：

- `groupKey` 和 `parentGroupKey` 最长 512 字节，子资源不能把自己设为父级；
- 没有轨道的合集必须提供稳定 `groupKey`；
- `track.id` 在资源内唯一，轨道 URL 必须是合法远程地址；`capture-file` 改用 `captureKey`；
- 扩展名必须以 `.` 开头且最长 20 个字符；
- `preview.trackId` 必须引用当前资源已有轨道；
- 单个资源序列化后不能超过 1 MiB；不要把完整响应 Body 放入 `metadata`。

合集父项推荐使用 `media.collection`、稳定的 `groupKey` 和 `download` 能力，本身不包含 `tracks`。图片、音频等独立输出应作为带 `parentGroupKey` 的子资源；不要把它们放进父项的 `tracks`，因为轨道表示生成单个输出所需的输入。父项删除会级联删除子项，删除单个子项不会影响同级资源。

`preview.renderer` 支持 `image`、`audio`、`video`、`pdf` 和 `text`。不同资源的预览方式如下：

- **需要 WASM 处理的资源**：预览请求只传应用中的资源 ID。后端根据 `trackId` 选择输入，再执行该输入和输出上声明的 WASM 处理器，因此也能预览需要解密的资源。
- **普通图片**：没有处理器时，优先由 WebView 直接加载，避免部分图片 CDN 拒绝 Go 发起的 TLS 连接。加载失败后，改由后端携带抓取时的请求头请求图片。
- **HLS**：主清单、媒体清单、分片、初始化段和 AES 密钥的地址会转换为带短期令牌的本地地址。后端使用轨道的请求头转发请求，源站无需额外配置 CORS。预览接口只能访问这些已登记的地址，不能作为任意 URL 的代理。

### 增量聚合和关联

`api.upsert(resource)` 与 `api.emit(resource)` 的提交行为相同。同一插件多次上报相同 `groupKey` 时，应用会将它们合并为一条资源，并按 `track.id` 更新轨道。合并过程不会被其他并发更新打断，完成后会通知界面更新。

不要按请求到达的先后顺序配对音视频。插件应先从网站接口取得作品 ID 和下载地址，再将可能指向同一资源或轨道的 URL 登记为别名：

```javascript
api.correlate.register({
  groupKey: "video:" + payload.id,
  trackId: "audio-default",
  role: "audio",
  aliases: payload.audioUrls
})

api.correlate.find(observation.request.url).forEach(function (ref) {
  api.upsert({
    groupKey: ref.groupKey,
    kind: "media.video",
    tracks: [{
      id: ref.trackId,
      role: ref.role,
      url: observation.request.url
    }]
  })
})
```

每个插件使用独立的关联表，同一个 URL 可以关联多个资源或轨道。关联记录有过期时间和数量上限。应用只对 URL 做基本规范化，不会替插件决定哪些签名参数可以删除。无法确认关联关系时，应分别上报资源，或将资源保留为不完整状态。

## JavaScript 钩子

JavaScript 运行时支持四个同步顶层全局函数，可按插件需求实现：

```javascript
function onObservation(observation, api) {
  var payload = JSON.parse(observation.response.body)
  api.log("found " + payload.id)
  api.emit(/* Resource */)
  return {decision: "continue", handled: true}
}

function createDownloadPlan(input, api) {
  api.log("生成下载计划，插件版本：" + api.pluginVersion)
  var track = input.resource.tracks[0]
  return {
    inputs: [{
      id: track.id,
      executor: track.executor || "http-file",
      url: track.url,
      headers: track.headers || {},
      extension: track.extension || "",
      processors: track.processors || []
    }],
    output: {input: track.id, extension: track.extension || ""}
  }
}
```

`observation.settings` 和 `input.options.settings` 是应用中保存的插件设置。`api.pluginVersion` 是 Manifest 版本。`decision` 可为 `continue` 或 `stop`。

四个钩子的 API 参数如下：

| 钩子 | API 参数 |
| --- | --- |
| `onObservation(observation, api)` | 完整 `PluginAPI` |
| `onPageMessage(message, context, api)` | 完整 `PluginAPI` |
| `createDownloadPlan(input, api)` | 基础 `PluginBaseAPI`，仅包含 `log` 和 `pluginVersion` |
| `refreshResource(input, api)` | 基础 `PluginBaseAPI`，仅包含 `log` 和 `pluginVersion` |

基础 API 不需要额外权限。下载计划和刷新结果通过返回值提交。每次钩子调用使用独立运行时，不能跨调用保存 API 对象。

### Observation 结构

`onObservation(observation, api)` 接收以下 JSON 结构：

```ts
interface Observation {
  stage: "request" | "response"
  request: {
    method: string
    url: string
    host: string
    path: string
    headers: Record<string, string[]>
    body?: string
    truncated?: boolean
  }
  response?: {
    statusCode: number
    headers: Record<string, string[]>
    contentType: string
    body?: string
    truncated?: boolean
  }
  settings?: Record<string, unknown>
}
```

Header 值始终是字符串数组，读取时不要假定只有一个值。请求阶段没有 `response`。只有匹配规则允许读取且插件具有对应 Body 权限时，`body` 才会出现；达到 `bodyLimit` 时 `truncated` 为 `true`。

JSON 插件在解析前应同时检查 `response`、状态码、Content-Type、Body 是否为空以及 `truncated`。截断内容不能作为完整 JSON 或完整媒体数据使用。

### `onObservation` 返回值

钩子可以通过 `api.emit` / `api.upsert` 上报资源，也可以返回：

```ts
interface PluginResult {
  decision?: "continue" | "stop"
  handled?: boolean
  resources?: ResourceCandidate[]
  patch?: {statusCode?: number; headers?: Record<string, string>; body?: string}
  syntheticResponse?: {statusCode: number; headers?: Record<string, string>; body?: string}
  captures?: Array<{key: string; mode?: "range-file"}>
  diagnostics?: string[]
}
```

- `handled: true` 表示平台插件已经识别该响应，宿主会跳过最后的通用探测器。
- `decision: "stop"` 会立即终止后续插件，仅在确实需要排他处理时使用。
- `patch` 需要 `modify-response`；`syntheticResponse` 需要 `intercept-request`。
- `diagnostics` 用于脱敏后的开发诊断，不应包含请求凭据或完整私人 URL。

### 运行时 API

下表中的 `api.log` 和 `api.pluginVersion` 在四个钩子中均可使用，其他接口仅供 `onObservation` 和 `onPageMessage` 使用。

| API | 返回值 | 说明 |
| --- | --- | --- |
| `api.emit(resource)` | `void` | 上报资源；与 `upsert` 使用相同的合并语义 |
| `api.upsert(resource)` | `void` | 推荐用于具有稳定 `groupKey` 的增量资源 |
| `api.log(message)` | `void` | 仅当当前插件设置 `enableLog` 为布尔值 `true` 时写入日志；调用方负责脱敏 |
| `api.pluginVersion` | `string` | 当前 Manifest 版本 |
| `api.correlate.register(value)` | `void` | 登记 URL 别名与逻辑资源、轨道的关联 |
| `api.correlate.find(url)` | `ResourceReference[]` | 查找当前插件登记的关联 |
| `api.page.send(sessionId, message)` | `boolean` | 向一个页面会话发送消息；需要 `page-bridge` |
| `api.page.broadcast(filter, message)` | `number` | 向匹配页面广播，返回接收会话数 |
| `api.page.sessions(filter?)` | `PageMessageContext[]` | 列出当前插件可见的页面会话 |

调用 `emit` 后，资源先进入本次调用的待处理列表。应用随后会校验字段、大小、URL、轨道、处理器和操作。插件输出的资源必须符合这些要求，否则无法正常使用。

四个钩子的 `api.log()` 都由应用根据本次调用的设置控制。设置分别来自 `observation.settings`、页面消息的 `context.settings` 或 `input.options.settings`。

- 只有 `enableLog` 为布尔值 `true` 时才写入插件日志；未配置、设为 `false` 或类型错误时均不写入。
- 插件可以直接调用 `api.log()`，无需重复判断开关。
- 插件加载失败、钩子执行异常等应用错误日志不受该开关影响。

要在插件管理页显示日志开关，需在 `settingsSchema.properties` 中声明 `enableLog`。保存设置时仍会按 Manifest 校验。日志位置见[如何查看软件日志](../guide/troubleshooting.md#如何查看软件日志)。

需要复用浏览器已经成功取得、但无法用同一 URL 再次请求的数据时，插件可以在响应钩子中返回通用捕获指令。宿主只负责缓存当前响应字节，不理解站点协议；捕获键会自动限定在当前插件内：

```javascript
return {
  decision: "continue",
  captures: [{key: "asset:" + payload.id, mode: "range-file"}],
  resources: [{
    groupKey: "asset:" + payload.id,
    kind: "media.video",
    tracks: [{
      id: "video",
      role: "video",
      executor: "capture-file",
      captureKey: "asset:" + payload.id,
      extension: ".mp4"
    }],
    requiredTracks: ["video"],
    capabilities: ["download"]
  }]
}
```

`range-file` 根据响应的 `Content-Range`、请求的 `Range` 或 URL 中的 `range` 与 `clen` 信息合并区间。浏览器未实际加载全部区间时，`capture-file` 会拒绝生成残缺文件并提示继续加载后重试。应用启动时会清理超过 24 小时的捕获缓存，单个对象最大 16 GiB；该能力不会绕过登录、CSP、代理或站点访问控制。

支持的函数为 `onObservation`、`onPageMessage`、`createDownloadPlan` 和 `refreshResource`。它们都是可选的同步顶层函数；Goja 钩子中不能等待 Promise。页面脚本运行在浏览器页面中，可以使用页面环境提供的异步 API，但仍受页面 CSP、同源策略和桥接限制。

平台插件确认自己已经接管当前响应时应返回 `handled: true`。插件管理器仍会让其他高优先级平台插件完成处理，但会跳过最后的 `builtin.generic-detector`，因此不会再生成一条通用 MIME/HLS 资源。`decision: "stop"` 则会立即终止整个后续插件链，只有确实需要排他处理时才使用。仅输出诊断、修改响应但仍希望通用探测器运行时，不要设置 `handled`。

资源 URL、Header 或签名会过期时，可实现 `refreshResource(input, api)`。它接收与 `createDownloadPlan` 相同的 `{resource, options}` 和基础 API，可用 `api.log` 记录刷新诊断，并返回：

```javascript
return {
  status: "refreshed",
  resource: updatedResource,
  message: ""
}
```

`status` 可为 `refreshed`、`authenticationRequired` 或 `recaptureRequired`。成功时返回完整的更新资源；需要重新登录或重新访问页面时返回原资源和对应状态，并可在 `message` 中提供不含敏感信息的提示。没有实现或返回 `null` 表示不支持刷新。

刷新逻辑只能依据资源中的业务 ID、页面 URL、当前设置等已有信息重新计算；插件没有 `fetch`，也不能生成登录凭据。通常应通过页面脚本或新的网络 observation 获得最新数据，再用稳定 `groupKey` 更新资源。

修改响应需要 `modify-response`：

```javascript
return {
  patch: {
    body: modifiedBody,
    headers: {"X-Plugin": "com.example.video"}
  }
}
```

请求拦截使用 `syntheticResponse`，并需要 `intercept-request`。

## 下载计划

`createDownloadPlan(input, api)` 用于生成下载计划，指定需要下载的文件或轨道、合并或解密等处理步骤，以及最终输出：

```javascript
return {
  inputs: [
    {id: "part-1", executor: "http-file", url: payload.part1},
    {id: "part-2", executor: "http-file", url: payload.part2}
  ],
  pipeline: [{
    id: "joined",
    executor: "builtin.concat",
    inputs: ["part-1", "part-2"]
  }],
  output: {input: "joined", extension: ".bin"}
}
```

应用并行下载 `inputs`，依次执行 `pipeline` 中的处理步骤，再执行输出处理器。全部处理成功后，才将临时结果保存到最终路径。应用统一管理进度、临时文件、取消操作和失败后的清理。

暂停或停止方式取决于当前阶段：

- 普通 `http-file` 下载会记录各个 Range 分片的进度，暂停后可以继续；分轨和合集任务也支持暂停后继续。
- 开始执行输入处理器、WASM 或媒体处理步骤后，不再接受暂停。
- `hls` 只支持取消，不支持暂停。
- `ffmpeg-hls` 直播录制使用“停止并保存”，保留已录制的有效内容。

可用输入执行器：

- `http-file`：普通 HTTP/HTTPS 文件，支持请求头、Range 并发和下载代理。
- `capture-file`：读取应用在抓取过程中保存的完整响应数据，不再请求远端 URL；需要 `capture-response-body`。
- `hls`：解析 HLS 主清单和媒体清单，支持相对 URL、按最高/最低带宽或带宽上限选择清晰度，以及 `EXT-X-MAP`、`BYTERANGE` 和 AES-128。可下载点播内容，或仅下载当前清单中已有的分片，不会持续跟进直播；不支持暂停。
- `ffmpeg-hls`：由用户安装的 FFmpeg 直接下载或录制网络流，支持请求头、重连和最大录制时长；需要 `media.ffmpeg.network`。直播停止后，应用会保存有效的录制文件。

输入和处理步骤的 ID 必须合法，且在同一个计划中不能重复。每个处理步骤只能引用已声明的输入或排在它前面的步骤；`output.input` 指定最终要保存的结果。`capture-file` 使用缓存，不能同时提供 URL；其他输入必须提供合法的 HTTP/HTTPS URL。

HLS 可通过 `options` 设置：

```javascript
{
  id: "stream",
  executor: "hls",
  url: payload.m3u8,
  extension: ".ts",
  options: {
    variant: "lowest",
    maxBandwidth: 2500000,
    requireEndList: true
  }
}
```

处理步骤所需权限：

| executor | 所需权限 |
| --- | --- |
| `builtin.concat` | 无额外权限 |
| `builtin.media.mux` | `media.basic` |
| `builtin.media.remux` | `media.basic` |
| `builtin.media.extract_audio` | `media.basic` |
| `plugin.ffmpeg` | `media.ffmpeg` |

媒体步骤需要用户已经配置兼容的 FFmpeg。插件可以通过 Manifest 的 `requires.ffmpeg` 声明最低版本，例如 `">=6.0"`。

这些操作只接受宿主管理的输入输出。`plugin.ffmpeg` 通过 `args` 数组及 <code v-pre>{{input.0}}</code>、<code v-pre>{{output}}</code> 占位符传参；宿主固定使用设置中检测到的 FFmpeg，不经过 Shell，也不接受任意可执行文件路径。参数中不要引用宿主未提供的文件路径。

## 插件自带 WASM 处理器

私有加密或转换算法可作为 `.wasm` 随插件分发。JavaScript 识别资源并传参数，Go 核心负责受限执行、流式读写和回滚。

```json
{
  "permissions": {
    "domains": ["api.example.com"],
    "capabilities": ["observe-response", "emit-resource", "process-download"]
  },
  "processors": {
    "decrypt": {
      "runtime": "wasm",
      "entry": "decrypt.wasm",
      "apiVersion": 1
    }
  }
}
```

轨道或下载计划通过 Manifest 中声明的处理器 ID 选择 WASM 模块，不能传入任意本地文件路径：

```javascript
processors: [{
  type: "plugin-wasm",
  options: {
    processor: "decrypt",
    key: payload.key,
    nonce: payload.nonce
  }
}]
```

### 处理本地文件

插件可以把声明过的 WASM 处理器作为资源操作提供给用户。例如用户复制链接并用其他工具下载加密文件后，再回到资源右侧菜单执行解密：

```json
{
  "actions": {
    "decrypt-local-file": {
      "kind": "process-file",
      "processor": "decrypt",
      "inputExtensions": [".mp4"],
      "outputExtension": ".mp4",
      "locales": {
        "zh": {"name": "解密本地视频", "description": "选择已下载的加密文件。"},
        "en": {"name": "Decrypt Local Video", "description": "Select the downloaded encrypted file."}
      }
    }
  }
}
```

资源携带本次操作需要的动态参数：

```javascript
actions: [{
  id: "decrypt-local-file",
  data: {options: {key: payload.key, nonce: payload.nonce}}
}]
```

应用负责显示并执行 `process-file` 操作。用户通过系统对话框选择文件后，应用调用当前插件声明的 WASM 处理器。插件不会取得文件路径或任意读写文件的权限。处理结果保存到原目录，文件名增加 `.decrypted`，原文件保留。

### 向页面发送资源命令

JavaScript 插件可以把资源操作声明为 `page-command`。它必须引用当前 Manifest 中一个 `bridge: true` 的页面脚本，并申请 `inject-page-script` 和 `page-bridge`：

```json
{
  "permissions": {
    "domains": ["www.example.com"],
    "capabilities": ["inject-page-script", "page-bridge"]
  },
  "pageScripts": [{
    "id": "resource-controller",
    "entry": "page/controller.js",
    "match": [{"host": "www.example.com", "path": "/watch/*"}],
    "runAt": "document-start",
    "frames": "top",
    "bridge": true
  }],
  "actions": {
    "inspect-page-resource": {
      "kind": "page-command",
      "pageScript": "resource-controller",
      "locales": {
        "zh": {"name": "检查页面资源"},
        "en": {"name": "Inspect Page Resource"}
      }
    }
  }
}
```

资源携带由插件定义的动态参数，序列化后最大 60 KiB：

```javascript
actions: [{
  id: "inspect-page-resource",
  data: {assetId: payload.id, expectedType: "video"}
}]
```

`action.data` 会随资源写入 `resources.db`，只能放业务 ID、格式选项等可持久化数据；Cookie、Authorization、页面会话令牌和短期 `sessionBuffer` 等值应留在页面内存，并在页面确认目标匹配后使用。

页面脚本通过消息监听器接收命令，并确认命令中的资源 ID 与当前页面一致：

```javascript
pageApi.onMessage(function (message) {
  if (message.type !== "resource-action" || message.actionId !== "inspect-page-resource") return
  if (String(currentAssetId()) !== String(message.data.assetId)) {
    showPageNotice("请打开对应资源后重试")
    return
  }
  inspectCurrentResource(message.requestId)
})
```

默认情况下，应用会检查权限，再将 `data` 发送给对应的页面脚本。具体如何处理由页面脚本决定；普通页面消息不会自动显示在应用界面中。

页面可以自行显示提示，也可以通过 `pageApi.send` 返回带有 `requestId` 的结果，由插件的 `onPageMessage` 处理。需要生成文件时，可将数据写入 Capture Store，再上报资源并通过 `enqueue-download` 创建下载任务。

#### 执行权与进度回报

需要在应用中显示执行进度时，在 `page-command` 操作上设置 `trackProgress: true`。页面命令需要 `inject-page-script` 和 `page-bridge` 权限；自动创建下载任务还需要 `enqueue-download`。

页面先确认命令中的作品或资源 ID 与当前页面一致，再调用 `claim` 申请执行。只有收到 `accepted: true` 后，页面才能开始执行该命令：

```javascript
// message 来自 pageApi.onMessage，已校验 protocol/type/actionId 和视频等业务 ID。
var claim = await pageApi.commands.claim(message.requestId)
if (!claim.accepted) return // 同一命令已由其他匹配页面领取。
try {
  await pageApi.commands.report(message.requestId, {
    state: "running", progress: 25, message: "正在读取页面数据"
  })
  // 执行插件自己的流程。长任务需定期回报，即使百分比没有变化。
  await pageApi.commands.report(message.requestId, {
    state: "completed", progress: 100, message: "处理完成"
  })
} catch (error) {
  await pageApi.commands.report(message.requestId, {
    state: "failed", message: "页面处理失败，请重新打开目标页面"
  })
}
```

**由哪个页面执行**

`claim(requestId)` 确保同一命令只由一个页面执行，即使多个页面同时申请，也只有一个能收到 `accepted: true`。重复申请返回 `accepted: false`。没有收到该命令的页面、其他插件或脚本不能申请执行。

页面中的资源不匹配或页面正在忙碌时，可在申请前报告 `state: "rejected"`。所有接收页面都拒绝后，应用显示失败。只有成功取得执行权的页面可以更新该命令的状态。

**报告状态与进度**

取得执行权后，可以报告 `running`、`completed`、`failed` 或 `cancelled`。命令进入已完成、已失败或已取消状态后，不能再修改其状态。

| 字段或限制 | 说明 |
| --- | --- |
| `progress` | 可省略，或填写 0–100 的有限数字；省略表示进度未知，不会生成估算百分比 |
| `message` | 最长 1024 UTF-8 字节，按纯文本显示；不得包含凭据、私有地址或令牌 |
| 完整请求 | 最大 4096 字节；未知字段和无效类型会被拒绝 |
| 建议报告频率 | 最多每秒一次；进度没有变化时，也应至少每 30 秒报告一次，表明任务仍在运行 |
| 请求频率上限 | 与普通消息共用每个会话每 10 秒最多 100 次的限额 |

以下情况会使命令失败：

- 发出后 30 秒内，没有页面取得执行权；
- 执行过程中，连续 90 秒没有收到进度或心跳报告；
- 命令运行总时长超过 6 小时。

页面断开后，如果无法恢复进度上报，应用会在超时后将命令标记为失败。重载、禁用或卸载插件，也会使该插件已有的命令失效。

**刷新页面后恢复连接**

成功取得执行权的页面会收到短期 `resumeToken`。如果插件需要自动刷新页面，可以：

1. 将 `resumeToken` 临时保存到当前标签页的 `sessionStorage`。
2. 刷新后取出令牌，调用 `claim(requestId, resumeToken)` 恢复连接。
3. 应用重新校验插件和脚本。恢复成功后会返回新令牌，旧令牌失效，旧页面不能再报告进度。

插件需要限制自动重试次数和令牌保存时间，并在使用后删除旧令牌。如需再次刷新，只保留最新令牌。不要将令牌写入资源、日志、fixture 或 URL。重载插件或重启应用后，不能用该令牌恢复连接。

**命令记录与界面显示**

应用最多保存 256 条命令记录。命令完成、失败或取消后，记录保留 10 分钟；查询状态、发送命令或报告进度时，会清理过期记录。同一插件对同一资源的同一操作尚未结束时，不能重复发起。命令状态仅保存在内存中，应用重启后不会恢复，也不会作为下载任务保存。

资源列表约每 1.5 秒查询一次进度，并按 `resourceId` 显示最新命令的状态、百分比和提示。页面处理完成并创建下载任务后，该资源行会转为显示下载或合并进度。

普通 `pageApi.send` 回复不会自动变成进度信息。页面命令接口不能伪造下载任务或指定文件路径。目前应用没有用于取消页面命令的按钮；插件可以在网页上提供取消操作，并报告 `cancelled`。

完整的脱敏示例位于 `examples/plugins/page-command/`。

**错误码**

应用通过固定的 `errorCode` 表示错误，界面会按当前语言显示对应提示。发起操作失败时，本地 API 响应包含 `code`、`message` 和 `data.errorCode`；命令开始后的错误，通过命令状态的 `errorCode` 返回。

| 阶段 | 原因 | `errorCode` |
| --- | --- | --- |
| 发起操作 | 同一操作仍在执行 | `page_command_already_active` |
| 发起操作 | 命令数量达到上限 | `page_command_limit_reached` |
| 发起操作 | 没有符合条件的已连接页面 | `page_command_no_page` |
| 发起操作 | 目标页面的消息队列已满 | `page_command_queue_full` |
| 发起操作 | 服务不可用 | `page_command_unavailable` |
| 发起操作 | 消息参数过大 | `page_command_too_large` |
| 发起操作 | 其他启动错误 | `page_command_start_failed` |
| 等待或执行 | 没有页面接受命令 | `page_command_not_accepted` |
| 等待或执行 | 页面资源不匹配或页面忙碌 | `page_command_target_unavailable` |
| 等待或执行 | 命令超时 | `page_command_timeout` |
| 等待或执行 | 插件被重新加载 | `page_command_reloaded` |

这些错误码由应用设置，插件的普通进度报告不能自行填写。

资源表的“保存路径”列也用于显示页面命令的进度和提示；没有插件提示时，显示应用提供的状态文字。此时内容不可点击。创建下载任务后，该列显示下载或处理进度，下载成功后才显示可打开的文件路径。

应用只翻译自身的状态和错误提示。插件提供的 `message` 会原样显示，多语言文案需要由插件处理。

界面通过 `POST /api/resources/page-commands` 查询命令状态，请求需要 API 会话认证。每次查询最多等待 5 秒，失败后会继续重试。

查询失败时，未结束的命令显示“进度暂不可用”，不再沿用旧的等待状态或百分比。这只表示暂时无法获取进度，不代表命令执行失败。连接恢复后，界面显示最新状态；如果期间已经创建新的下载任务，则显示该下载任务的进度。连续查询失败期间只提示一次，不会每次重试都弹窗。

### ABI v1

模块必须导出线性内存和以下函数，整数均为 WebAssembly `i32`：

```c
int32_t rd_abi_version(void); // 必须返回 1
uint32_t rd_alloc(uint32_t size);
void rd_free(uint32_t pointer, uint32_t size); // 可选
int32_t rd_init(uint32_t options_pointer, uint32_t options_length);
int32_t rd_transform(
  uint32_t pointer,
  uint32_t input_length,
  uint32_t capacity,
  uint32_t offset_low,
  uint32_t offset_high,
  uint32_t final
);
```

- `rd_init` 接收 UTF-8 JSON，宿主内部的处理器归属字段不会传入；返回 `0` 表示成功。
- `rd_alloc` 必须返回可容纳请求长度的有效线性内存地址；宿主会校验地址范围。
- 输入最大分块 256 KiB，模块在同一内存区域写回；`rd_transform` 返回 `0..capacity` 范围内的输出字节数，负数表示处理失败。
- `capacity` 当前为 320 KiB；`offset_low/high` 是当前输入的无符号 64 位字节偏移。
- `final=1` 表示最后一次调用。如果文件大小恰好是分块大小的整数倍，应用会额外调用一次 `rd_transform`，输入长度为 0，`final` 为 1。
- `rd_free` 可选；提供时必须能释放 `rd_alloc` 返回的内存。模块不能保留宿主已经释放的地址。

WASM 模块不能导入 WASI 或应用提供的函数，因此无法访问网络、文件系统、环境变量、时钟或系统命令。每个模块最大 8 MiB，线性内存最多 64 MiB，`options` 最大 64 KiB。单次调用和整个处理过程都有超时限制。处理失败时，应用删除临时输出并保留原文件。完整示例位于 `examples/plugins/wasm-xor/`。

## 声明式插件

声明式运行时适合一条响应直接产生一个单轨资源的 JSON API：

```yaml
runtime: declarative
permissions:
  domains: [api.example.com]
  capabilities: [observe-response, read-response-body, emit-resource]
match:
  - stage: response
    host: api.example.com
    path: /videos
extractors:
  - stage: response
    format: json
    root: $.data.items[*]
    resource:
      url: {path: $.playUrl}
      title: {path: $.title}
      kind: {value: media.video}
      role: {value: video}
      executor: {value: http-file}
      preview: {value: video}
      contentType: {value: video/mp4}
      extension: {value: .mp4}
```

JSON Path 子集支持 `$.a.b`、数组数字下标和结尾的 `[*]`。多请求聚合、关联或自定义下载计划应使用 JavaScript 运行时。

声明式字段：

| 字段 | 说明 |
| --- | --- |
| `stage` | `request` 或 `response`，通常使用 `response` |
| `format` | 当前只支持 `json` |
| `root` | 选择一个对象或对象数组；省略时使用整个 JSON |
| `resource.url` | 必填，下载地址选择器 |
| `resource.title` / `coverUrl` | 可选标题和封面地址 |
| `resource.kind` | 资源细分类，建议与 `resourceKinds` 中声明的 ID 一致 |
| `resource.role` | 轨道角色；省略时根据 `kind` 末段推断 |
| `resource.executor` | 默认 `http-file` |
| `resource.contentType` / `extension` / `size` | 文件类型、扩展名和字节数 |
| `resource.preview` | `image`、`audio`、`video`、`pdf` 或 `text` |
| `resource.metadata` | 自定义元数据选择器映射，站点字段应使用命名空间 |

每个选择器使用 `{path: "$.field"}` 读取数据，或使用 `{value: "固定值"}` 提供常量。无法取得 URL 的条目会被跳过；数值型 `size` 会转换为整数。声明式运行时不会执行自定义代码、跨响应保存状态或生成多输入下载计划。

## 校验和离线回放

无需启动代理即可校验插件：

```bash
go run main.go plugin create
go run main.go plugin lint ./plugins/com.example.my-plugin
go run main.go plugin replay ./examples/plugins/javascript-basic ./examples/plugins/javascript-basic/fixtures/video.json
go run main.go plugin pack ./plugins/com.example.my-plugin
```

fixture 包含脱敏的 `observation` 和预期结果：

```json
{
  "observation": {
    "stage": "response",
    "request": {
      "method": "GET",
      "url": "https://api.example.com/api/videos/42",
      "host": "api.example.com",
      "path": "/api/videos/42",
      "headers": {}
    },
    "response": {
      "statusCode": 200,
      "headers": {"Content-Type": ["application/json"]},
      "contentType": "application/json",
      "body": "{\"url\":\"https://cdn.example.com/42.mp4\"}"
    }
  },
  "expected": {
    "resourceCount": 1,
    "resourceUrls": ["https://cdn.example.com/42.mp4"]
  }
}
```

fixture 可以包含单个 `observation`，也可以通过 `observations` 按顺序回放多个请求，例如页面接口、视频、音频和滚动加载请求。相同 `groupKey` 的资源会按应用实际使用的规则合并。

预期结果支持 `resourceCount`、`resourceUrls`、`processorTypes`、`decision` 和 `patchBodyContains`。模板位于 `examples/plugins/`，JSON Schema 和 TypeScript 声明见 [插件 SDK v1](plugin-sdk.md)。

fixture 编写建议：

- 只保留触发匹配和生成资源所需的 Header 与 Body 字段；
- 将真实域名之外的私人路径、账号和资源 ID 替换为稳定示例值；
- Cookie、Authorization、设备标识和临时签名必须删除或替换；
- 多请求关联使用 `observations` 固定回放顺序，但插件本身仍不能依赖线上到达顺序；
- 至少断言资源数量和 URL；处理器插件还应断言 `processorTypes`。

## 调试与常见错误

插件卡片会显示 Manifest 校验、入口编译和运行时错误。开发时建议按以下顺序排查：

1. 运行 `plugin lint`，先解决目录、字段、权限和入口文件错误；
2. 运行 `plugin replay`，确认 fixture 能稳定输出预期结果；
3. 在应用中重新加载插件，查看插件卡片和日志中的最后错误；
4. 确认页面产生了新的请求，域名和路径确实命中 `permissions.domains` 与 `match`；
5. 检查 Body 权限、`readBody`、Content-Type 和 `truncated`；
6. 最后再检查站点接口、登录状态或签名是否变化。

常见问题：

| 现象 | 常见原因 |
| --- | --- |
| 插件无法加载 | ID、版本、API 版本、入口路径或权限依赖无效 |
| 钩子没有执行 | 阶段、域名、路径、方法或 Content-Type 未命中 |
| `response.body` 为空 | 未申请读取权限、`readBody: false`，或匹配到了另一条规则 |
| JSON 解析失败 | Body 被截断、响应不是 JSON，或站点返回了登录/风控页面 |
| 资源出现但不能下载 | 缺少必需轨道、URL 无效、处理器未声明或能力字段不完整 |
| 插件暂时停止处理 | 连续错误或超时触发熔断；修复后重新加载 |
| 页面脚本未注入 | 页面未经过 TLS 拦截、响应被压缩、CSP 不允许或未找到可注入位置 |
| fixture 通过但线上失败 | fixture 遗漏分支、站点数据变化、凭据过期或线上请求顺序不同 |

每次 JavaScript 钩子最多执行 5 秒，包含初始化和返回结果转换；算上排队时间后，总共最多 10 秒。网页中的脚本不受这个钩子时限限制，但页面命令仍有等待执行、进度报告和总运行时长的限制。

避免在循环中处理超大对象，不要将完整响应、Cookie 或带签名的 URL 写入日志。需要重复排查的问题，应准备脱敏 fixture。

## 发布前检查

建议发布插件前完成以下自检；商店收录的必要条件见[发布要求与建议](extension-store.md#发布要求与建议)：

- 插件 ID 稳定且符合[保留 ID 规则](extension-store.md#插件来源与保留-id)，版本符合语义化版本；
- 域名和 capability 已缩减到实际需要的范围；
- Manifest 中的名称、作者、说明、资源类型和设置项包含合适的本地化文案；
- 至少一个脱敏 fixture 通过离线回放；复杂分支和多请求关联应提供多个 fixture；
- JavaScript、WASM 和页面脚本等运行文件已经包含在插件目录中；
- 日志、fixture、README 和示例配置不含账号、Cookie、Authorization 或私人地址；
- `go run main.go plugin lint ...`、`go run main.go plugin replay ...` 和 `go run main.go plugin pack ...` 均成功；
- 发布到扩展商店且提供 `dist/plugin.zip` 加速包时，已重新打包并在创建 Tag 前提交；
- 在当前稳定版应用中从最终 ZIP 完成一次安装和基本操作检查。

公开发布到扩展商店的仓库结构、GitHub Topic、Release Tag 和版本要求见[发布到扩展商店](extension-store.md)。

## 维护应用内嵌插件（仅项目维护者）

本节用于更新随应用发布的插件快照。独立插件作者完成上述开发、打包和发布流程即可，无需执行这些命令。

在 `res-downloader` 源码根目录运行 `go run main.go plugin sync-bundled <插件目录>`，可校验源插件，并以源目录名称替换 `internal/plugin/bundled/` 中同 ID 的旧快照。

`go run main.go plugin lint-bundled <目录>` 用于核对预装快照，只接受 ID 存在于命令所用应用镜像且目录内容与该内嵌版本完全一致的插件。普通插件开发使用 `lint`；这些命令不用于申请官方身份。
