---
description: Develop res-downloader plugins with the Manifest, permissions, JavaScript API, page scripts, resource model, download plans, WASM processors, and packaging workflow.
---

# Plugin Development

This guide covers plugin directories, the Manifest, permissions, runtime APIs, resource models, download plans, WASM processors, debugging, testing, and publishing. For installing or managing plugins as a user, see [Plugin Management](../guide/plugin-management.md).

res-downloader uses a versioned plugin protocol. Plugins receive network observations and report resources through structured data. They do not directly access Go objects, the filesystem, shell, or arbitrary network interfaces.

The repository's `plugins/` directory is a local workspace for cloning and debugging independent plugin repositories. Its sources are not committed to the host repository, automatically loaded, or packaged with the app by default. Official examples live in `examples/plugins/`; bundled plugins shipped with the app live in `internal/plugin/bundled/`.

For your first plugin, read “Choose a plugin type → Quick start → Manifest → Permissions → Resource model → JavaScript hooks or Declarative plugins → Validation and offline replay.” Page scripts, download plans, and WASM are advanced capabilities to use as needed.

Supported capabilities include:

- `declarative`: extract single-track resources from individual JSON responses using JSON/YAML and a restricted JSON Path syntax.
- `javascript`: handle complex JSON, cross-request correlation, custom download plans, and resource refresh.
- Plugin-provided WASM: run private decryption/transformation algorithms on download inputs or outputs.
- Resource actions: process local files or send plugin-defined arguments to matching page scripts.
- Host downloads: plain HTTP and HLS, plus audio/video merging, remuxing, audio extraction, and live recording with FFmpeg configured.

## Choose a plugin type

| Scenario | Recommendation |
| --- | --- |
| One JSON response directly contains the title and download URL | `declarative` |
| Conditional logic, complex object traversal, or multiple qualities | `javascript` |
| Video and audio arrive in separate requests and must be merged by content ID | `javascript` + correlation APIs |
| Links expire and must be resolved again before download | `javascript` + `refreshResource` |
| Private decryption or byte transformations after downloading | JavaScript + plugin WASM |
| Player data is available only in the page runtime | JavaScript + page scripts; use only when ordinary observations are insufficient |

Choose the simplest implementation that works. If response JSON is enough, do not request page script, response modification, or advanced FFmpeg permissions.

## Quick start

### 1. Copy an example

Run from the repository root:

```bash
mkdir -p ./plugins
cp -R examples/plugins/javascript-basic ./plugins/com.example.my-plugin
```

For a simple JSON API, copy `declarative-basic`. The WASM example is `wasm-xor`. You can also interactively scaffold a JavaScript plugin with the project CLI:

```bash
go run main.go plugin create
```

Pressing Enter accepts the default parent directory `./plugins`, plugin ID, and display name `com.example.my-plugin`, creating `./plugins/com.example.my-plugin`. Enter different values for the parent directory, ID, and name if needed, confirming each with Enter. EOF, such as Ctrl+D on an empty input, exits without creating files. You can also provide arguments directly; the first is the full target directory:

```bash
go run main.go plugin create ./plugins/com.example.my-plugin com.example.my-plugin "Example Video"
```

The scaffold includes `plugin.json`, `main.js`, `README.md`, `.gitignore`, and an empty `fixtures/` directory. `.gitignore` ignores `.idea` and `.vscode` by default. The command refuses to overwrite a nonempty target directory.

New plugins include an **Enable logging** setting named `enableLog`, with type `boolean` and default `false`. Manually created plugins should also declare it with Chinese and English labels.

Choose either copying an example or using the CLI. This guide uses `./plugins/com.example.my-plugin`; replace later paths if you choose another directory.

### 2. Edit the Manifest and entry point

At minimum, change the plugin `id`, name, version, allowed domains, and matching rules. Use a stable reverse-domain ID such as `com.example.video`. Local and community plugins cannot use the host-reserved `builtin.` or `official.` prefixes, including case variants. Official store identity is determined by the index source.

JavaScript plugins implement `onObservation` in `main.js`. Start by matching one response and emitting one resource, then add correlation, refresh, or download processing incrementally.

### 3. Prepare a sanitized fixture

Save a representative request/response as a fixture, removing cookies, Authorization headers, accounts, private signatures, and real user content. Keep only the fields required for the plugin's decisions.

### 4. Static validation and offline replay

```bash
go run main.go plugin lint ./plugins/com.example.my-plugin
go run main.go plugin replay ./plugins/com.example.my-plugin ./plugins/com.example.my-plugin/fixtures/video.json
```

`lint` checks the directory, Manifest, entry files, and permission dependencies. `replay` executes a fixture without starting the proxy.

Project maintainers use the internal command `plugin lint-bundled <directory>` to validate bundled preinstalled plugins. It accepts only IDs present in the application image whose directory contents exactly match the embedded version. Community plugins always use ordinary `lint` and cannot use this command to claim an `official.` prefix.

To update a bundled official plugin, run `go run main.go plugin sync-bundled <plugin-directory>`. The command validates it and replaces the old snapshot with the same ID under `internal/plugin/bundled/`, using the source directory's name. No manual copying is needed.

### 5. Package and install

```bash
go run main.go plugin pack ./plugins/com.example.my-plugin
```

The default output is `<plugin-directory>/dist/plugin.zip`. You can append a custom output path. The packer excludes `.git/`, `.idea/`, `.vscode/`, `dist/`, `tests/`, the output file itself, and `.gitignore`, `.DS_Store`, `README.md`, and `LICENSE`. These exclusions are independent of Git: the command does not read `.gitignore` and does not prevent committing `dist/plugin.zip` to the plugin repository. Select the generated ZIP under **Plugins**, review its permissions, and install. During development, you can also place the plugin directory under `plugins` in the user data directory and click **Reload**.

