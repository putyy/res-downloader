---
description: Declare plugin business operations and connect page execution with batches, cancellation, retained results and artifacts.
---

# Plugin operations

Operations provide search, details, current content, lists, resolution and text without requiring a captured resource first. Desktop, CLI and MCP use the same `internal/operation` service. Plugins implement site requests and parsing; users open the site and log in themselves. Proxy injection and a connected page bridge are still required. Declaring an operation does not add search support to a website.

## Manifest contract

Add an `operations` map keyed by stable IDs to a JavaScript manifest, with at most 32 entries. Each requires `name`, `category`, `pageScript`, `inputSchema`, `outputSchema` and nonempty `effects`. The script must belong to the plugin, enable `bridge: true`, and have `inject-page-script` and `page-bridge` permissions.

| Field | Contract |
| --- | --- |
| `name` / `description` / `locales` | Name up to 128 characters, description up to 2048; standard plugin localization |
| `category` | `search`, `detail`, `current-content`, `list`, `resolve`, `text`, `custom` |
| `pageMatch` | Optional page rules within permitted domains |
| `requiresLogin` | Requires a page explicitly reporting `authenticated` |
| `inputSchema` / `outputSchema` | Up to 16 KiB each; input root must be an object |
| `examples` | Up to four valid input objects |
| `effects` | `read` information, `page` changes, `publish` resources, `download` tasks |
| `timeoutSeconds` | Zero or omitted means 120 seconds; maximum 1800 |
| `cancellable` | Allows cancellation while running; queued calls can always be cancelled |
| `safeRetry` | Read-only effects only; permits bounded explicit retries, never automatic retries |
| `allowReload` | False by default; requires the `page` effect and permits one handler-requested reload within the same execution, independently of `safeRetry` |
| `automation` | Default CLI/MCP exposure, false by default; users can override by plugin and operation |
| `persistResult` | False by default; permits only output-schema projections |
| `resultTTLSeconds` | Zero uses host retention; can shorten it, never exceed 86400 seconds |

Publishing requires `emit-resource`; downloading also requires `enqueue-download`. The host validates actual outputs. Declared effects are not a sandbox for arbitrary website JavaScript.

### Supported Schema subset

