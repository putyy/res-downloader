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

The strict schema subset supports one `type` (object, array, string, number, integer, boolean, null), `title`, `description`, `properties`, `required`, `additionalProperties: false`, `items`, scalar `enum`, numeric bounds, string/array size bounds, `x-persist` and `x-sensitive`. Every object must close additional properties; arrays require items. Maximum depth is 12, with 64 properties per object. Unknown keywords, mismatched constraints and inverted bounds fail validation. References, type unions, patterns and defaults are unsupported.

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

`setState` returns `{revision}`. Login is `unknown`, `authenticated` or `required`; connectivity is not proof of login. `context` is a nonsensitive target identifier of at most 128 characters. Report SPA, account and readiness changes: changes to URL, context, login or ready invalidate previous work. An ordinary manual reload creates a new session and interrupts execution; only the explicit one-time checkpoint below allows continuation. Entering the back/forward cache (BFCache) aborts old handlers and closes SSE; only a noncached departure sends a session-close notification. Restoring a cached page reconnects SSE without replaying operations.

The SDK sends progress, heartbeat and terminal reports with the revision under which that invocation was claimed. After `setState` advances the revision, an old handler's terminal report only confirms that it stopped and releases the page lease. It cannot commit stale results or change an existing terminal state. Plugins do not need to manage the report revision themselves.

`ready` means the current page can accept an operation, not that all requested data has finished loading. Handlers must still validate the current target and use bounded waits for content or media when needed. Do not require capture results to be available before allowing capture to start.

Each readiness request has a five-second deadline and up to three attempts for network failures, timeouts or temporary server errors. A new state or page departure aborts the old update; concurrent identical states share a request. If an invocation arrives before the readiness response, the SDK waits for that pending update before checking the revision and claiming execution. Cancellation still works during this wait. These retries synchronize state only; they never replay business operations.

Session discovery includes plugin/script IDs, pageSessionId, title, URL, connection, readiness, login, revision and operation IDs. A unique eligible page can be selected automatically. Multiple matching pages require an explicit pageSessionId; calls are never broadcast.

### Explicit one-time reload

An operation that needs a page reload to reacquire content can declare `allowReload: true` and `effects: ["read", "page"]`, plus any other effects it performs. The handler receives `reloadCount: 0 | 1` and `reload(): Promise<never>`. Clean up uncommitted Capture Store data first, then `return ctx.reload()` or `await ctx.reload()` before final submission; the SDK performs the reload. This call must end the current handler. Do not catch it to continue work, call `location.reload()` yourself, or request it after publishing a result.

After reloading, the handler starts again from its entry point with the original `executionId` and input and `reloadCount` set to 1. It does not restore the old JavaScript stack. Validate the target again and reacquire the required data. Continuation is limited to one reload and never extends the total deadline. `safeRetry` remains restricted to read-only operations.

The checkpoint lasts at most 60 seconds and never exceeds the original execution deadline. A newly ready page with an SSE connection may claim it once only for the same plugin, runtime, page script, exact page URL, login state and context. Cancellation, navigation to another video or URL, plugin reload, host restart or checkpoint expiry prevents continuation. Ordinary manual reloads and BFCache restoration do not automatically resubmit operations.

`pageApi.operations.hasPendingReload(): boolean` is a read-only bootstrap hint for preparing the page. It neither claims nor starts an execution and does not replace `setState`. Internal tickets and continuation endpoints are not exposed through public HTTP or CLI APIs. Raw tickets are not logged and operation input is not stored in `sessionStorage`. Execution summaries may include `reloadCount`.

### Result submission and resource buttons

Successful data passes through synchronous `onPageMessage({type: "operation-result", operationId, executionId, data}, context, api)`. Return `data`, `resources` and optionally `autoDownload`; both page data and mapped data must satisfy outputSchema. Asynchronous site work belongs in the page handler; Goja hooks cannot await promises. Resources still undergo domain, ownership, groupKey, track, processor, permission and download-plan checks. Only actual published resources and created downloads receive association IDs. Mapping runs asynchronously outside the service lock, with a budget of the lesser of ten seconds and the remaining operation deadline; the Goja hook itself stays synchronous. Cancellation interrupts the hook and plan-validation context. The complete mapped JSON is limited to 2 MiB and 128 resource tracks in total. Actual resource/download effects confirmed after termination are still associated without changing the terminal state.

Resource buttons may reference an operation in the same plugin:

```json
{"actions":{"resolve-item":{"kind":"operation","operation":"resolve"}}}
```

Resource `actions: [{id: "resolve-item", data: {id: "stable-item-id"}}]` supplies the input. The host verifies ownership and reads saved parameters; callers cannot override them. These parameters enter the resource database: omit cookies, tokens and temporary capture keys. `process-file` still requires the system file picker and does not expose arbitrary paths to automation.