Putyy official plugins can be installed directly from local ZIPs. The Manifest author URL must point to a repository under `github.com/putyy`; the installed plugin remains labeled **Official**. Other local plugins are labeled **Community** and cannot use `official.*` IDs.

Keep your own JavaScript tests under the plugin repository's root `tests/`, preferably named `*.test.js`. Tests are excluded from the package; sanitized data for `plugin replay` stays in `fixtures/`.

## Development requirements

- **Least privilege**: declare only the domains and capabilities you need; avoid `*` domains.
- **Minimal bodies**: narrow `match`, explicitly use `readBody: false` when bodies are unnecessary, and choose the smallest sufficient `bodyLimit`.
- **Sensitive data**: do not log or report cookies, Authorization headers, accounts, administrator passwords, or long-lived credentials. Use `nonPersistentHeaders` for headers that should not be saved.
- **Reliable correlation**: correlate requests using content IDs, track IDs, or explicit aliases, never by guessing from arrival order.
- **Understandable failures**: return explicit refresh states for expired URLs. Keep incomplete resources non-downloadable rather than generating invalid files that appear successful.
- **Untrusted page messages**: page scripts share the website's environment. Validate types, lengths, and business fields in every bridge message.
- **Lawful use**: plugins must not bypass access controls or unauthorized DRM. Fixtures and logs must not contain other people's private data.

## Plugin directories and loading

Each plugin has its own directory:

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

A Manifest can be `plugin.json`, `plugin.yaml`, or `plugin.yml`. In a ZIP, it must be at the archive root or inside the single top-level directory. Entry and asset files must use paths relative to the plugin directory. Symlinks, escaping paths, and oversized files are rejected.

Plugins load once on app startup, without periodic scans. Use **Reload** during development. Enabling/disabling plugins and saving their settings also refreshes the active plugin set. The directory name should match the Manifest `id`.

App upgrades overwrite official plugins. To customize one, copy it with a new plugin ID.

## Manifest

The Manifest file can be `plugin.json`, `plugin.yaml`, or `plugin.yml`.

For JSON, use the [Manifest Schema and editor declarations](plugin-sdk.md) for field assistance. The app's `plugin lint` result is always authoritative.

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

Main fields:

| Field | Required | Description |
| --- | --- | --- |
| `id` | Yes | Up to 64 characters: letters, digits, dots, hyphens, and underscores only. External plugins cannot use `builtin.` or `official.` prefixes; comparison is case-insensitive |
| `name` | Yes | Plugin name shown when no localized text is available |
| `author` | No | `name` appears on the plugin card; `url` must be HTTP/HTTPS |
| `version` | Yes | Semantic version, for example `1.2.0` |
| `apiVersion` | Yes | Must currently be `1` |
| `runtime` | Yes | `javascript` or `declarative` |
| `entry` | For JavaScript | A `.js` entry inside the plugin directory, up to 1 MiB |
| `priority` | No | Higher values run first; multiple plugins may process the same request |
| `permissions` | Yes | Allowed domains, capabilities, and body limit |
| `match` | Yes | Request/response matching rules; an empty array matches all permitted stages within allowed domains |
| `locales` | No | Translated plugin names and descriptions |
| `resourceKinds` | No | Site-specific resource kinds and display labels |
| `settingsSchema` | No | Plugin setting structure, defaults, and form hints |
| `pageScripts` | No | Scripts injected by the host into matching HTML pages |
| `extractors` | For declarative | Rules extracting resources from JSON bodies |
| `processors` | No | Plugin-provided WASM processors |
| `actions` | No | Host-rendered local file processing or page command actions |
| `requires` | No | Optional host tool requirements, such as `ffmpeg: ">=6.0"` |

In `match`, `host`, `path`, and the full `url` support `*` wildcards; `method` is case-insensitive. `contentTypes` matches the response Content-Type, and `readBody` determines whether a matching rule needs the body. Set `readBody: false` explicitly if only the URL or headers are needed.

`resourceKinds` appear among the capture types on the main page. Users can filter broadly by stable `primaryType` or precisely by the plugin's `kind`.

`settingsSchema` provides basic validation for `string`, `number`, `integer`, `boolean`, `object`, `array`, and `enum`. Basic types generate forms in plugin management; complex structures can still use the advanced JSON editor.

The host interface currently supports `zh` (Simplified Chinese, the default locale) and `en` (English). Use these keys consistently in plugin `locales`, settings `x-locales`, and enum `x-enumLabels`. There is no need to duplicate identical text under `zh-CN`.

Localized entries are resolved in this order: current full locale, base language, `en`, then the first available entry. Missing plugin names fall back to the top-level `name`. Region-specific keys are supported, but add them only when the text differs and keep base-language entries. Fallback is one-way: `zh-CN` can resolve to `zh`, but the host's `zh` does not search for `zh-CN`. With only `zh-CN` and `en`, the Chinese interface will show English text.

Use `x-locales` for setting names and descriptions and `x-enumLabels` for enum option labels:

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

## Permissions