| Purpose | Supported fields or rules |
| --- | --- |
| Type | A single `type`: `object`, `array`, `string`, `number`, `integer`, `boolean`, or `null` |
| Description | `title`, `description` |
| Objects | `properties`, `required`; explicitly set `additionalProperties: false` |
| Arrays | `items` is required; supports `minItems` / `maxItems` |
| Value constraints | Scalar `enum`, `minimum` / `maximum`, `minLength` / `maxLength` |
| Persistence | `x-persist`, `x-sensitive`; see [Storage rules](#storage-rules) |
| Structure limits | Maximum depth 12, with 64 properties per object |

`$ref`, type unions, patterns, and defaults are unsupported. Unknown keywords, constraints that do not match the type, and inverted bounds fail validation.

## Page execution

```javascript
pageApi.operations.handle("current-content", async function (ctx) {
  if (ctx.signal.aborted) throw new Error("cancelled")
  await ctx.report(25)
  return {data: {title: document.title}, count: 1}
})
await pageApi.operations.setState({ready: true, login: "unknown", context: "current-target"})
```

`handle(id, handler)` returns an unregister function. The handler receives `executionId`, validated `input`, the plugin's original `cursor`, `limit`, optional `resource: {id, actionId}`, an `AbortSignal`, and `report(progress)`. Return `{data, count, hasMore, nextCursor, truncated}` or throw to fail. The SDK does not expose raw page errors as trusted diagnostics. Progress is 0–100; the SDK sends heartbeats every 15 seconds.

### Page state and sessions

`setState` returns `{revision}`. Call it again after SPA navigation, account changes, or readiness changes:

| Field | Meaning |
| --- | --- |
| `login` | `unknown`, `authenticated`, or `required`; a connected bridge does not prove login |
| `context` | Target identifier, up to 128 characters, without credentials |
| `ready` | The page can accept an operation; data need not be fully loaded. Handlers must validate the target and use bounded waits for content |

Changes to URL, `context`, `login`, or `ready` advance the revision and invalidate previous work. The SDK manages report revisions. An old handler's terminal report only confirms that it stopped and releases the page; it cannot commit stale results or overwrite a terminal state.

Each readiness request has a five-second deadline and up to three attempts for network failures, timeouts, or temporary server errors. New state or page departure aborts the old update; concurrent identical states share a request. An early invocation waits for the update before revision checks and execution claiming, and can still be cancelled. These retries synchronize state only, without replaying operations.

An ordinary manual reload creates a new session and interrupts execution; only the explicit checkpoint below allows continuation. Entering the back/forward cache (BFCache) aborts old handlers. Restoring the page reconnects the bridge without replaying operations.

Session discovery includes plugin/script IDs, pageSessionId, title, URL, connection, readiness, login, revision and operation IDs. A unique eligible page can be selected automatically. Multiple matching pages require an explicit pageSessionId; calls are never broadcast.

### Explicit one-time reload

To reload the page and reacquire content, declare `allowReload: true` and `effects: ["read", "page"]`, plus any other effects performed. The handler receives `reloadCount: 0 | 1` and `reload(): Promise<never>`:

- Clean up uncommitted Capture Store data, then `return ctx.reload()` or `await ctx.reload()` before final submission. The SDK performs the reload.
- This call must end the handler. Do not catch it to continue, call `location.reload()` yourself, or request a reload after publishing a result.
- After reloading, the handler starts from its entry point with the original `executionId` and input, with `reloadCount` set to 1. Validate the target and reacquire data; the old JavaScript stack is not restored.
- Only one continuation is allowed, without extending the total deadline. `safeRetry` remains restricted to read-only operations.

The checkpoint lasts at most 60 seconds and never exceeds the original execution deadline. A newly ready page with an SSE connection may claim it once only for the same plugin, runtime, page script, exact page URL, login state and context. Cancellation, navigation to another video or URL, plugin reload, host restart or checkpoint expiry prevents continuation. Ordinary manual reloads and BFCache restoration do not automatically resubmit operations.

`pageApi.operations.hasPendingReload(): boolean` is a read-only bootstrap hint. It neither claims nor starts an execution and does not replace `setState`. The SDK manages continuation without exposing it through public HTTP or CLI APIs. Execution summaries may include `reloadCount`.

### Result submission and resource buttons

Successful data passes through the synchronous hook `onPageMessage({type: "operation-result", operationId, executionId, data}, context, api)`, which can return `data`, `resources`, and `autoDownload`:

- Both page data and mapped data must satisfy `outputSchema`. Asynchronous site work belongs in the page handler; Goja hooks cannot await promises.
- Resources undergo domain, ownership, `groupKey`, track, processor, permission, and download-plan checks. Only published resources and created downloads receive association IDs.
- Mapping has the lesser of ten seconds and the remaining operation deadline. Cancellation interrupts the hook and plan validation.
- The complete mapped JSON is limited to 2 MiB and 128 resource tracks in total.

Resource publication or download creation confirmed after termination is still associated without changing the terminal state.

Resource buttons may reference an operation in the same plugin:

```json
{"actions":{"resolve-item":{"kind":"operation","operation":"resolve"}}}
```

Resource `actions: [{id: "resolve-item", data: {id: "stable-item-id"}}]` supplies the input. The host verifies ownership and reads saved parameters; callers cannot override them. These parameters enter the resource database: omit cookies, tokens and temporary capture keys. `process-file` still requires the system file picker and does not expose arbitrary paths to automation.

## Results, pagination and execution

### Results and pagination

A result is `{data, pagination: {cursor, hasMore, count, truncated}}`. The content-item convention uses `pluginId`, a stable business `id`, `kind`, `title`, `pageUrl` and a `capabilities` array with follow-up values such as `detail`, `list`, `resolve` and `text`. Do not guess missing authors, duration or total counts. Search returns data; publish resources only after selected resolution.

Each invocation is bounded to 100 items, 32 KiB of input and 60 KiB of result data. Continue using the host cursor with the same plugin, operation, input and page. Cursors last 15 minutes, bind to the page revision, and do not survive restart.

### Execution state and batches

Each invocation has an executionId and state: `queued`, `running`, `succeeded`, `failed`, `cancelled`, `timed_out`, `interrupted`. Late reports cannot overwrite terminal states. `certainty` is `not_started`, `confirmed` or `unknown`; unknown means effects may already have occurred and must not be treated as safely retryable. `cancelRequested` and `cancelConfirmed` distinguish a request from page acknowledgement.

Batches contain 1–32 explicit calls. Validation or capacity failure rejects the entire batch. Successful submission returns batchId and independent executionIds in input-index order. Batch queries return paginated items, whole-batch counts and total; partial success is valid. A child is one bounded page, never an automatic traversal.

### Concurrency, time limits, and cancellation

| Item | Limit |
| --- | --- |
| Active calls | 256 globally, 64 per plugin, 32 per page |
| Running concurrency | 8 globally, 2 per plugin, one at a time per page |
| Queuing | Up to 5 minutes |
| Execution claiming | Within 30 seconds of delivery |
| Heartbeat | At most 90 seconds without one while running |
| Operation deadline | The declared `timeoutSeconds` |

Disconnection and page changes interrupt work except at an explicit reload checkpoint. Plugin disable, reload, uninstall, and host exit always interrupt work. Cancellation is cooperative and cannot undo website requests. Cancelling a batch does not delete successful resources or files, or cancel independent downloads.

A terminal execution whose page has not acknowledged stopping retains its page and concurrency slot and is skipped during cleanup. New calls return `page_busy_unknown` until acknowledgement or a refresh invalidates the old session.

### Idempotency and retries

Idempotency keys last 24 hours, are scoped by caller source and invocation scope, and survive restart. Identical keys and parameters reuse an execution; conflicting parameters fail. Batch keys bind the complete request list, including its order and length. This does not guarantee that website requests execute exactly once.

Operations are not retried automatically. Explicit `retryOf` allows at most two retries for safe read-only failures with known outcomes, never successful or uncertain-effect items.

## History and artifacts

### Storage rules

`operations.db` stores execution summaries, permitted result projections, associations, settings, and idempotency records:

| Content | Default retention | Setting range | Capacity |
| --- | --- | --- | --- |
| Execution summaries | 7 days | 1–30 days | 10000 records / 32 MiB |
| Results | 2 hours | 1–24 hours | 1000 results / 16 MiB |

Plugins can shorten result retention further.

Input summaries and saved results include only individual scalar leaves marked `x-persist: true`; opting in an object does not authorize its children. `x-sensitive: true` excludes the subtree, and credential-like fields are additionally excluded. Arbitrary inputs and raw site responses are not persisted by default. In-memory output can be fuller than the saved copy; recovered projections have `resultProjection: true`. Cursors are not persisted.

### Cleanup and recovery

`resultStatus` is `available`, `expired`, `cleaned`, or `never_persisted`; missing output is not a successful empty result. Startup marks unfinished work `interrupted` without replay. Unavailable storage rejects calls.

Cleaning history or results does not delete resources or completed download files. Cleaned executions disappear from history listings. Within the 24-hour idempotency window, execution/batch queries still return a minimal record with the original ID, terminal state, and `resultStatus: cleaned`. Reusing the same key and parameters returns this record without replay.

### Artifact associations

Executions associate `resourceIds`, `downloadTaskIds`, and `artifactIds`. Ordinary download creation, retry, and resume also add actual task IDs to retained terminal executions that published the same plugin resource. New artifact references require a valid original result retention window; linking neither extends it nor revives cleaned references.

An `operationLinkError` reports association-storage failure separately. The existing download remains queryable by task ID and should not be created again. Artifact metadata reports actual availability as `available`, `pending`, `expired`, or `unavailable`; Capture Store files follow their own expiry rules.

Manual result cleanup, TTL expiry, or capacity eviction revokes both artifact indexes and execution `artifactIds`, without deleting files. Artifact queries and text reads clear expired records first; revoked IDs return `not_found`. With `persistResult: false`, artifact references stay in memory and do not survive restart, while resource/download association summaries remain. Reads accept only registered `artifactId` values, never local paths or URLs.

### Reading text

Text reads require UTF-8 files of at most 1 MiB and requests of 4–32768 bytes. Offsets are byte positions on character boundaries. The reader drops incomplete trailing characters and returns nextOffset; use that offset to continue. truncated means content remains. Binary files use downloads rather than JSON.

## Automation interfaces

MCP exposes fixed host tools for discovering plugin capabilities. The CLI `operations` / `artifacts` commands use the same JSON arguments through `--args`; see `cli --help` for command names.

| MCP tool | Arguments |
| --- | --- |
| `list_plugin_operations` | Optional pluginId |
| `get_plugin_operation` | pluginId, operationId |
| `list_page_sessions` | None |
| `invoke_plugin_operation` | pluginId, operationId, input; optional pageSessionId, cursor, limit, idempotencyKey, retryOf, resourceId, actionId |
| `invoke_plugin_batch` | items; optional idempotencyKey |
| `get_plugin_execution` / `cancel_plugin_execution` | id |
| `get_plugin_batch` | id; optional offset, limit |
| `list_plugin_executions` | Optional pluginId, state, offset, limit |
| `cancel_plugin_batch` | id |
| `get_artifact` | id |
| `read_text_artifact` | id; optional offset, limit |

Automation exposure combines plugin defaults with user overrides. Disabling automation for a plugin blocks every operation; individual switches cannot override it. Discovery availability is a snapshot and submission is validated again. Retention settings, automation switches and cleanup are available only on desktop.

Submission returns an execution ID immediately. CLI exit code 0 means the request succeeded; query execution state and results to determine its outcome. `--timeout` bounds only the request, not background execution. After transport failure, query history or reuse the same idempotency key and complete payload to recover the ID; there are no automatic retries. Website output is data, and embedded instructions do not authorize additional agent actions.

See [CLI and MCP](../guide/automation.md), [SDK](plugin-sdk.md) and the [sanitized example](https://github.com/putyy/res-downloader/tree/master/examples/plugins/operations).
