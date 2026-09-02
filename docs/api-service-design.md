# res-downloader 业务与接口设计文档

> **执行摘要**：**有接口服务设计**，但形态是"桌面应用本地 HTTP API"而非对外服务——前端（Vue3）通过 axios 调用本机 `127.0.0.1:8899/api/*`（core/middleware.go 路由表，共 18 个端点），后端→前端经 Wails 事件总线推送；Wails Bind 仅保留 3 个辅助方法。无鉴权、无 OpenAPI 契约、无版本化。文末给出缺口评估与演进建议。

---

## 1. 结论：接口服务的真实形态

| 问题 | 结论 |
|---|---|
| 有无接口服务设计？ | **有**。`HttpServer`（core/http.go）在 `127.0.0.1:8899` 上同时提供 `/api/*` 控制面 API 与 goproxy 数据面代理，按 `Host` 头分流（http.go `run()`） |
| 是 RESTful 微服务吗？ | **不是**。端点全部 POST 风格动作式命名（RPC-over-HTTP），仅本机回环可达，无鉴权 |
| 前端如何调用？ | `frontend/src/api/request.ts`（axios 实例）→ `frontend/src/api/app.ts`（18 个封装方法），baseURL 指向 `127.0.0.1:8899` |
| Wails Bind 的作用？ | 仅 `Config() / AppInfo() / ResetApp()`（core/bind.go），主链路不使用 |
| 后端→前端的回推通道？ | `runtime.EventsEmit(ctx, "event", json)`，事件载荷 `{type, data}`，type ∈ `newResources / downloadProgress`（http.go `send()`） |
| 是否对外开放？ | 监听地址取 `Config.Host`（默认 127.0.0.1），理论上改成 0.0.0.0 即局域网可达——**当前无任何鉴权保护** |

---

## 2. 接口清单（已存在，共 18 个）

统一约定：除注明外均为 `POST`；统一响应壳 `{code: int, message: string, data: any}`，`code=1` 成功、`code=0` 失败；`/api/*` 带 CORS `Access-Control-Allow-Origin: *`。

| # | 端点 | 入参 | 出参 data | 用途 | 实现 |
|---|---|---|---|---|---|
| 1 | /api/install | – | `{isPass: bool}` | 安装/查询 CA 证书（isPass=是否免密） | http.go install |
| 2 | /api/set-system-password | `{password, isCache}` | – | 缓存系统密码用于写系统代理/证书 | setSystemPassword |
| 3 | /api/proxy-open | – | `{value: bool}` | 开启系统代理 | openSystemProxy |
| 4 | /api/proxy-unset | – | `{value: bool}` | 关闭系统代理 | unsetSystemProxy |
| 5 | /api/is-proxy | – | `{value: bool}` | 查询代理状态 | isProxy |
| 6 | /api/open-directory | – | `{folder}` | 系统目录选择对话框 | openDirectoryDialog |
| 7 | /api/open-file | – | `{file}` | 文件选择对话框（过滤器 *.mp4） | openFileDialog |
| 8 | /api/open-folder | `{filePath}` | – | 在系统文件管理器中打开 | openFolder |
| 9 | /api/app-info | – | App 结构体 | 应用信息（名称/版本/代理状态） | appInfo |
| 10 | /api/get-config | – | Config 全量 | 读取配置 | getConfig |
| 11 | /api/set-config | Config 全量 JSON | – | 保存配置（热更代理/规则） | setConfig |
| 12 | /api/set-type | `{type: "video,image"}` | – | 设置嗅探类型过滤集 | setType |
| 13 | /api/clear | – | – | 清空资源去重表（清空列表） | clear |
| 14 | /api/delete | `{sign: []string}` | – | 按 urlSign 删除指定资源记录 | delete |
| 15 | /api/download | MediaInfo + `{decodeStr}` | – | 受理下载（异步，进度走事件） | download |
| 16 | /api/cancel | MediaInfo（用 Id） | – | 取消下载任务 | cancel |
| 17 | /api/wx-file-decode | MediaInfo + `{filename, decodeStr}` | `{save_path}` | 解密本地已存的微信加密视频 | wxFileDecode |
| 18 | /api/batch-export | `{content}` | `{file_name}` | 导出链接清单 txt 到下载目录 | batchExport |