| Capability | Purpose | Prerequisites or notes |
| --- | --- | --- |
| `observe-request` | Receive request URL, headers, and other metadata | Restricted to `permissions.domains` and matching rules |
| `read-request-body` | Read matching request bodies | Requires `observe-request`; request only when necessary |
| `intercept-request` | Return locally synthesized responses | Requires `observe-request`; changes page behavior |
| `observe-response` | Receive response status, headers, and other metadata | Basic permission for normal resource discovery |
| `read-response-body` | Read matching response bodies | Requires `observe-response`; bounded by `bodyLimit` |
| `modify-response` | Modify matching responses | Requires `observe-response`; truncated bodies cannot be modified |
| `emit-resource` | Report resources | Output is still validated by the host |
| `process-download` | Invoke WASM processors declared by this plugin | Cannot select arbitrary local files or modules |
| `media.basic` | Use controlled media operations such as mux, remux, and audio extraction | Requires compatible user-configured FFmpeg |
| `media.ffmpeg` | Use the advanced FFmpeg argument-array interface | No shell; requires FFmpeg |
| `media.ffmpeg.network` | Allow FFmpeg to read plugin-provided HLS/live URLs | Download URLs must be valid HTTP/HTTPS; this is a sensitive permission |
| `inject-page-script` | Inject scripts into matching HTML pages | Target domains must undergo TLS interception and permit safe injection |
| `page-bridge` | Exchange JSON between page scripts and plugin runtime, or receive user-triggered `page-command` actions | Requires `inject-page-script` |
| `capture-response-body` | Cache Range responses actually read by the browser, or accept media segments captured by page scripts | Requires `observe-response`; page segments also require `inject-page-script` and `page-bridge` |
| `enqueue-download` | Automatically create downloads after a page message reports resources | Requires `page-bridge` and `emit-resource`; sensitive because it writes to the download directory |

Bodies reach plugins only when domain, rule, and read-permission checks all pass. Snapshots exceeding `bodyLimit` are marked `truncated`; truncated responses cannot be modified.

Each JavaScript hook call uses an isolated runtime. Scripts are limited to 1 MiB and each execution to 5 seconds, including runtime initialization, hook execution, and result export. The outer call allows up to 10 seconds including time waiting for a concurrency slot; request cancellation also interrupts JavaScript. A plugin can have at most 4 concurrent executions. An execution still running after timeout continues to occupy its slot until it actually exits. Plugins also have slow-call metrics and circuit breaking for consecutive failures. There is no Node.js, Promise awaiting, `fetch`, filesystem, or system-command API. Cross-request state must use host correlation APIs rather than JavaScript globals.

## Page scripts and the bidirectional message bridge

JavaScript plugins can declare `pageScripts` for the host to inject into the target page's main world. The MITM proxy performs injection, so current interception rules must already capture the target domain. Injection does not go through `onObservation` or consume its `bodyLimit` or 5-second Goja hook budget. Only `document-start` is supported. `frames` can be `top` (default) or `all`.

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

Each entry must be a regular `.js` file inside the plugin directory, no larger than 256 KiB. A plugin can declare at most 8 entries. The host scans at most the first 256 KiB of HTML and prefers an existing CSP nonce. It preserves the original response if there is no `<head>`, the response is not uncompressed `text/html`, or CSP prevents safe inline injection. The host never removes or relaxes the site's CSP.

The page entry runs inside an async function with a local `pageApi`:

```javascript
pageApi.onMessage(function (message) {
  if (message.type === "probe") runProbe(message.url)
})

var result = await pageApi.send({type: "player-ready", data: collectPlayerData()})
```

Page scripts with `capture-response-body` can also use the same page session token to write binary media segments into the host's existing Capture Store:

```javascript
await pageApi.capture.start("video:123:video")
await pageApi.capture.write("video:123:video", arrayBufferOrTypedArray)
await pageApi.capture.complete("video:123:video")
// On cancellation or failure:await pageApi.capture.abort("video:123:video")
```

`start` resets a cache with the same key; `write` appends bytes in call-completion order; `capture-file` can read it only after `complete`; `abort` immediately discards an incomplete cache. The page is responsible for timeline ordering and deduplication. The host does not parse private site streaming protocols. Each binary segment is limited to 32 MiB. A page session can use up to 4 capture keys simultaneously and write up to 16 GiB in total. Access remains restricted by page Origin, a random session token, plugin permissions, and plugin scope; neither the filesystem nor arbitrary cache keys are exposed to websites.

With the bridge enabled, page-to-plugin messages use same-origin POST and plugin-to-page messages use SSE. The proxy answers internal URLs directly without forwarding them to the website. Every page load receives its own `pageSessionId`; closing the page or reloading, disabling, or uninstalling the plugin invalidates the session.

Plugins handle page messages with a synchronous top-level hook:

```javascript
function onPageMessage(message, context, api) {
  if (message.type !== "player-ready") return {ok: false, error: "unsupported message"}

  api.page.send(context.pageSessionId, {type: "probe", url: message.data.url})
  api.upsert(/* ResourceCandidate */)
  return {ok: true, data: {accepted: true}}
}
```

Plugins declaring `enqueue-download` can return `autoDownload: true` from a page message. The host creates tasks only for resources in the same result that passed validation, were successfully published, and have stable `groupKey` values, up to 4 per call. Missing permissions, incomplete resources, or invalid plans prevent queuing:

```javascript
return {
  ok: true,
  resources: [resource],
  autoDownload: true
}
```

`context` contains `pageSessionId`, `scriptId`, `pageUrl`, `origin`, and the current effective plugin `settings`. Settings are passed only to the host-side `onPageMessage`, never injected into the website or included in `api.page.sessions()`. A plugin may answer page initialization messages with only the necessary nonsensitive settings, such as button visibility. `pageUrl` is the URL when the session was created; the page script must still verify the current business target after SPA navigation. Plugins can also use `api.page.broadcast(filter, message)` and `api.page.sessions(filter)`, available only with `page-bridge`. Ordinary messages and replies must be JSON, up to 64 KiB each; large media data must use `pageApi.capture.write`. Each plugin can have up to 32 active page sessions, each with queue, connection, and rate limits.