## Results, pagination and execution

A result is `{data, pagination: {cursor, hasMore, count, truncated}}`. The content-item convention uses `pluginId`, a stable business `id`, `kind`, `title`, `pageUrl` and a `capabilities` array with follow-up values such as `detail`, `list`, `resolve` and `text`. Do not guess missing authors, duration or total counts. Search returns data; publish resources only after selected resolution.

Each invocation is bounded to 100 items, 32 KiB of input and 60 KiB of result data. Continue using the host cursor with the same plugin, operation, input and page. Cursors last 15 minutes, bind to the page revision, and do not survive restart.

Each invocation has an executionId and state: `queued`, `running`, `succeeded`, `failed`, `cancelled`, `timed_out`, `interrupted`. Late reports cannot overwrite terminal states. `certainty` is `not_started`, `confirmed` or `unknown`; unknown means effects may already have occurred and must not be treated as safely retryable. `cancelRequested` and `cancelConfirmed` distinguish a request from page acknowledgement.

Batches contain 1–32 explicit calls. Validation or capacity failure rejects the entire batch. Successful submission returns batchId and independent executionIds in input-index order. Batch queries return paginated items, whole-batch counts and total; partial success is valid. A child is one bounded page, never an automatic traversal.

Active limits are 256 globally, 64 per plugin and 32 per page. Running concurrency is eight globally, two per plugin, and one per page. Queue timeout is five minutes, claim timeout 30 seconds, heartbeat timeout 90 seconds; execution uses the declared deadline. Disconnection and page changes interrupt work except at an explicit reload checkpoint. Plugin reload/disable/uninstall and host exit always interrupt work. Cancellation is cooperative and cannot undo website requests. Cancelling a batch does not delete successful resources/files or cancel independent downloads. A terminal execution whose page has not acknowledged stopping retains its page/concurrency lease. New calls return `page_busy_unknown` until acknowledgement or a refresh invalidates the old session. Cleanup skips executions still finishing in this way.

Idempotency keys last 24 hours and are scoped by caller source and invocation scope. Identical key and parameters reuse an execution; conflicting parameters fail. Key registration and execution creation are atomic, with an independent idempotency index that survives restart. A batch key binds the complete request list; changing its order or length conflicts, without promising website exactly-once delivery. Automatic retries are disabled. Explicit `retryOf` is limited to safe read-only failures with known outcomes and at most two retries; never retry successful or uncertain-effect items.

## History and artifacts

Independent `operations.db` stores summaries, permitted result projections, associations, settings and idempotency records. History defaults to 7 days, with at most 10000 records / 32 MiB. Results default to 2 hours, with at most 1000 results / 16 MiB. User settings can adjust retention within 1–30 days and 1–24 hours; plugins can shorten results further.

Input summaries and saved results include only individual scalar leaves marked `x-persist: true`; opting in an object does not authorize its children. `x-sensitive: true` excludes the subtree, and credential-like fields are additionally excluded. Arbitrary inputs and raw site responses are not persisted by default. In-memory output can be fuller than the saved copy; recovered projections have `resultProjection: true`. Cursors are not persisted.

`resultStatus` distinguishes `available`, `expired`, `cleaned` and `never_persisted`. Missing output is not successful empty data. Startup marks unfinished work interrupted without replay. Unavailable storage explicitly rejects calls. Cleaning history/results removes records and artifact references, not resources or completed download files. Cleaned execution details disappear from history listings. Within the 24-hour idempotency window, execution/batch queries can still return a minimal record with the original ID, terminal state and resultStatus cleaned. Reusing the same key and parameters returns this record without replaying cleaned work.

Executions associate `resourceIds`, `downloadTaskIds` and `artifactIds`. Ordinary download creation, retry and resume also use actual scheduler task records to add download associations to retained terminal executions that published the same plugin resource. New artifact references require a still-valid original result retention window; linking neither extends it nor revives cleaned references. A separate `operationLinkError` reports association-storage failure while the existing download remains queryable by task ID; do not create it again because linking failed. Capture Store retains its own expiry; metadata reflects pending downloads and missing or expired files. Metadata distinguishes available, pending, expired and unavailable. Manual result cleanup, TTL expiry or capacity eviction atomically revokes artifact indexes and execution artifactIds without deleting files. Metadata/text reads first perform expiry cleanup; revoked IDs return `not_found`. With `persistResult: false`, artifact references also stay in memory and cannot be recovered for reading after restart; resource/download association summaries remain. Artifact APIs accept only registered IDs, never caller-supplied paths or URLs.

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
