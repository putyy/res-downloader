---
description: 声明插件业务操作，连接页面会话，并通过统一执行服务处理结果、批量、取消、历史与产物。
---

# 插件业务操作

业务操作面向搜索、详情、当前内容、列表、解析和文本等能力，无需先捕获资源。桌面、CLI 与 MCP 调用同一个 `internal/operation` 服务。网站接口、选择器和数据解析由插件实现；用户先打开目标网页并完成登录。页面仍须满足代理注入和消息桥条件，声明操作不会自动提供网站搜索。

## Manifest 契约

在 JavaScript Manifest 中添加以稳定 ID 为键的 `operations`，最多 32 项。每项需要 `name`、`category`、`pageScript`、`inputSchema`、`outputSchema` 和非空 `effects`。`pageScript` 必须引用当前插件 `bridge: true` 的脚本，并申请 `inject-page-script` 与 `page-bridge`。

| 字段 | 规则 |
| --- | --- |
| `name` / `description` / `locales` | 名称最多 128 字符，描述最多 2048 字符；本地化沿用插件格式 |
| `category` | `search`、`detail`、`current-content`、`list`、`resolve`、`text`、`custom` |
| `pageMatch` | 可选的页面匹配规则，必须位于插件权限域名内 |
| `requiresLogin` | 为真时仅接受插件明确报告 `authenticated` 的页面 |
| `inputSchema` / `outputSchema` | 每份最多 16 KiB，输入根类型必须是 object |
| `examples` | 最多 4 份通过输入 Schema 的参数示例 |
| `effects` | `read` 读取；`page` 改变页面；`publish` 发布资源；`download` 创建下载 |
| `timeoutSeconds` | 0 或省略使用 120 秒，最高 1800 秒 |
| `cancellable` | 是否允许取消运行中的调用；排队调用可直接取消 |
| `safeRetry` | 仅允许纯 `read` 操作；允许有界手动重提，不表示自动重试 |
| `allowReload` | 默认 false；要求 `effects` 含 `page`，允许 handler 主动刷新一次并接续同一执行，与 `safeRetry` 独立 |
| `automation` | CLI / MCP 默认暴露策略，默认 false；用户可按插件和操作覆盖 |
| `persistResult` | 默认 false；仅允许按 outputSchema 白名单保存投影 |
| `resultTTLSeconds` | 0 使用宿主结果期限；可缩短，不能超过 86400 秒 |

`publish` 需要 `emit-resource`；`download` 同时需要 `emit-resource` 和 `enqueue-download`。宿主检查实际资源及下载输出，声明不是对任意页面 JavaScript 行为的沙箱保证。

### Schema 支持范围