Manifest `page-command` resource actions let users send commands from the app's resource list to a specified bridge-enabled page script. The host rereads `action.data` from the saved resource instead of accepting frontend-supplied custom arguments. Messages use a fixed envelope:

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

The host generates a random `requestId`, which the page must echo in asynchronous results. Before delivery, the host removes expired idle sessions. Commands go only to sessions for the same plugin whose script matches the action's `pageScript`, has `bridge: true`, and currently has an SSE connection. The action fails immediately if no matching page is connected, the message exceeds 64 KiB, or all page queues are full. On success, the local API returns `requestId`, `pageScriptId`, and `delivered`, the number of sessions successfully queued. Queuing does not prove that a page received or completed the action; it may disconnect later. The page must report business results using `requestId`. Multiple pages may receive the same command, so plugins must verify the target by its business ID and use `requestId` to prevent duplicate execution.

Page scripts and website code share the main world, so website code could observe or imitate bridge requests. Treat all page messages as untrusted. By default, the bridge grants no access to files, shell, databases, the downloader, or other plugins. `enqueue-download` is an explicit exception: it can only queue validated resources just published by the current plugin, without exposing file paths or arbitrary task controls.

## Resource model

Plugins report logical resources. A normal file usually has one track. Sites with separate audio and video can provide multiple `video`, `audio`, or `subtitle` tracks and quality variants.

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
    // Only affects persisted copies in resources.db/tasks.db; this session can still download and preview.
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

Resource fields:

- `primaryType`: stable main type; one of `video`, `audio`, `image`, `document`, `archive`, `collection`, or `other`.
- `kind`: required plugin-specific kind, not limited by a core enum.
- `traits`: composable features such as `encrypted`, `multiTrack`, `segmented`, `streaming`, `live`, `gallery`, and `mergeRequired`. Use `plugin.id:name` for private traits.
- `groupKey`: stable logical resource key within the plugin. Required for incremental merging across requests.
- `parentGroupKey`: optional parent resource `groupKey`. The resource appears as a child in the parent's expanded row. Downloading the parent downloads all downloadable children.
- `parentId`: parent resource ID resolved and persisted by the core. Plugins cannot set it; the core clears it from plugin output to prevent cross-plugin attachment.
- `dedupeKey`: optional custom deduplication key. Usually let the core derive it from `pluginId + groupKey` or the primary track URL.
- `tracks`: independently acquired input tracks. Each `id` is unique within the resource, `role` describes its meaning, and `executor` defaults to `http-file`.
- `requiredTracks`: roles required for `ready` status. Missing any required role results in `partial`, disabling download in the frontend.
- `capabilities`: determines frontend buttons. Current generic values are `download`, `preview`, `open`, and `copy`.
- `preview`: selects a trusted frontend renderer and preview track. Plugins cannot inject frontend code.
- `coverUrl`: optional cover URL. Do not embed large Base64 images in resources.
- `technical`: optional MIME, container, codec, and duration information for display or naming.
- `lifecycle.expiresAt`: optional millisecond timestamp for expected URL expiration.
- `metadata`: namespace site-private fields. The generic `author` field is available to filename templates.
- `actions`: references host actions declared in the Manifest, such as WASM local file processing or page commands. Dynamic arguments are stored in the action's `data`.

Resource output must also meet these constraints:

- `groupKey` and `parentGroupKey` are at most 512 bytes; a child cannot be its own parent.
- Trackless collections require a stable `groupKey`.
- `track.id` is unique within a resource. Track URLs must be valid remote addresses; `capture-file` uses `captureKey` instead.
- Extensions start with `.` and are at most 20 characters.
- `preview.trackId` must reference an existing track in the same resource.
- A serialized resource cannot exceed 1 MiB. Do not put complete response bodies in `metadata`.

A collection parent should use `media.collection`, a stable `groupKey`, and the `download` capability, with no `tracks`. Independent outputs such as images and audio should be children with `parentGroupKey`, not tracks of the parent: tracks represent inputs needed to produce a single output. Deleting a parent cascades to its children; deleting one child does not affect its siblings.

Supported `preview.renderer` values are `image`, `audio`, `video`, `pdf`, and `text`. Preview requests requiring processors accept only a core resource ID. The backend selects direct input by `trackId` and runs the WASM chains declared on that input and the output, allowing encrypted resources to be previewed with plugin processing. Ordinary images without processors load directly in the WebView first, for compatibility with image CDNs that reject Go TLS client fingerprints. If direct loading fails, they fall back to the backend proxy using captured headers. HLS master playlists, media playlists, segments, initialization segments, and AES keys are rewritten to short-lived local token URLs. The backend forwards all requests using the track headers. This does not require origin CORS support or expose an arbitrary-URL proxy.

### Incremental aggregation and correlation

`api.upsert(resource)` and `api.emit(resource)` have the same submission behavior. For matching `pluginId + groupKey`, the core atomically merges by `track.id` and sends update events to the frontend.

Do not pair separate requests by arrival order. Obtain the content ID and candidate URLs from the site's API, then register one-to-many aliases:

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

The Go core isolates correlation tables per plugin, with one-to-many mappings, TTLs, and capacity limits. URL normalization is conservative; the site plugin decides which signature parameters may be removed. If reliable correlation is impossible, emit separate or incomplete resources instead of guessing from request order.

