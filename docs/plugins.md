# 插件开发

本文面向插件开发者，说明插件目录、Manifest、权限、运行时 API、资源模型、下载计划、WASM 处理器、调试测试和发布流程。普通用户安装或管理插件请阅读[插件管理](plugin-management.md)。

res-downloader 使用版本化的插件协议。插件通过结构化数据接收网络观察结果并上报资源，不直接访问 Go 对象、文件系统、Shell 或任意网络接口。

项目仓库中的 `plugins/` 是本地插件开发工作区，方便克隆和调试独立插件仓库。该目录下的插件源码默认不会提交到宿主仓库，也不会被宿主自动加载或打进应用；正式示例位于 `examples/plugins/`，随应用发布的内嵌插件位于 `internal/plugin/bundled/`。

第一次开发建议依次阅读“选择插件类型 → 快速开始 → Manifest → 权限 → 资源模型 → JavaScript 钩子或声明式插件 → 校验和离线回放”。页面脚本、下载计划和 WASM 属于按需使用的高级能力。

当前支持：

- `declarative`：用 JSON/YAML 和受限 JSON Path 从单个 JSON 响应提取单轨资源。
- `javascript`：处理复杂 JSON、跨请求关联、自定义下载计划和资源刷新。
- 插件自带 WASM：在下载输入或输出阶段执行私有解密/转换算法。
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
cp -R examples/plugins/javascript-basic ./my-plugin
```

简单 JSON API 可以复制 `declarative-basic`；WASM 示例位于 `wasm-xor`。也可以通过项目 CLI 创建 JavaScript 脚手架：

```bash
go run main.go plugin create ./my-plugin com.example.video
```

### 2. 修改 Manifest 和入口

至少修改插件 `id`、名称、版本、允许访问的域名和匹配规则。插件 ID 建议使用反向域名格式，并保持长期稳定，例如 `com.example.video`。本地和社区插件不能使用宿主保留的 `builtin.` 或 `official.` 前缀，大小写变体同样会被拒绝；官方商店插件的身份由索引来源决定。

JavaScript 插件在 `main.js` 中实现 `onObservation`。先完成“匹配一个响应并输出一个资源”的最小流程，再逐步加入关联、刷新或下载处理。

### 3. 准备脱敏 fixture

将一次代表性的请求/响应保存为 fixture，并删除 Cookie、Authorization、账号、私人签名和真实用户内容。fixture 应保留插件判断所必需的最小字段。

### 4. 静态校验和离线回放

```bash
go run main.go plugin lint ./my-plugin
go run main.go plugin replay ./my-plugin ./my-plugin/fixtures/video.json
```

`lint` 校验目录、Manifest、入口文件和权限关系；`replay` 在不启动代理的情况下执行 fixture。

项目维护者校验应用内嵌的预装插件时使用内部命令 `plugin lint-bundled <目录>`。该命令只接受 ID 存在于应用镜像且目录内容与内嵌版本完全一致的插件；社区插件始终使用普通 `lint`，不能借此申请 `official.` 前缀。

更新内嵌官方插件时运行 `go run main.go plugin sync-bundled <插件目录>`。命令会完成校验，并以源目录名称覆盖 `internal/plugin/bundled/` 中同 ID 的旧快照，无需手动复制。

### 5. 打包并安装

```bash
go run main.go plugin pack ./my-plugin
```

默认生成 `<插件目录>/dist/plugin.zip`；如有需要，也可以在命令末尾传入自定义输出路径。打包器会排除插件目录中的 `.git/`、`dist/`、`tests/` 和输出文件自身。在应用“插件管理”中选择生成的 ZIP，确认权限后安装。开发期间也可以把插件目录放入用户数据目录的 `plugins` 子目录，然后使用“重新加载”。

Putyy 官方插件可以直接选择本地 ZIP 安装，Manifest 的作者地址需指向 `github.com/putyy` 下的仓库；安装后仍显示为“官方”。其他本地插件显示为“社区”，不能使用 `official.*` ID。

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

插件在应用启动时加载一次，不会定时扫描。开发期间可点击“重新加载”；启停插件和保存插件设置也会刷新当前插件集合。目录名建议与 Manifest 的 `id` 相同。

官方插件随应用升级覆盖。需要二次开发时应复制为新的插件 ID。

## Manifest

manifest 文件可使用 `plugin.json`、`plugin.yaml` 或 `plugin.yml`：

使用 JSON 时，可以查看[Manifest Schema 和编辑器类型声明](plugin-sdk/README.md)获得字段提示；应用的 `plugin lint` 校验结果始终是最终标准。

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
  zh-CN:
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
| `id` | 是 | 最长 64 个字符，只能包含字母、数字、点、短横线和下划线；外部插件不能使用 `builtin.` 或 `official.` 前缀，判断忽略大小写 |
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
| `actions` | 否 | 绑定 WASM 处理器的本地文件操作 |
| `requires` | 否 | 可选宿主工具要求，例如 `ffmpeg: ">=6.0"` |

`match` 中的 `host`、`path` 和完整 `url` 支持 `*` 通配符；`method` 忽略大小写。`contentTypes` 匹配响应 Content-Type，`readBody` 决定命中该规则时是否需要 Body。只依赖 URL 或响应头时应明确设置 `readBody: false`。

`resourceKinds` 会出现在首页抓取类型中，用户既可以按稳定的 `primaryType` 宽泛筛选，也可以按插件的 `kind` 精确筛选。

`settingsSchema` 支持 `string`、`number`、`integer`、`boolean`、`object`、`array` 和 `enum` 的基础校验。基础类型会在插件管理页生成表单；复杂结构仍可使用高级 JSON 编辑。`locales` 依次匹配完整语言、基础语言、英文和任意首项，最后回退到顶层 `name`。

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
| `page-bridge` | 页面脚本与插件运行时交换 JSON 消息 | 需要 `inject-page-script` |
| `capture-response-body` | 缓存浏览器实际读取的 Range 响应，或接收页面脚本捕获的媒体分片 | 需要 `observe-response`；页面分片还需要 `inject-page-script` 和 `page-bridge` |
| `enqueue-download` | 页面消息上报资源后自动创建下载任务 | 需要 `page-bridge` 和 `emit-resource`；属于会写入下载目录的敏感权限 |

Body 只有在域名、规则和读取权限同时满足时才会进入插件。超过 `bodyLimit` 后快照会标记 `truncated`，截断响应不能被插件修改。

JavaScript 每次钩子调用使用独立运行时，限制为 500ms 和 1MiB 脚本。插件还有并发上限、慢调用统计和连续失败熔断；它没有 Node.js、Promise 等待、`fetch`、文件系统或系统命令 API。跨请求状态必须使用核心提供的关联接口，而不是依赖 JS 全局变量。

## 页面脚本和双向消息桥

JavaScript 插件可声明由宿主注入目标网页主世界的 `pageScripts`。注入由 MITM 代理完成，因此目标域名必须已被当前拦截规则捕获；不经过 `onObservation`，也不占用 `bodyLimit` 或 500ms 的 Goja 调用时间。当前仅支持 `document-start`；`frames` 可为 `top`（默认）或 `all`。

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

每个入口必须是插件目录内的普通 `.js` 文件，单文件不超过 256 KiB，每个插件最多声明 8 个。宿主只扫描 HTML 开头最多 256 KiB，优先复用页面已有的 CSP nonce；找不到 `<head>`、响应不是未压缩的 `text/html`，或 CSP 不允许安全内联时会保持原响应不变。宿主不会删除或放宽网站 CSP。

页面入口运行在异步函数中，并获得局部 `pageApi`：

```javascript
pageApi.onMessage(function (message) {
  if (message.type === "probe") runProbe(message.url)
})