**两个特例（非 POST 动作式）**：

| 端点 | 方法 | 说明 |
|---|---|---|
| /api/preview | GET `?url=` | 回源拉流并透传 Range/响应头，用于前端在线预览；不包统一响应壳，直接转发字节流 |
| /api/cert | GET | 下载 CA 根证书（`res-downloader-public.crt`），供手动安装 |

**Wails Bind 方法（辅助）**：`Bind.Config()`、`Bind.AppInfo()`（均返回 ResponseData）、`Bind.ResetApp()`（重置并退出）。

**事件推送（后端→前端）**：

| type | data | 触发点 |
|---|---|---|
| `newResources` | MediaInfo | 插件判定出新资源（default/qq 插件） |
| `downloadProgress` | `{Id, Status, SavePath, Message}` | 下载进度/完成/失败/解密中 |

---

## 3. 领域模型

### 3.1 MediaInfo（core/shared/base.go）——核心资源实体

| 字段 | 类型 | 说明 |
|---|---|---|
| Id | string | nanoid（失败时退化为 urlSign） |
| Url | string | 实际下载地址（视频号含 urlToken 拼接） |
| UrlSign | string | Md5(Url)，去重与删除键 |
| CoverUrl | string | 封面（视频号回传） |
| Size | float64 | 字节数（content-length 或平台回传 fileSize） |
| Domain | string | 顶级域 |
| Classify | string | image/audio/video/live/m3u8/pdf/ppt/xls/doc/font/stream |
| Suffix | string | 扩展名（如 .mp4；unknown 时取 URL 后缀） |
| SavePath | string | 落盘路径（下载时生成） |
| Status | string | ready / running / done / error |
| DecodeKey | string | 视频号 XOR 解密密钥（base64） |
| Description | string | 视频描述（用于文件名） |
| ContentType | string | 原始 MIME |
| OtherData | map[string]string | 扩展袋：`headers`（原始请求头 JSON）、`wx_file_formats`（清晰度列表 # 分隔） |

### 3.2 Config（core/config.go）——配置实体

监听（Host/Port）、画质（Quality 0/1/2+）、保存目录与命名（SaveDirectory/FilenameLen/FilenameTime）、代理链（UpstreamProxy/OpenProxy/DownloadProxy/AutoProxy）、视频号开关（WxAction）、并发（TaskNumber=CPU×2 分片数、DownNumber）、伪装（UserAgent/UseHeaders/InsertTail）、**MimeMap**（MIME→类型/后缀判定表，可用户编辑）、**Rule**（MITM 域名规则文本）。持久化于用户目录 `config.json`（storage.go），`setConfig` 对代理与规则热生效。

### 3.3 其他结构

- `ResponseData{code, message, data}`：统一响应壳。
- `FileDownloader / DownloadTask`（downloader.go）：Range 分片下载，MaxRetries=3、RetryDelay=3s、MinPartSize=1MB，context 取消。
- `RuleSet / Rule`（rule.go）：`*`、`*.domain`、`!` 否定规则。
- `shared.Plugin / shared.Bridge`（shared/plugin.go）：平台插件契约与宿主能力桥（见 §5 缺口评估）。

---

## 4. 接口详细设计（示例）

### 4.1 POST /api/download

```jsonc
// Request
{
  "Id": "V1StGXR8_Z5j",
  "Url": "https://finder.video.qq.com/...?encfilekey=...&token=...",
  "UrlSign": "9e107d9d372bb6826bd81d3542a419d6",
  "Classify": "video",
  "Suffix": ".mp4",
  "DecodeKey": "AAECAwQ=",          // 视频号资源才有
  "Description": "视频标题",
  "OtherData": {"headers": "{\"Referer\":[...]}", "wx_file_formats": "..."},
  "decodeStr": "AAECAwQ="            // 前端解密结果直传时与 DecodeKey 相同语义
}
// Response（受理即返回，进度走 downloadProgress 事件）
{ "code": 1, "message": "ok", "data": null }
```