## JavaScript hooks

The JavaScript runtime supports four synchronous top-level global functions. Implement those your plugin needs:

```javascript
function onObservation(observation, api) {
  var payload = JSON.parse(observation.response.body)
  api.log("found " + payload.id)
  api.emit(/* Resource */)
  return {decision: "continue", handled: true}
}

function createDownloadPlan(input, api) {
  api.log("Creating download plan; plugin version: " + api.pluginVersion)
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

`observation.settings` and `input.options.settings` contain the plugin settings saved in the app. `api.pluginVersion` is the Manifest version. `decision` can be `continue` or `stop`.

The four hooks receive these APIs:

| Hook | API argument |
| --- | --- |
| `onObservation(observation, api)` | Full `PluginAPI` |
| `onPageMessage(message, context, api)` | Full `PluginAPI` |
| `createDownloadPlan(input, api)` | Basic `PluginBaseAPI`, containing only `log` and `pluginVersion` |
| `refreshResource(input, api)` | Basic `PluginBaseAPI`, containing only `log` and `pluginVersion` |

The basic API needs no additional permission. Submit download plans and refresh results through return values. Every call uses an isolated runtime; API objects cannot be retained across calls.

### Observation structure

`onObservation(observation, api)` receives this JSON structure:

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

Header values are always string arrays; do not assume a single value. The request stage has no `response`. A `body` appears only when a matching rule permits reading and the plugin has the corresponding body permission. `truncated` is `true` when `bodyLimit` is reached.

Before parsing JSON, check that `response` exists, its status and Content-Type are appropriate, the body is nonempty, and it is not `truncated`. Truncated content must not be treated as complete JSON or complete media.

### `onObservation` return value

A hook can report resources through `api.emit` / `api.upsert`, or return:

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

- `handled: true` means a site plugin recognized the response; the host skips the final generic detector.
- `decision: "stop"` immediately stops subsequent plugins. Use it only when exclusive handling is required.
- `patch` requires `modify-response`; `syntheticResponse` requires `intercept-request`.
- `diagnostics` is for sanitized development diagnostics, without request credentials or complete private URLs.

### Runtime API

`api.log` and `api.pluginVersion` are available in all four hooks. The other interfaces below are available only in `onObservation` and `onPageMessage`.

| API | Returns | Description |
| --- | --- | --- |
| `api.emit(resource)` | `void` | Report a resource, with the same merge semantics as `upsert` |
| `api.upsert(resource)` | `void` | Recommended for incremental resources with a stable `groupKey` |
| `api.log(message)` | `void` | Write a log only when the current plugin setting `enableLog` is boolean `true`; the caller must sanitize it |
| `api.pluginVersion` | `string` | Current Manifest version |
| `api.correlate.register(value)` | `void` | Associate URL aliases with logical resources and tracks |
| `api.correlate.find(url)` | `ResourceReference[]` | Look up this plugin's registered associations |
| `api.page.send(sessionId, message)` | `boolean` | Send to one page session; requires `page-bridge` |
| `api.page.broadcast(filter, message)` | `number` | Broadcast to matching pages; returns the number of recipient sessions |
| `api.page.sessions(filter?)` | `PageMessageContext[]` | List page sessions visible to this plugin |

`emit` only adds a resource to the current call's output queue. Its fields, size, URLs, tracks, processors, and actions are still validated. Do not rely on invalid resources being silently repaired.

The host controls `api.log()` consistently in all four hooks using effective settings from `observation.settings`, page message `context.settings`, or `input.options.settings`. Nothing is logged if `enableLog` is missing, `false`, or the wrong type. Plugins can call `api.log()` directly without checking the switch. Host errors such as plugin load failures or hook exceptions are unaffected. To expose the switch in plugin management, declare `enableLog` in `settingsSchema.properties`; saved settings must still pass Manifest validation. See [application logs](../guide/troubleshooting.md#find-application-logs) for release log locations and rotation rules.

To reuse bytes the browser successfully received that cannot be requested again using the same URL, return a generic capture instruction from a response hook. The host only caches the current response bytes and does not interpret the site protocol. Capture keys are automatically scoped to the current plugin:

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

`range-file` merges ranges using response `Content-Range`, request `Range`, or `range` and `clen` in the URL. If the browser has not loaded every range, `capture-file` refuses to generate an incomplete file and asks the user to load more and retry. On startup, the app clears capture caches older than 24 hours. Each object is limited to 16 GiB. This capability does not bypass login, CSP, proxies, or site access controls.

The supported functions are `onObservation`, `onPageMessage`, `createDownloadPlan`, and `refreshResource`. All are optional synchronous top-level functions; Goja hooks cannot await Promises. Page scripts run in the browser and can use its asynchronous APIs, subject to page CSP, same-origin policy, and bridge limits.

A site plugin that has taken responsibility for a response should return `handled: true`. Other high-priority site plugins still run, but the final `builtin.generic-detector` is skipped, preventing an extra generic MIME/HLS resource. `decision: "stop"` immediately stops the entire remaining plugin chain and should be reserved for exclusive handling. Do not set `handled` when merely producing diagnostics or modifying a response while still expecting the generic detector to run.

If resource URLs, headers, or signatures expire, implement `refreshResource(input, api)`. It receives the same `{resource, options}` and basic API as `createDownloadPlan`, can log diagnostics with `api.log`, and returns:

```javascript
return {
  status: "refreshed",
  resource: updatedResource,
  message: ""
}
```

`status` can be `refreshed`, `authenticationRequired`, or `recaptureRequired`. On success, return the full updated resource. If sign-in or revisiting the page is required, return the original resource with the appropriate status and optionally a nonsensitive `message`. An absent implementation or `null` return means refresh is unsupported.

Refresh logic can only recompute from existing resource information such as business IDs, page URLs, and current settings. Plugins have no `fetch` and cannot invent login credentials. Usually, obtain fresh data through page scripts or new network observations, then update the resource using a stable `groupKey`.

Modifying a response requires `modify-response`:

```javascript
return {
  patch: {
    body: modifiedBody,
    headers: {"X-Plugin": "com.example.video"}
  }
}
```

Request interception uses `syntheticResponse` and requires `intercept-request`.

## Download plans

`createDownloadPlan(input, api)` converts a logical resource into a persistable download DAG:

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

The core acquires `inputs` concurrently, runs `pipeline` in order, executes output processors, and atomically places the result. Plain `http-file` inputs record Range segment checkpoints in the task working directory; separate-track and collection tasks can also pause and resume. Once input processors, WASM, or media pipelines start, pausing is unavailable. `hls` supports cancellation only; `ffmpeg-hls` live recording uses **Stop and Save**. The core handles cancellation, progress, temporary files, and rollback on failure.

Available input executors:

- `http-file`: ordinary HTTP/HTTPS files, supporting request headers, concurrent Range requests, and the download proxy.
- `capture-file`: reads a complete response cache captured alongside proxy traffic without requesting the remote URL again. Requires `capture-response-body`.
- `hls`: parses master/media playlists, supporting relative URLs, highest/lowest or maximum-bandwidth selection, `EXT-X-MAP`, `BYTERANGE`, AES-128, and VOD/explicit snapshot downloads. No pause support.
- `ffmpeg-hls`: user-installed FFmpeg directly downloads or records network streams, with headers, reconnection, and maximum recording duration. Requires `media.ffmpeg.network`. After recording stops, the host retains and places valid output.

Input IDs and pipeline step IDs must be valid and unique. Each step can reference only previously declared inputs or steps, and `output.input` must reference an available final result. `capture-file` cannot also provide a URL; other inputs must provide valid HTTP/HTTPS URLs.

Configure HLS through `options`:

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

Pipeline permissions:

| Executor | Required permission |
| --- | --- |
| `builtin.concat` | No additional permission |
| `builtin.media.mux` | `media.basic` |
| `builtin.media.remux` | `media.basic` |
| `builtin.media.extract_audio` | `media.basic` |
| `plugin.ffmpeg` | `media.ffmpeg` |

Media steps require compatible FFmpeg configured by the user. Plugins can declare a minimum version through Manifest `requires.ffmpeg`, such as `">=6.0"`.

These operations accept only host-managed inputs and outputs. `plugin.ffmpeg` passes arguments through an `args` array and <code v-pre>{{input.0}}</code> / <code v-pre>{{output}}</code> placeholders. The host uses only FFmpeg detected from settings, without a shell or arbitrary executable paths. Do not reference file paths the host has not provided.

## Plugin-provided WASM processors

Private encryption or transformation algorithms can ship with a plugin as `.wasm`. JavaScript identifies resources and supplies arguments; the Go core handles restricted execution, streaming I/O, and rollback.

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

Tracks and download plans reference modules only by processor IDs declared in the Manifest, never by arbitrary local paths:

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

### Process local files

Plugins can expose a declared WASM processor as a resource action. For example, a user may copy a link, download the encrypted file with another tool, then return to the resource menu to decrypt it:

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

The resource carries dynamic arguments for the action:

```javascript
actions: [{
  id: "decrypt-local-file",
  data: {options: {key: payload.key, nonce: payload.nonce}}
}]
```

The host renders and executes `process-file`. The user selects a file through a system dialog, and Go invokes only the WASM bound in the current plugin's Manifest. The plugin receives neither filesystem paths nor arbitrary read/write access. The result is a new `.decrypted` file in the same directory; the original is not overwritten.

### Send resource commands to a page

JavaScript plugins can declare a resource action as `page-command`. It must reference a page script with `bridge: true` in the current Manifest and request `inject-page-script` and `page-bridge`:

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

The resource carries plugin-defined dynamic arguments, up to 60 KiB serialized:

```javascript
actions: [{
  id: "inspect-page-resource",
  data: {assetId: payload.id, expectedType: "video"}
}]
```

`action.data` is persisted with the resource in `resources.db`. Store only persistable data such as business IDs or format options. Cookies, Authorization headers, page session tokens, and short-lived values such as `sessionBuffer` should remain in page memory and be used only after the page verifies the target.

The page script receives the standard envelope through its existing listener and validates the target:

```javascript
pageApi.onMessage(function (message) {
  if (message.type !== "resource-action" || message.actionId !== "inspect-page-resource") return
  if (String(currentAssetId()) !== String(message.data.assetId)) {
    showPageNotice("Open the matching resource and try again")
    return
  }
  inspectCurrentResource(message.requestId)
})
```

By default, the host only delivers messages subject to permissions. It does not interpret `data` or automatically show ordinary page messages in the desktop UI. Pages can display their own feedback or send results with `requestId` through `pageApi.send` to `onPageMessage`. To generate files, continue using Capture Store, resource reporting, and `enqueue-download`.

#### Execution ownership and progress reports

To display execution progress in the app, set `trackProgress: true` on the `page-command` action; it defaults to `false`. Page commands require `inject-page-script` and `page-bridge`; automatic download queuing separately requires `enqueue-download`.

The page must validate the business ID, then claim execution ownership. It may perform side effects only after receiving `accepted: true`:

```javascript
// message comes from pageApi.onMessage; protocol/type/actionId and business IDs have been validated.
var claim = await pageApi.commands.claim(message.requestId)
if (!claim.accepted) return // Another matching page already claimed this command.
try {
  await pageApi.commands.report(message.requestId, {
    state: "running", progress: 25, message: "Reading page data"
  })
  // Run the plugin workflow. Long tasks must report periodically even if the percentage is unchanged.
  await pageApi.commands.report(message.requestId, {
    state: "completed", progress: 100, message: "Processing complete"
  })
} catch (error) {
  await pageApi.commands.report(message.requestId, {
    state: "failed", message: "Page processing failed; reopen the target page"
  })
}
```

- `claim(requestId)` atomically grants ownership to one recipient page. Repeated claims return `accepted: false`. Pages that did not receive the command, and other plugins or scripts, cannot claim it.
- A recipient with a mismatched target or that is busy can report `state: "rejected"` before claiming. If all recipients reject it, the host shows failure. A page without ownership cannot overwrite the status of a claimed command.
- After claiming, a page may report `running`, `completed`, `failed`, or `cancelled`. Terminal states cannot be changed. `progress` is optional or a finite number from 0–100. Omission means unknown progress, not a fabricated percentage. `message` is plain text, up to 1024 UTF-8 bytes, and must not contain credentials, private URLs, or tokens. The entire request is limited to 4096 bytes; unknown fields and invalid types are rejected.
- Report at most once per second and send a heartbeat at least every 30 seconds. Reports share the ordinary message limit of 100 calls per 10 seconds per session. A command fails if unclaimed for 30 seconds, without reports for 90 seconds during execution, or running longer than 6 hours in total. A disconnected page cannot reliably report; timeout eventually resolves the command. Reloading, disabling, or uninstalling a plugin invalidates its existing commands.
- `claim` returns a short-lived `resumeToken` only to the successful claimant. Plugins that must reload a page automatically may temporarily save it in that tab's `sessionStorage`, then call `claim(requestId, resumeToken)` after reload to reconnect. The plugin and script are revalidated, the token rotates on each reconnection, and the old page loses reporting rights. Limit automatic retries and storage duration, and delete the token after use. Never put it in resources, logs, fixtures, or URLs. Reconnection cannot survive a plugin reload or app restart.
- The host retains at most 256 commands, keeping terminal states for 10 minutes. Cleanup occurs during status queries, delivery, and reports. The same active plugin/resource/action cannot be started twice. State is in-memory only and is not restored as download tasks.
- The desktop resource list queries status about every 1.5 seconds and shows the latest command's state, percentage, and message by `resourceId`. Page processing and downloading are separate stages: once a plugin finishes capture, publishes resources, and automatically queues downloads, the row continues with existing download/merge task state. Ordinary `pageApi.send` return values do not automatically become progress. This API cannot fabricate download tasks or specify file paths. There is currently no host-side command cancel button; plugins can provide their own cancellation interaction and report `cancelled`.

A complete sanitized example lives in `examples/plugins/page-command/`.

Common host execution errors use stable `errorCode` values translated by the desktop UI: duplicate execution (`page_command_already_active`), command limit (`page_command_limit_reached`), no connected page (`page_command_no_page`), full queue (`page_command_queue_full`), unavailable service (`page_command_unavailable`), oversized arguments (`page_command_too_large`), and other startup failures (`page_command_start_failed`). Failed local resource action responses contain `code`, `message`, and `data.errorCode`. Errors arising during execution are returned in command status `errorCode`: no acceptance (`page_command_not_accepted`), mismatched/busy pages (`page_command_target_unavailable`), timeout (`page_command_timeout`), and plugin reload (`page_command_reloaded`). Ordinary plugin reports cannot set host `errorCode` values.

The resource table's save-path column also shows page command messages and progress, falling back to localized generic status when there is no business message. These are not paths and are not clickable. A new download task replaces them with download or processing status; an openable real path appears only after success. The host translates only its own states and errors. Plugin business `message` values are displayed unchanged as plain text; plugins are responsible for their localization.

The desktop queries command status through `POST /api/resources/page-commands`, protected by API session authentication. Each query times out after 5 seconds and failures are retried. The UI marks unfinished commands as **Progress unavailable**, without presenting a query failure as execution failure or indefinitely retaining apparently live waiting states and percentages. When connectivity returns, it uses the latest host state. During query outages, newer download tasks may replace stale command displays. Each consecutive query outage produces at most one notification to avoid repeated polling popups.

### ABI v1

Modules must export linear memory and the following functions. All integers are WebAssembly `i32`:

```c
int32_t rd_abi_version(void); // Must return 1
uint32_t rd_alloc(uint32_t size);
void rd_free(uint32_t pointer, uint32_t size); // Optional
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