var result = await pageApi.send({type: "player-ready", data: collectPlayerData()})
```

声明了 `capture-response-body` 的页面脚本还可通过同一个页面会话令牌把二进制媒体分片写入宿主现有 Capture Store：

```javascript
await pageApi.capture.start("video:123:video")
await pageApi.capture.write("video:123:video", arrayBufferOrTypedArray)
await pageApi.capture.complete("video:123:video")
// 取消或失败时：await pageApi.capture.abort("video:123:video")
```

`start` 会重置同名缓存，`write` 按调用完成顺序追加字节，`complete` 后才能由 `capture-file` 读取，`abort` 会立即丢弃未完成缓存。页面端负责按媒体时间线排序和去重；宿主不解析站点私有流协议。单个二进制分片最大 32 MiB，每个页面会话最多同时使用 4 个捕获键、累计写入 16 GiB。入口仍受页面 Origin、随机会话令牌、插件权限和插件作用域限制，不会把文件系统或任意缓存键暴露给网页。

启用桥后，页面到插件使用同源 POST，插件到页面使用 SSE。内部地址由代理直接响应，不会发往目标网站。页面每次加载获得独立 `pageSessionId`；页面关闭、插件重载、禁用或卸载后会话失效。

插件使用新的同步顶层钩子处理页面消息：

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

`context` 包含 `pageSessionId`、`scriptId`、`pageUrl` 和 `origin`。插件还可使用 `api.page.broadcast(filter, message)` 和 `api.page.sessions(filter)`；这些接口只会提供给声明了 `page-bridge` 权限的插件。普通消息与回复仅接受 JSON，单条最大 64 KiB；大块媒体数据必须使用 `pageApi.capture.write`。每个插件最多 32 个活动页面会话，每个会话具有队列、连接数和速率限制。

页面脚本和目标网站代码处于同一个主世界，网站代码理论上可以观察或模拟桥请求。因此页面消息始终是不可信输入；默认消息桥不会授予页面文件、Shell、数据库、下载器或其他插件访问权。`enqueue-download` 是显式例外，只允许把当前插件刚发布且通过校验的资源送入宿主下载队列，不暴露文件路径或任意任务控制接口。

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
- `groupKey`：插件域内稳定的逻辑资源键。跨请求增量合并时必填。
- `parentGroupKey`：可选的父资源 `groupKey`。设置后当前资源作为子项显示在父记录的展开区域；父项下载会批量下载其所有可下载子项。
- `parentId`：核心解析并持久化的父资源 ID。插件不能自行指定；插件输出中的该字段会被核心清空，避免跨插件挂接资源。
- `dedupeKey`：可选的自定义去重键；通常让核心从 `pluginId + groupKey` 或主轨 URL 生成。
- `tracks`：独立获取的输入轨道。`id` 在资源内唯一，`role` 表示语义，`executor` 默认 `http-file`。
- `requiredTracks`：资源进入 `ready` 状态所需的角色；缺少任一角色时为 `partial`，前端不会开放下载。
- `capabilities`：前端按钮的能力来源，当前通用值为 `download`、`preview`、`open`、`copy`。
- `preview`：选择受信任的前端渲染器以及用于预览的轨道。插件不能注入前端代码。
- `coverUrl`：可选封面地址；不要把大段 Base64 图片直接放入资源。
- `technical`：可选 MIME、容器、编码和时长信息，用于展示或命名。
- `lifecycle.expiresAt`：可选的毫秒时间戳，表示链接预计过期时间。
- `metadata`：站点私有字段应使用命名空间；通用 `author` 可供文件名模板使用。
- `actions`：引用 Manifest 中声明的宿主操作，例如使用 WASM 处理本地文件。

资源输出还需满足以下约束：

- `groupKey` 和 `parentGroupKey` 最长 512 字节，子资源不能把自己设为父级；
- 没有轨道的合集必须提供稳定 `groupKey`；
- `track.id` 在资源内唯一，轨道 URL 必须是合法远程地址；`capture-file` 改用 `captureKey`；
- 扩展名必须以 `.` 开头且最长 20 个字符；
- `preview.trackId` 必须引用当前资源已有轨道；
- 单个资源序列化后不能超过 1 MiB；不要把完整响应 Body 放入 `metadata`。

合集父项推荐使用 `media.collection`、稳定的 `groupKey` 和 `download` 能力，本身不包含 `tracks`。图片、音频等独立输出应作为带 `parentGroupKey` 的子资源；不要把它们放进父项的 `tracks`，因为轨道表示生成单个输出所需的输入。父项删除会级联删除子项，删除单个子项不会影响同级资源。

`preview.renderer` 当前支持 `image`、`audio`、`video`、`pdf`、`text`。需要处理器的预览请求只接受核心资源 ID，后端按 `trackId` 选择直接输入，并执行该输入和输出声明的 WASM 处理链。因此加密资源也可在插件参与后预览。没有处理器的普通图片优先由 WebView 直接加载，以兼容会拒绝 Go TLS 客户端指纹的图片 CDN；直连加载失败时再回退到携带捕获 Header 的后端代理。HLS 资源的 Master Playlist、媒体清单、分片、初始化段和 AES Key 会被改写为短期本地令牌地址，所有请求由后端复用轨道 Header 转发，不要求源站支持 CORS，也不会把预览接口开放成任意 URL 代理。

### 增量聚合和关联

`api.upsert(resource)` 与 `api.emit(resource)` 的提交行为相同；当资源具有相同的 `pluginId + groupKey` 时，核心按 `track.id` 原子合并并向前端发送更新事件。

分离请求不能按到达顺序配对。插件应先从业务接口取得作品 ID 和候选 URL，再登记一对多别名：

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

关联表由 Go 核心按插件隔离，支持一对多，并带 TTL 和容量上限。核心只做保守 URL 规范化；哪些签名参数可以删除必须由站点插件决定。无法可靠关联时，插件应输出独立或未完成资源，不能按请求顺序猜测。

## JavaScript 钩子

JavaScript 运行时支持四个同步顶层全局函数，可按插件需求实现：

```javascript
function onObservation(observation, api) {
  var payload = JSON.parse(observation.response.body)
  api.log("found " + payload.id)
  api.emit(/* Resource */)
  return {decision: "continue", handled: true}
}