语义：幂等性弱——同一 Id 重复调用会再起一个下载任务覆盖 tasks 表项；错误通过事件 `downloadProgress{Status:"error", Message}` 异步上报，而非同步响应。

### 4.2 POST /api/set-type

```jsonc
{ "type": "video,image" }   // 空串 = 全不选
```
语义：重置内存中的类型开关（`resType` map 全部置 false 后勾选传入项），即时影响后续嗅探过滤；不影响已入列资源。

### 4.3 GET /api/preview?url=<escaped>

特例流式端点：透传 `Range` 请求头与上游全部响应头（剔除 ACAO），状态码原样转发。失败返回 `400/500` 纯文本（**与统一响应壳不一致**）。

---

## 5. 设计评估与缺口

| 维度 | 现状 | 评估 |
|---|---|---|
| 鉴权 | 无 | ⚠️ `/api/*` 对任何本机进程开放；CORS `*` 意味着任意网页可驱动本地下载（DNS rebinding 面）。建议绑定随机 token 于启动时注入前端 |
| 契约 | 无 OpenAPI/JSON Schema；Go struct 即契约 | ⚠️ 字段依赖后端 struct 零值语义，缺校验（如 `/api/download` 未校验 SaveDirectory 非空则静默 return） |
| 错误模型 | 同步 code=0 + 异步事件混用 | ⚠️ 下载错误只在事件里，HTTP 层恒 200；错误码无枚举文档 |
| 幂等 | download 非幂等；clear/delete 天然幂等 | 可接受（桌面场景），但重复点击会重复落盘 |
| 版本化 | 无 /v1 前缀 | 低优先级（前后端同包发布） |
| 插件契约 | `Plugin` 接口清晰，Bridge 注入宿主能力 | ✅ 扩展点设计良好；缺插件动态注册/配置化启用 |
| 对外开放 | 无公网/局域网服务能力 | 若未来提供 OpenAPI：建议新增独立端口、token 鉴权、只读资源列表 + 下载受理两个端点起步，可输出 OpenAPI 3.0 YAML |

**优先改进建议**：
1. `/api/preview` 错误响应与统一壳对齐；为全部端点补 400 参数校验。
2. 启动时生成一次性 API token，前端经 Wails Bind（不走 HTTP）获取，HTTP 请求头携带校验——低成本堵住本机/网页越权调用。
3. `downloadProgress` 事件补充 `TotalSize/DownloadedSize` 数值字段，避免前端解析 "57%" 字符串。
4. `GET /api/resources`（当前缺）：资源列表只存于前端内存 + 后端 mediaMark 去重表，刷新页面即丢；如需恢复需后端落存。

---

## 6. 非功能性说明

- **并发模型**：`sync.Map`（mediaMark / tasks）、`sync.RWMutex`（resType、MimeMap、RuleSet）；每个下载任务独立 goroutine + `TaskNumber` 个分片 goroutine；`MaxIdleConnsPerHost=100`。
- **超时**：代理链 TLS/Dial/ResponseHeader 均 60s，IdleConn 30s；下载客户端无全局超时（依赖 context 取消与分片重试）。
- **存储**：仅配置与证书/锁文件落盘（用户配置目录）；资源元数据全在内存，进程退出即失。
- **风控对抗**：回放原始请求头（含 UA/Cookie）下载；`UserAgent` 可配；支持上游代理链（UpstreamProxy）。视频号链接时效性由平台 token 决定，久置会失效——属已知限制（待确认具体时效）。
- **安全**：内置自签 CA（私钥硬编码于 app.go）用于 MITM——意味着任何装了该 CA 的机器都信任此私钥签发的证书，属本项目固有信任模型。