- `rd_init` receives UTF-8 JSON without the host's internal processor ownership fields. Return `0` on success.
- `rd_alloc` must return a valid linear memory address with enough space for the requested length. The host validates the range.
- Input chunks are at most 256 KiB, and the module writes back into the same memory region. `rd_transform` returns an output byte count in `0..capacity`; negative values signal failure.
- `capacity` is currently 320 KiB. `offset_low/high` represent the current input's unsigned 64-bit byte offset.
- `final=1` marks the last call. Files ending on a full chunk also receive a final zero-length call.
- `rd_free` is optional. If provided, it must release memory returned by `rd_alloc`. Modules must not retain addresses the host has already freed.

Modules have no WASI or host imports, so they have no network, filesystem, environment, clock, or command access. Each module is limited to 8 MiB, linear memory to 64 MiB, and options to 64 KiB. Individual calls and total processing both have timeouts. On failure, the host deletes temporary output without overwriting the original. A full example lives in `examples/plugins/wasm-xor/`.

## Declarative plugins

The declarative runtime suits JSON APIs where one response directly yields a single-track resource:

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

The JSON Path subset supports `$.a.b`, numeric array indexes, and a trailing `[*]`. Use JavaScript for multi-request aggregation, correlation, or custom download plans.

Declarative fields:

| Field | Description |
| --- | --- |
| `stage` | `request` or `response`, usually `response` |
| `format` | Only `json` is currently supported |
| `root` | Selects an object or array of objects; defaults to the entire JSON |
| `resource.url` | Required download URL selector |
| `resource.title` / `coverUrl` | Optional title and cover URL |
| `resource.kind` | Resource kind; should match an ID declared in `resourceKinds` |
| `resource.role` | Track role; inferred from the last segment of `kind` if omitted |
| `resource.executor` | Defaults to `http-file` |
| `resource.contentType` / `extension` / `size` | File type, extension, and byte count |
| `resource.preview` | `image`, `audio`, `video`, `pdf`, or `text` |
| `resource.metadata` | Map of custom metadata selectors; namespace site-specific fields |

Each selector uses `{path: "$.field"}` to read data or `{value: "constant"}` for a fixed value. Entries without a URL are skipped, and numeric `size` values are converted to integers. The declarative runtime does not execute custom code, persist cross-response state, or generate multi-input download plans.

## Validation and offline replay

Validate plugins without starting the proxy:

```bash
go run main.go plugin create
go run main.go plugin lint ./plugins/com.example.my-plugin
go run main.go plugin replay ./examples/plugins/javascript-basic ./examples/plugins/javascript-basic/fixtures/video.json
go run main.go plugin pack ./plugins/com.example.my-plugin
```