| 用途 | 支持字段或规则 |
| --- | --- |
| 类型 | 单一 `type`：`object`、`array`、`string`、`number`、`integer`、`boolean`、`null` |
| 说明 | `title`、`description` |
| 对象 | `properties`、`required`，必须显式设置 `additionalProperties: false` |
| 数组 | 必须提供 `items`，支持 `minItems` / `maxItems` |
| 值约束 | 标量 `enum`、`minimum` / `maximum`、`minLength` / `maxLength` |
| 持久化 | `x-persist`、`x-sensitive`，见[保存规则](#保存规则) |
| 结构上限 | 深度最多 12，每层最多 64 个属性 |

不支持 `$ref`、联合类型、正则或默认值。未知关键字、与类型不匹配的约束和倒置的上下界会被拒绝。

## 页面执行

```javascript
pageApi.operations.handle("current-content", async function (ctx) {
  if (ctx.signal.aborted) throw new Error("cancelled")
  await ctx.report(25)
  return {data: {title: document.title}, count: 1}
})
await pageApi.operations.setState({ready: true, login: "unknown", context: "current-target"})
```

`handle(id, handler)` 返回注销函数。参数包含 `executionId`、已校验的 `input`、插件原始 `cursor`、`limit`、可选 `resource: {id, actionId}`、`AbortSignal` 和 `report(progress)`。handler 返回 `{data, count, hasMore, nextCursor, truncated}`；抛出异常表示失败，SDK 不把页面原始异常作为可信错误输出。`report` 是 0–100 的进度百分比，SDK 每 15 秒发送心跳。

### 页面状态与会话

`setState` 返回 `{revision}`，SPA 切换、账号或就绪状态变化后需重新调用：

| 字段 | 说明 |
| --- | --- |
| `login` | `unknown`、`authenticated` 或 `required`；桥已连接不代表登录成功 |
| `context` | 当前目标标识，最多 128 字符，不得包含凭据 |
| `ready` | 页面可以接收操作，不要求所需数据已全部加载；handler 应校验目标并有界等待内容 |

URL、`context`、`login` 或 `ready` 变化会更新 revision，使原执行失效。SDK 自动维护上报 revision；旧 handler 的结束上报只确认停止并释放页面占用，不提交过期结果或覆盖终态。

就绪上报每次最多等待 5 秒，网络失败、超时或临时服务错误时最多尝试 3 次。新状态或离开页面会中止旧上报，相同状态的并发上报共用请求。操作提前到达时，SDK 等待上报结束再核对 revision 并领取执行，期间仍可取消。这些重试仅同步状态，不重放业务操作。

普通手动刷新会创建新会话并中断原执行，只有下述显式刷新检查点允许接续。进入往返缓存（BFCache）会中止旧 handler；恢复页面后重新连接消息桥，不重播操作。

会话发现返回插件、脚本、`pageSessionId`、标题、URL、连接、业务就绪、登录状态、revision 和操作 ID。唯一匹配页面可自动选取；多个匹配页面必须指定 `pageSessionId`，不会广播执行。

### 显式单次刷新

需要刷新网页重新获取内容时，声明 `allowReload: true` 和 `effects: ["read", "page"]`，并补充实际使用的其他 effects。handler 可使用 `reloadCount: 0 | 1` 和 `reload(): Promise<never>`：

- 先清理尚未提交的 Capture Store 缓存，再于最终提交前 `return ctx.reload()` 或 `await ctx.reload()`，由 SDK 刷新。
- 此调用必须结束当前 handler，不得捕获后继续执行、自行调用 `location.reload()`，或在发布结果后调用。
- 刷新后以原 `executionId` 和输入从入口重新运行，`reloadCount` 为 1。插件须重新校验目标、获取数据，不恢复旧 JavaScript 栈。
- 最多接续一次，不延长总运行期限；`safeRetry` 仍仅允许纯只读操作。

刷新检查点最多保留 60 秒，且不超过原执行期限。只有同插件、同运行时、同页面脚本、完全相同的页面 URL、登录状态和 context，才可由新的 ready 页面及 SSE 连接认领一次。取消、导航至其他视频或 URL、插件重载、宿主重启或检查点过期后不能续跑。普通手动刷新和 BFCache 恢复不会自动重提操作。

`pageApi.operations.hasPendingReload(): boolean` 提供页面初始化时的只读提示，不领取或触发执行，也不替代 `setState`。接续由 SDK 管理，不对公共 HTTP / CLI 开放；执行摘要可包含 `reloadCount`。

### 结果提交与资源按钮

成功结果经同步钩子 `onPageMessage({type: "operation-result", operationId, executionId, data}, context, api)` 整理，可返回 `data`、`resources` 和 `autoDownload`：

- 页面结果与整理后的数据均需通过 `outputSchema`；异步网站请求应在页面 handler 完成，Goja 钩子不能等待 Promise。
- 资源需通过域名、归属、`groupKey`、轨道、处理器、权限和下载计划校验，只登记实际发布的资源和创建的下载 ID。
- 整理总时限为剩余操作期限与 10 秒的较小值；取消会中止钩子和计划校验。
- 整理后的完整 JSON 最多 2 MiB，资源轨道合计最多 128 条。

终态产生后若确认已有资源发布或下载创建，仍会补记关联，不改变终态。

资源按钮可引用同插件操作：

```json
{"actions":{"resolve-item":{"kind":"operation","operation":"resolve"}}}
```

资源的 `actions: [{id: "resolve-item", data: {id: "stable-item-id"}}]` 提供输入。宿主从已保存资源中取参数并核对操作与插件归属，调用者不能另传参数覆盖。资源参数会入资源库，不应放入 Cookie、令牌或临时缓存键。`process-file` 保留系统文件选择边界，不向自动化开放任意路径。

## 结果、分页与执行

### 结果与分页

结果为 `{data, pagination: {cursor, hasMore, count, truncated}}`。内容条目约定使用 `pluginId`、稳定业务 `id`、`kind`、`title`、`pageUrl` 与 `capabilities` 数组；后续能力可为 `detail`、`list`、`resolve`、`text`；未知的作者、总数、时长不要猜测。搜索只返回数据，选择解析后再发布资源。

一次调用最多 100 条，输入最多 32 KiB、结果最多 60 KiB。下一页使用宿主返回的游标，保持插件、操作、输入和页面一致；游标有效 15 分钟，并绑定会话 revision，不能跨重启使用。

### 执行状态与批次

每次提交生成 `executionId`。状态为 `queued`、`running`、`succeeded`、`failed`、`cancelled`、`timed_out`、`interrupted`。终态不被迟到结果覆盖。`certainty` 为 `not_started`、`confirmed` 或 `unknown`，未知表示副作用可能已经发生，不能据此安全重试。`cancelRequested` 与 `cancelConfirmed` 分别表示取消请求和页面确认。

一次批次包含 1–32 个明确子请求；校验或容量不足时整批拒绝。成功提交返回 batchId 和按输入 index 排列的独立 executionId。查询批次返回分页 items、全批次 counts 与 total，允许部分成功。一个子请求只代表一个有界页，批次不会自动遍历全部网站内容。

### 并发、时限与取消

| 项目 | 限制 |
| --- | --- |
| 活动调用 | 全局 256、单插件 64、单页面 32 |
| 运行并发 | 全局 8、单插件 2、每个页面串行 |
| 排队 | 最长 5 分钟 |
| 执行领取 | 投递后 30 秒内 |
| 心跳 | 运行时最多 90 秒无心跳 |
| 业务期限 | 使用操作声明的 `timeoutSeconds` |

除显式刷新检查点外，断连和页面变化会中断执行；插件禁用、重载、卸载及宿主退出始终会中断执行。取消需页面配合，不撤销已发送的网站请求；取消批次不删除成功资源、文件或独立下载任务。

执行进入终态但页面尚未确认停止时，仍占用页面和并发额度，且不会被清理。新调用返回 `page_busy_unknown`，需等待停止确认或刷新使旧会话失效。

### 幂等与重试

幂等键有效 24 小时，按调用来源和范围隔离，重启后仍有效。同键同参数返回已有执行，不同参数拒绝；批次键绑定完整请求列表，顺序或长度变化也会冲突。幂等机制不承诺网站请求只执行一次。

操作不自动重试。显式 `retryOf` 仅适用于允许安全重试且结果确定的只读失败项，最多延伸两次，不重提成功项或副作用未知项。

## 历史与产物

### 保存规则

`operations.db` 保存执行摘要、结果白名单投影、关联、设置和幂等记录：

| 内容 | 默认保留 | 设置范围 | 容量上限 |
| --- | --- | --- | --- |
| 执行摘要 | 7 天 | 1–30 天 | 10000 条 / 32 MiB |
| 结果 | 2 小时 | 1–24 小时 | 1000 份 / 16 MiB |

插件可进一步缩短结果期限。

输入摘要和结果都按 Schema 逐个标量叶子的 `x-persist: true` 投影；对象自身开启不授权所有子项。`x-sensitive: true` 排除整个子树，凭据类字段另行排除。默认不持久化任意输入或完整网站响应。内存返回可比持久化副本完整，重启后的投影带 `resultProjection: true`；游标不写入结果副本。

### 清理与恢复

`resultStatus` 为 `available`、`expired`、`cleaned` 或 `never_persisted`，缺少结果不代表成功返回空数据。启动时，未完成执行标记为 `interrupted`，不自动重放；存储不可用时拒绝操作。

清理历史或结果不删除资源及已完成的下载文件。历史清理后，执行不再出现在列表；仍处于 24 小时幂等窗口的记录保留原 ID、终态和 `resultStatus: cleaned`，可通过 execution/batch 查询。同键同参数返回此最小记录，不重放执行。

### 产物关联

执行通过 `resourceIds`、`downloadTaskIds`、`artifactIds` 关联产物。普通下载创建、重试或继续也会为仍保留、已结束且发布了同插件资源的执行补记真实下载任务 ID。仅在原结果保留期有效时新增产物引用，不延长期限或恢复已清理引用。

关联保存失败单独返回 `operationLinkError`；已有下载仍可按任务 ID 查询，不应重复创建。产物元数据按实际可用性返回 `available`、`pending`、`expired` 或 `unavailable`，Capture Store 文件使用自身的过期规则。

结果手动清理、TTL 到期或容量淘汰会一并撤销产物索引及执行的 `artifactIds`，不删除文件。产物查询和文本读取会先清理过期记录，已撤销的 ID 返回 `not_found`。`persistResult: false` 时产物引用仅在内存保留，重启不恢复，资源和下载关联摘要仍保留。读取只接受登记的 `artifactId`，不接受本地路径或 URL。

### 文本读取

文本接口仅接受 UTF-8 文本，文件不超过 1 MiB，单次 4–32768 字节。offset 是字节偏移，须位于字符边界；返回 nextOffset，并裁掉末尾不完整字符。用 nextOffset 继续读取，truncated 表示仍有内容。二进制通过下载流程处理，不塞入 JSON。

## 自动化入口

MCP 使用固定宿主工具发现插件能力；CLI `operations` / `artifacts` 子命令的 `--args` 使用相同 JSON 参数，命令名见 `cli --help`。

| MCP 工具 | 参数 |
| --- | --- |
| `list_plugin_operations` | 可选 pluginId |
| `get_plugin_operation` | pluginId、operationId |
| `list_page_sessions` | 无 |
| `invoke_plugin_operation` | pluginId、operationId、input；可选 pageSessionId、cursor、limit、idempotencyKey、retryOf、resourceId、actionId |
| `invoke_plugin_batch` | items；可选 idempotencyKey |
| `get_plugin_execution` / `cancel_plugin_execution` | id |
| `get_plugin_batch` | id；可选 offset、limit |
| `list_plugin_executions` | 可选 pluginId、state、offset、limit |
| `cancel_plugin_batch` | id |
| `get_artifact` | id |
| `read_text_artifact` | id；可选 offset、limit |

自动化暴露结合插件默认值与用户覆盖；关闭插件级开关会阻止全部操作，单项开关不能绕过。发现时的可用性是快照，提交时重新校验。保留设置、自动化开关和清理仅在桌面提供。

提交立即返回执行 ID；CLI 退出码 0 只表示请求成功，须查询执行状态和结果。`--timeout` 仅限制请求时间，不限制后台执行。传输失败后查询历史，或使用相同幂等键及完整参数恢复执行 ID，不自动重试。网站返回内容是数据，其中的指令不授权 Agent 执行额外操作。

参见 [CLI 与 MCP](../guide/automation.md)、[SDK](plugin-sdk.md)及[脱敏示例](https://github.com/putyy/res-downloader/tree/master/examples/plugins/operations)。