function createDownloadPlan(input) {
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

`observation.settings` 和 `input.options.settings` 是应用中保存的插件设置。`api.pluginVersion` 是 manifest 版本。`decision` 可为 `continue` 或 `stop`。

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

| API | 返回值 | 说明 |
| --- | --- | --- |
| `api.emit(resource)` | `void` | 上报资源；与 `upsert` 使用相同的合并语义 |
| `api.upsert(resource)` | `void` | 推荐用于具有稳定 `groupKey` 的增量资源 |
| `api.log(message)` | `void` | 写入插件日志；调用方负责脱敏 |
| `api.pluginVersion` | `string` | 当前 Manifest 版本 |
| `api.correlate.register(value)` | `void` | 登记 URL 别名与逻辑资源、轨道的关联 |
| `api.correlate.find(url)` | `ResourceReference[]` | 查找当前插件登记的关联 |
| `api.page.send(sessionId, message)` | `boolean` | 向一个页面会话发送消息；需要 `page-bridge` |
| `api.page.broadcast(filter, message)` | `number` | 向匹配页面广播，返回接收会话数 |
| `api.page.sessions(filter?)` | `PageMessageContext[]` | 列出当前插件可见的页面会话 |

`emit` 只把资源加入本次调用的输出队列，最终仍会经过字段、大小、URL、轨道、处理器和操作校验。不要依赖无效资源被静默修复。

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

资源 URL、Header 或签名会过期时，可实现 `refreshResource(input)`。它接收与 `createDownloadPlan` 相同的 `{resource, options}`，并返回：

```javascript
return {
  status: "refreshed",
  resource: updatedResource,
  message: ""
}
```

`status` 可为 `refreshed`、`authenticationRequired` 或 `recaptureRequired`。成功时返回完整的更新资源；需要重新登录或重新访问页面时返回原资源和对应状态，并可在 `message` 中提供不含敏感信息的提示。没有实现或返回 `null` 表示不支持刷新。

刷新逻辑只能依据资源中的业务 ID、页面 URL、当前设置等已有信息重新计算；插件没有 `fetch`，也不能无中生有地生成登录凭据。通常应通过页面脚本或新的网络 observation 获得最新数据，再用稳定 `groupKey` 更新资源。

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

`createDownloadPlan(input)` 把逻辑资源转换为可持久化的下载 DAG：

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

核心并行获取 `inputs`，按顺序执行 `pipeline`，然后执行输出处理器并原子安装结果。普通 `http-file` 输入会在任务工作目录记录 Range 分片检查点，分轨和合集任务也能暂停后继续；进入输入处理器、WASM 或媒体流水线后不接受暂停。`hls` 只支持取消，`ffmpeg-hls` 直播使用“停止并保存”。取消、进度、临时文件和失败回滚均由核心负责。

可用输入执行器：

- `http-file`：普通 HTTP/HTTPS 文件，支持请求头、Range 并发和下载代理。
- `capture-file`：读取宿主旁路捕获且已经完整的响应缓存，不再访问远端 URL；需要 `capture-response-body`。
- `hls`：解析 master/media playlist，支持相对 URL、最高/最低或最大带宽选择、`EXT-X-MAP`、`BYTERANGE`、AES-128 和 VOD/显式快照下载，不支持暂停。
- `ffmpeg-hls`：由用户安装的 FFmpeg 直接下载或录制网络流，支持请求头、重连和最大录制时长；需要 `media.ffmpeg.network`。直播停止后宿主保留并安装有效输出。

计划中的输入 ID 和流水线步骤 ID 必须合法且不能重复。每个流水线步骤只能引用前面已经声明的输入或步骤，`output.input` 必须引用最终可用结果。`capture-file` 不能同时提供 URL，其他输入必须提供合法的 HTTP/HTTPS URL。

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

流水线权限：

| executor | 所需权限 |
| --- | --- |
| `builtin.concat` | 无额外权限 |
| `builtin.media.mux` | `media.basic` |
| `builtin.media.remux` | `media.basic` |
| `builtin.media.extract_audio` | `media.basic` |
| `plugin.ffmpeg` | `media.ffmpeg` |

媒体步骤需要用户已经配置兼容的 FFmpeg。插件可以通过 Manifest 的 `requires.ffmpeg` 声明最低版本，例如 `">=6.0"`。

这些操作只接受宿主管理的输入输出。`plugin.ffmpeg` 通过 `args` 数组及 `{{input.0}}`、`{{output}}` 占位符传参；宿主固定使用设置中检测到的 FFmpeg，不经过 Shell，也不接受任意可执行文件路径。参数中不要引用宿主未提供的文件路径。

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

轨道或下载计划只按 manifest 中的处理器 ID 引用模块，不能传入任意本地路径：

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

`process-file` 由宿主渲染并执行：用户通过系统对话框选取文件，Go 只调用当前插件清单中绑定的 WASM，插件不会获得文件系统路径或任意读写权限。处理结果写为同目录的 `.decrypted` 新文件，原文件不会被覆盖。

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
- `final=1` 表示最后一次调用；整分块文件还会收到一次零长度最终调用。
- `rd_free` 可选；提供时必须能释放 `rd_alloc` 返回的内存。模块不能保留宿主已经释放的地址。

模块不注册 WASI 或宿主导入，因此没有网络、文件系统、环境变量、时钟或命令能力。单模块最大 8 MiB、线性内存最多 64 MiB、options 最大 64 KiB；单次调用和总处理均有超时。处理失败时宿主删除临时输出，不覆盖原文件。完整示例位于 `examples/plugins/wasm-xor/`。

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
go run main.go plugin create ./my-plugin com.example.video
go run main.go plugin lint ./my-plugin
go run main.go plugin replay ./examples/plugins/javascript-basic ./examples/plugins/javascript-basic/fixtures/video.json
go run main.go plugin pack ./my-plugin
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

fixture 可以用单个 `observation`，也可以用 `observations` 顺序回放页面接口、视频、音频和滚动加载等多请求会话；同一 `groupKey` 会按真实运行语义聚合。测试支持 `resourceCount`、`resourceUrls`、`processorTypes`、`decision` 和 `patchBodyContains`。模板位于 `examples/plugins/`，JSON Schema 和 TypeScript 声明见 [插件 SDK v1](plugin-sdk/README.md)。

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

每次 JavaScript 钩子最多运行 500ms。不要在循环中处理超大对象，也不要把完整响应、Cookie 或带签名 URL 写入日志。需要长期复现的数据应制作脱敏 fixture。

## 发布前检查

发布插件前确认：

- 插件 ID 稳定且不使用 `builtin.`、`official.` 保留前缀，版本符合语义化版本；
- 域名和 capability 已缩减到实际需要的范围；
- Manifest 中的名称、作者、说明、资源类型和设置项包含合适的本地化文案；
- 至少一个脱敏 fixture 通过离线回放；复杂分支和多请求关联应提供多个 fixture；
- JavaScript、WASM 和页面脚本等运行文件已经包含在插件目录中；
- 日志、fixture、README 和示例配置不含账号、Cookie、Authorization 或私人地址；
- `go run main.go plugin lint ...`、`go run main.go plugin replay ...` 和 `go run main.go plugin pack ...` 均成功；
- 发布到扩展商店时，`dist/plugin.zip` 已在创建 Tag 前提交；
- 在当前稳定版应用中从最终 ZIP 完成一次安装和基本操作检查。

公开发布到扩展商店的仓库结构、GitHub Topic、Release Tag 和版本要求见[发布到扩展商店](extension-store.md)。