A fixture contains a sanitized `observation` and expected results:

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

Fixtures can contain one `observation`, or `observations` to replay multi-request sessions such as page APIs, video, audio, and scrolling in sequence. The same `groupKey` aggregates using actual runtime semantics. Assertions support `resourceCount`, `resourceUrls`, `processorTypes`, `decision`, and `patchBodyContains`. Templates live in `examples/plugins/`; see [Plugin SDK v1](plugin-sdk.md) for JSON Schema and TypeScript declarations.

Fixture guidelines:

- Keep only headers and body fields needed to match and generate resources.
- Retain real domains where needed, but replace private paths, accounts, and resource IDs with stable example values.
- Remove or replace cookies, Authorization headers, device identifiers, and temporary signatures.
- Use `observations` for a fixed replay sequence in correlation tests, while keeping the plugin independent of live arrival order.
- Assert at least resource count and URLs. Processor plugins should also assert `processorTypes`.

## Debugging and common errors

Plugin cards show Manifest validation, entry compilation, and runtime errors. Troubleshoot in this order:

1. Run `plugin lint` and resolve directory, field, permission, and entry-file errors first.
2. Run `plugin replay` and confirm the fixture consistently produces expected results.
3. Reload the plugin in the app and inspect the latest card and log errors.
4. Confirm the page generated new requests and the domain/path match `permissions.domains` and `match`.
5. Check body permissions, `readBody`, Content-Type, and `truncated`.
6. Finally, check for changes to site APIs, login state, or signatures.

Common issues:

| Symptom | Common causes |
| --- | --- |
| Plugin fails to load | Invalid ID, version, API version, entry path, or permission dependency |
| Hook never runs | Stage, domain, path, method, or Content-Type does not match |
| Empty `response.body` | Missing read permission, `readBody: false`, or a different rule matched |
| JSON parsing fails | Truncated body, non-JSON response, or a login/anti-abuse page |
| Resource appears but cannot download | Missing required tracks, invalid URL, undeclared processor, or incomplete capabilities |
| Plugin temporarily stops processing | Consecutive errors or timeouts triggered circuit breaking; fix and reload |
| Page script is not injected | No TLS interception, compressed response, restrictive CSP, or no safe injection point |
| Fixture passes but live traffic fails | Missing fixture branches, changed site data, expired credentials, or different request order |

Each JavaScript hook has a 5-second limit, including initialization and result export. The outer call allows 10 seconds including queuing. Long-running capture in page scripts is not bound by that single-hook limit; page commands still have separate claiming, heartbeat, and total-duration limits. Avoid processing huge objects in loops or logging complete responses, cookies, or signed URLs. Create sanitized fixtures for data needed in long-term reproductions.

## Pre-release checks

Before publishing, confirm:

- The plugin has a stable ID without reserved `builtin.` / `official.` prefixes, and uses semantic versioning.
- Domains and capabilities are limited to actual needs.
- Manifest names, author information, descriptions, resource kinds, and settings have appropriate localization.
- At least one sanitized fixture passes offline replay. Complex branches and multi-request correlation have multiple fixtures.
- Runtime files such as JavaScript, WASM, and page scripts are included in the plugin directory.
- Logs, fixtures, README, and example settings contain no account details, cookies, Authorization headers, or private URLs.
- `go run main.go plugin lint ...`, `go run main.go plugin replay ...`, and `go run main.go plugin pack ...` all succeed.
- For extension store releases, `dist/plugin.zip` is committed before tagging.
- The final ZIP has been installed and basic operations checked in the current stable app.

See [Publishing to the Extension Store](extension-store.md) for public repository structure, GitHub topic, release tag, and version requirements.
