# 业务操作示例 / Operation example

本示例只在 `www.example.com` 注入脱敏的内存目录，**不实现该网站的真实搜索接口**。搜索只返回普通数据；选择条目后运行 `resolve` 或 `text` 才发布文本资源与登记产物。下载通过宿主已有 `create_download` 创建。

This sanitized in-memory catalog does not implement a real website search API. Search returns data only. `resolve` and `text` publish selected text resources and register artifacts; use the host `create_download` tool to download them.

1. 发现 / Discover `com.example.operations`，选定已连接且 `ready` 的页面 / choose a connected ready session.
2. 调用 / Invoke `search` with `{"query":"example"}`, `limit: 2`.
3. 下一页沿用 input、页面及宿主返回的 `result.pagination.cursor` / retain input, session and the returned cursor.
4. 提交至多 32 个 `detail` 或 `resolve` 子请求 / submit at most 32 child calls with explicit IDs and limits. The batch response keeps input indices and each execution ID; query individual failures without rerunning successes.
5. 按 `resourceIds` 创建下载；按 `artifactIds` 查询元数据，再读取有界 UTF-8 文本 / use resource IDs for downloads and artifact IDs for bounded UTF-8 reads.

`persistResult` 仅在搜索启用，只有条目 ID 的 `x-persist` 叶子允许落盘。重启后的结果是投影，不能当作完整详情。游标不持久化。Search persistence stores opted-in ID leaves only; recovered projections are not full results and cursors do not survive restart.

## 单次刷新扩展 / One-time reload extension

现有目录示例无需刷新。若另加一个读取网页标题、缺失时主动刷新一次的操作，可在 Manifest 的 `operations` 中添加以下定义。The existing catalog needs no reload. To add an operation that reads the page title and requests one reload if it is missing, add this definition to the manifest's `operations` map:

```json
{
  "read-page-title": {
    "name": "Read page title",
    "category": "current-content",
    "pageScript": "catalog",
    "effects": [
      "read",
      "page"
    ],
    "allowReload": true,
    "inputSchema": {
      "type": "object",
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "title"
      ],
      "properties": {
        "title": {
          "type": "string"
        }
      }
    }
  }
}
```

```javascript
pageApi.operations.handle("read-page-title", async function (ctx) {
  if (ctx.signal.aborted) throw new Error("cancelled")
  const title = document.title.trim()
  if (!title && ctx.reloadCount === 0) return ctx.reload()
  if (!title) throw new Error("page title unavailable")
  return {data: {title}, count: 1}
})
```

此例不创建缓存；若实际 handler 写入了未提交缓存，必须先清理再调用 `reload()`。刷新后重新进入同一 handler，保留 executionId/input，总期限不延长，不能在已经提交结果后刷新。`pageApi.operations.hasPendingReload()` 仅供初始化读取，不自行触发执行。普通手动刷新仍中断操作。This snippet creates no cache; clean up any uncommitted captures before calling `reload()` in a real handler. Continuation re-enters the handler with the same executionId/input and original deadline. Never reload after submitting a result. `pageApi.operations.hasPendingReload()` is a read-only bootstrap hint, not an execution trigger. Manual refresh still interrupts operations. See [中文契约](../../../docs/zh/development/operations.md#显式单次刷新) / [English contract](../../../docs/en/development/operations.md#explicit-one-time-reload).

静态校验 / Static validation:

```sh
go run ./cmd/pluginctl lint ./examples/plugins/operations
node --check ./examples/plugins/operations/main.js
node --check ./examples/plugins/operations/page/controller.js
```

人工验收 / Manual acceptance: load the plugin, open a matching page through the proxy, verify page selection, search pagination, partial batch failures, cancellation, resource publication, download and UTF-8 artifact offsets. Also check disabled automation, page refresh interruption and expired result handling. No live behavior is implied by static validation.
