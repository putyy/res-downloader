---
description: Develop res-downloader plugins using the Manifest, permissions, JavaScript API, page scripts, resource model, download plans, WASM, and packaging workflow.
---

# Plugin Development

This guide is for plugin developers. It covers plugin directories, the Manifest, permissions, runtime APIs, resource models, download plans, WASM processors, debugging, testing, and publishing. For installing and managing plugins as a user, see [Plugin Management](../guide/plugin-management.md).

res-downloader identifies its plugin protocol by version. The app passes captured request and response information to plugins, which return resources in the agreed data format. Plugins cannot directly access Go objects, the filesystem, or the shell, or make arbitrary network requests themselves.

The repository's `plugins/` directory is a local workspace for cloning and debugging independent plugin repositories. By default, these sources are not committed to the host repository, loaded automatically, or packaged with the app. Example plugins live in `examples/plugins/`; bundled plugins shipped with the app live in `internal/plugin/bundled/`.

Before developing a plugin, read “Choose a plugin type → Quick start → Manifest → Permissions → Resource model → JavaScript hooks or Declarative plugins → Validation and offline replay.” Page scripts, download plans, and WASM are advanced capabilities to use as needed.

Supported capabilities include:

- `declarative`: extract a single-track resource from one JSON response using JSON/YAML and a restricted JSON Path syntax.
- `javascript`: process complex JSON, correlate multiple requests, define download plans, and refresh resources.
- Plugin-provided WASM: run private decryption or transformation algorithms on download inputs or outputs.
- Resource actions: process local files or send plugin-defined arguments to matching page scripts.
- Host downloads: ordinary HTTP and HLS, plus audio/video merging, remuxing, audio extraction, and live recording when FFmpeg is configured.

## Choose a plugin type

| Scenario | Recommendation |
| --- | --- |
| One JSON response directly contains the title and download URL | `declarative` |
| Conditional logic, complex object traversal, or multiple quality options | `javascript` |
| Video and audio arrive in separate requests and must be paired by content ID | `javascript` + correlation APIs |
| Links expire and must be resolved again before download | `javascript` + `refreshResource` |
| Private decryption or byte transformations after downloading | JavaScript + plugin WASM |
| Player data must be read from the page's runtime environment | JavaScript + page scripts; use only when ordinary observation APIs are insufficient |

Choose the simplest implementation that works. If the response JSON is sufficient, do not request page script, response modification, or advanced FFmpeg permissions.

## Quick start

### 1. Copy an example

Run from the repository root:

```bash
cp -R examples/plugins/javascript-basic ./plugins/com.example.my-plugin
```

For a simple JSON API, copy `declarative-basic`. The WASM example is `wasm-xor`. You can also use the project CLI to create a JavaScript scaffold interactively:

```bash
go run main.go plugin create
```

Press Enter at each prompt to use the default parent directory `./plugins` and set both the plugin ID and display name to `com.example.my-plugin`. This creates `./plugins/com.example.my-plugin`. You can also provide arguments directly:

```bash
go run main.go plugin create ./plugins/com.example.my-plugin com.example.my-plugin "Example Video"
```

The scaffold contains `plugin.json`, `main.js`, `README.md`, `.gitignore`, and an empty `fixtures/` directory.

Plugins include an **Enable logging** setting named `enableLog`, with type `boolean` and default `false`. Manually created plugins should also declare this setting and provide Chinese and English labels.

### 2. Edit the Manifest and entry point

At minimum, change the plugin `id`, name, version, permitted domains, and matching rules. Use a stable reverse-domain ID such as `com.example.video`. Community plugins cannot use the host-reserved `builtin.` or `official.` prefixes, including case variants. See [Plugin sources and reserved IDs](extension-store.md#plugin-sources-and-reserved-ids) for how official plugins and local ZIPs are classified.

JavaScript plugins can implement `onObservation` in `main.js`. Start by matching one response and reporting one resource, then add correlation, refresh, or download processing as needed.

### 3. Prepare a sanitized fixture

Save a representative request/response as a fixture. Remove cookies, Authorization headers, account details, signatures, and real content. Keep only the fields needed for the plugin's decisions.

### 4. Static validation and offline replay

```bash
go run main.go plugin lint ./plugins/com.example.my-plugin
go run main.go plugin replay ./plugins/com.example.my-plugin ./plugins/com.example.my-plugin/fixtures/video.json
```

`lint` checks the directory, Manifest, entry files, and permission dependencies. `replay` executes a fixture without starting the proxy.

### 5. Package and install

```bash
go run main.go plugin pack ./plugins/com.example.my-plugin
```

The default output is `<plugin-directory>/dist/plugin.zip`. To save it elsewhere, add an output path at the end of the command.

The packer excludes:

- Directories: `.git/`, `.idea/`, `.vscode/`, `dist/`, and `tests/`.
- Files: the output ZIP itself, plus `.gitignore`, `.DS_Store`, `README.md`, and `LICENSE`.

These are the packer's own rules; it does not read `.gitignore`. Git's rules determine whether `dist/plugin.zip` can be committed to the plugin repository.

To test the package, select the generated ZIP under **Plugins** in the app and install it. During development, you can also place the plugin directory in the `plugins` subdirectory of res-downloader's data directory, then click **Reload**.

For local ZIP source labels, reserved IDs, and replacement conditions, see [Plugin sources and reserved IDs](extension-store.md#plugin-sources-and-reserved-ids) and [Other distribution methods](extension-store.md#other-distribution-methods).

Keep your own JavaScript tests in `tests/` at the plugin repository root, preferably named `*.test.js`. This directory is excluded from the package. Sanitized data used by `plugin replay` stays in `fixtures/`.

## Development requirements

- **Least privilege**: declare only the domains and capabilities you need; avoid `*` domains.
- **Minimal bodies**: narrow the `match` rules, set `readBody: false` when bodies are unnecessary, and choose the smallest sufficient `bodyLimit`.
- **Sensitive data**: do not log or report cookies, Authorization headers, account details, administrator passwords, or long-lived credentials. Use `nonPersistentHeaders` for headers that should not be saved.
- **Reliable correlation**: associate requests using content IDs, track IDs, or explicit aliases. Do not guess based on arrival order.
- **Clear failures**: return an explicit refresh status for expired URLs. Keep incomplete resources non-downloadable rather than producing invalid files that appear successful.
- **Untrusted page messages**: page scripts share the website's environment. Validate types, lengths, and business fields in every bridge message.
- **Lawful use**: plugins must not bypass access controls or unauthorized DRM. Fixtures and logs must not contain other people's private data.

## Plugin directories and loading

Each plugin uses its own directory:

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

The Manifest can be `plugin.json`, `plugin.yaml`, or `plugin.yml`. In a ZIP, it must be at the archive root or inside its single top-level folder. Entry and asset files must use paths relative to the plugin directory. Symlinks, paths escaping the directory, and oversized files are rejected.

Plugins load when the app starts. During development, click **Reload** to load changes. Enabling or disabling plugins and saving plugin settings also refresh the active plugin set. The directory name should match the Manifest `id`.

App upgrades overwrite official plugins. To customize one, copy it with a new plugin ID.

## Manifest

The Manifest file can be `plugin.json`, `plugin.yaml`, or `plugin.yml`:

For JSON, use the [Manifest Schema and editor declarations](plugin-sdk.md) for field assistance. After making changes, run `plugin lint` to check the configuration and entry files. Installation still performs validation according to the plugin's source.

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
| `id` | Yes | Up to 64 characters: letters, digits, dots, hyphens, and underscores only. Reserved prefixes are case-insensitive; see [Plugin sources and reserved IDs](extension-store.md#plugin-sources-and-reserved-ids) |
| `name` | Yes | Plugin name shown when localized text is unavailable |
| `author` | No | `name` appears on the plugin card; `url` must be an HTTP/HTTPS address |
| `version` | Yes | Semantic version, for example `1.2.0` |
| `apiVersion` | Yes | Must currently be `1` |
| `runtime` | Yes | `javascript` or `declarative` |
| `entry` | For JavaScript | A `.js` entry inside the plugin directory, up to 1 MiB |
| `priority` | No | Higher values run first; multiple plugins may process the same request |
| `permissions` | Yes | Permitted domains, capabilities, and body size limit |
| `match` | Yes | Request or response matching rules; an empty array matches all permitted stages within the permitted domains |
| `locales` | No | Translations of the plugin name and description |
| `resourceKinds` | No | Site-specific resource kinds and display labels |
| `settingsSchema` | No | Plugin setting structure, defaults, and form hints |
| `pageScripts` | No | Scripts injected by the host into matching HTML pages |
| `extractors` | For declarative | Rules for extracting resources from JSON bodies |
| `processors` | No | Plugin-provided WASM processors |
| `actions` | No | Local file processing or page command actions displayed by the host |
| `requires` | No | Optional host tool requirements, such as `ffmpeg: ">=6.0"` |

In `match`, `host`, `path`, and the full `url` support `*` wildcards; `method` is case-insensitive. `contentTypes` matches the response Content-Type. `readBody` determines whether a matching rule needs the body. Set `readBody: false` explicitly when only the URL or response headers are needed.

`resourceKinds` appear among the capture types on the main page. Users can filter broadly by the stable `primaryType` or more precisely by the plugin's `kind`.

`settingsSchema` supports basic validation for `string`, `number`, `integer`, `boolean`, `object`, `array`, and `enum`. Basic types generate forms in plugin management; complex structures can use the advanced JSON editor.

The app currently supports Chinese (`zh`) and English (`en`). Use these two language keys in plugin `locales`, setting `x-locales`, and enum `x-enumLabels`, and provide text for both languages.

Use `x-locales` for setting names and descriptions, and `x-enumLabels` for enum option labels:

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
| `observe-request` | Receive request URLs, headers, and other metadata | Restricted to `permissions.domains` and matching rules |
| `read-request-body` | Read matching request bodies | Requires `observe-request`; request only when necessary |
| `intercept-request` | Return a locally generated response | Requires `observe-request`; changes page behavior |
| `observe-response` | Receive response status, headers, and other metadata | Basic permission for resource discovery |
| `read-response-body` | Read matching response bodies | Requires `observe-response`; bounded by `bodyLimit` |
| `modify-response` | Modify matching responses | Requires `observe-response`; truncated bodies cannot be modified |
| `emit-resource` | Report resources | Output is still validated by the host |
| `process-download` | Invoke WASM processors declared by this plugin | Cannot select arbitrary local files or modules |
| `media.basic` | Use controlled operations such as mux, remux, and audio extraction | Requires compatible user-configured FFmpeg |
| `media.ffmpeg` | Use the advanced FFmpeg argument-array interface | No shell; requires FFmpeg |
| `media.ffmpeg.network` | Allow FFmpeg to read plugin-provided HLS/live URLs | Download URLs must be valid HTTP/HTTPS addresses; this is a sensitive permission |
| `inject-page-script` | Inject scripts into matching HTML pages | Target domains must undergo TLS interception and allow safe injection |
| `page-bridge` | Exchange JSON between page scripts and the plugin runtime, or receive user-triggered `page-command` actions | Requires `inject-page-script` |
| `capture-response-body` | Cache Range responses actually read by the browser, or accept media segments captured by page scripts | Requires `observe-response`; page segments also require `inject-page-script` and `page-bridge` |
| `enqueue-download` | Automatically create downloads after a page message reports resources | Requires `page-bridge` and `emit-resource`; sensitive because it writes to the download directory |

Bodies reach plugins only when the domain, matching rule, and read permission all allow it. Data exceeding `bodyLimit` is marked `truncated`; plugins cannot modify truncated responses.

Each JavaScript hook call runs in an independent environment. The main limits are:

| Item | Limit |
| --- | --- |
| Script size | Up to 1 MiB |
| Execution time | Up to 5 seconds, including environment initialization, hook execution, and conversion of the returned result |
| Total time including queuing | Up to 10 seconds, including time waiting for other calls to finish |
| Concurrent calls | Up to 4 per plugin |

Cancelling a request also interrupts its JavaScript execution. A call that has timed out but has not yet exited still counts toward the concurrency limit until it ends. The app records slow calls and suspends plugin execution after consecutive failures.

The plugin environment provides no Node.js, `fetch`, filesystem, or system-command APIs, and cannot wait for Promises. Use the app's correlation APIs to retain associations across requests. JavaScript globals are not retained between calls.

## Page scripts and the bidirectional message bridge

JavaScript plugins declare scripts to inject into pages through `pageScripts`. The app's MITM proxy performs the injection, so TLS interception must be enabled for the target domain.

Page script injection does not go through `onObservation` and is not subject to that hook's `bodyLimit` or 5-second execution limit. Only `document-start` is currently supported. Set `frames` to `top` (the default, top-level pages only) or `all` (including subframes).

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

Each entry must be a regular `.js` file inside the plugin directory, up to 256 KiB. A plugin may declare at most 8 page scripts.

The app checks only the first 256 KiB of HTML and prefers the page's existing CSP nonce. It injects scripts only when the response is uncompressed `text/html`, a `<head>` can be found, and the page's CSP allows inline scripts. Otherwise, it preserves the original response. It does not remove or relax the site's CSP.

The page entry runs inside an async function with a local `pageApi`:

```javascript
pageApi.onMessage(function (message) {
  if (message.type === "probe") runProbe(message.url)
})

var result = await pageApi.send({type: "player-ready", data: collectPlayerData()})
```

With `capture-response-body` declared, a page script can use its current page session token to write binary media segments into the app's capture cache (Capture Store):

```javascript
await pageApi.capture.start("video:123:video")
await pageApi.capture.write("video:123:video", arrayBufferOrTypedArray)
await pageApi.capture.complete("video:123:video")
// On cancellation or failure:await pageApi.capture.abort("video:123:video")
```

The four methods serve these purposes:

- `start`: begin capturing, clearing any existing cache with the same name.
- `write`: append data in the order each call completes.
- `complete`: mark capture as complete so that `capture-file` can read it.
- `abort`: cancel capture and immediately delete the incomplete cache.

The page script must order segments for playback and remove duplicates. The app does not parse the site's private streaming protocol. A segment may be at most 32 MiB. Each page session may use up to 4 capture keys simultaneously and write up to 16 GiB in total.

Writes are checked against the page Origin, session token, and plugin permissions. A page can access only capture caches permitted for the current plugin, not arbitrary files or another plugin's caches.

With the bridge enabled, page-to-plugin messages use same-origin POST, and plugin-to-page messages use SSE. The proxy answers internal addresses directly instead of forwarding them to the website. Each page load receives its own `pageSessionId`. Closing the page or reloading, disabling, or uninstalling the plugin invalidates the session.

Plugins handle page messages through a synchronous top-level hook:

```javascript
function onPageMessage(message, context, api) {
  if (message.type !== "player-ready") return {ok: false, error: "unsupported message"}

  api.page.send(context.pageSessionId, {type: "probe", url: message.data.url})
  api.upsert(/* ResourceCandidate */)
  return {ok: true, data: {accepted: true}}
}
```

Plugins with `enqueue-download` can return `autoDownload: true` in a page message result. The app creates tasks only for resources in that same result that passed validation, were successfully published, and have stable `groupKey` values, up to 4 per call. Missing permissions, incomplete resources, or invalid download plans prevent queuing:

```javascript
return {
  ok: true,
  resources: [resource],
  autoDownload: true
}
```

`context` provides `pageSessionId`, `scriptId`, `pageUrl`, `origin`, and the current plugin `settings`.

- `settings` is passed only to `onPageMessage` in the app, not directly to the webpage or through `api.page.sessions()`. If a page needs settings, the plugin can return necessary nonsensitive options in an initialization reply, such as whether to show a button.
- `pageUrl` is the address when the session was created. After a single-page app changes content, the page script must check the current item or resource again rather than relying only on this address.
- Plugins with `page-bridge` may also call `api.page.broadcast(filter, message)` and `api.page.sessions(filter)`.

Ordinary messages and replies must be JSON, up to 64 KiB each. Use `pageApi.capture.write` for larger media data. Each plugin may retain up to 32 active page sessions, with additional limits on each session's message queue, connection count, and request rate.

Declaring a `page-command` resource action in the Manifest lets users send commands from the app's resource list to a designated page script. The app uses the `action.data` already saved in the resource record and does not accept custom arguments supplied separately by the frontend. Messages have this format:

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

The app generates a random `requestId` for each command. The page must return the same `requestId` with its result.

Before sending a command, the app removes expired idle page sessions. A recipient page must meet all of these conditions:

- It belongs to the current plugin and uses the script named by the action's `pageScript`.
- The script has `bridge: true`.
- An SSE connection between the page and the app is still active.

Sending fails if no page meets these conditions, the message exceeds 64 KiB, or all target pages have full message queues. On success, the local API returns `requestId`, `pageScriptId`, and `delivered`. Here, `delivered` counts page sessions whose queues accepted the command.

Entering a queue does not mean the page has received or executed the command; it may still disconnect. The page must return its result with `requestId`. A command may reach several pages, so each page must check the resource ID and use `requestId` to prevent duplicate execution.

Page scripts and website code run in the same webpage JavaScript environment. Website code may read or imitate bridge requests, so plugins must validate incoming page messages.

By default, the bridge does not allow webpages to access files, shell, databases, the downloader, or other plugins. With `enqueue-download`, a plugin can create download tasks for resources just reported and validated in the current call. The webpage still cannot specify file paths or arbitrarily control download tasks.

## Resource model

Plugins report a logical resource. An ordinary file usually has one track. Sites with separate audio and video may provide several `video`, `audio`, or `subtitle` tracks and multiple quality options.

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

- `primaryType`: a stable main type, limited to `video`, `audio`, `image`, `document`, `archive`, `collection`, and `other`.
- `kind`: a required plugin-specific kind, not restricted to a core enum.
- `traits`: features that can be combined, such as `encrypted`, `multiTrack`, `segmented`, `streaming`, `live`, `gallery`, and `mergeRequired`. Use `plugin.id:name` for plugin-private traits.
- `groupKey`: a stable identifier for the same resource within one plugin. Required when combining resource information from multiple requests.
- `parentGroupKey`: the optional parent resource's `groupKey`. The resource appears as a child in the parent's expanded row. Downloading the parent downloads all downloadable children.
- `parentId`: the parent resource ID resolved and persisted by the core. Plugins cannot set it; the core clears it from plugin output to prevent attaching resources across plugins.
- `dedupeKey`: an optional custom deduplication key. Usually let the core derive it from `pluginId + groupKey` or the primary track URL.
- `tracks`: tracks to download separately. Each `id` is unique within the resource, `role` identifies its purpose such as video or audio, and `executor` defaults to `http-file`.
- `requiredTracks`: roles required for `ready` status. If any is missing, the resource is `partial` and cannot be downloaded from the UI.
- `capabilities`: determines which action buttons are available. Current generic values are `download`, `preview`, `open`, and `copy`.
- `preview`: selects a trusted frontend renderer and the track to preview. Plugins cannot inject frontend code.
- `coverUrl`: an optional cover URL. Do not put large Base64 images into resources.
- `technical`: optional MIME, container, codec, and duration information for display or naming.
- `lifecycle.expiresAt`: an optional millisecond timestamp for the expected URL expiration time.
- `metadata`: namespace site-private fields. The generic `author` field is available to filename templates.
- `actions`: references host actions declared in the Manifest, such as WASM local file processing or page commands. Dynamic arguments are stored in the action's `data`.

Resource output must also meet these constraints:

- `groupKey` and `parentGroupKey` are at most 512 bytes. A child cannot be its own parent.
- A collection without tracks must have a stable `groupKey`.
- `track.id` is unique within a resource. Track URLs must be valid remote addresses; `capture-file` uses `captureKey` instead.
- Extensions start with `.` and are at most 20 characters.
- `preview.trackId` must reference a track in the current resource.
- A serialized resource cannot exceed 1 MiB. Do not put complete response bodies into `metadata`.

A collection parent should use `media.collection`, a stable `groupKey`, and the `download` capability, without `tracks`. Independent outputs such as images or audio should be child resources with `parentGroupKey`. Do not put them in the parent's `tracks`, which represent inputs needed to produce one output. Deleting a parent also deletes its children; deleting one child does not affect its siblings.

`preview.renderer` supports `image`, `audio`, `video`, `pdf`, and `text`. Preview behavior depends on the resource:

- **Resources requiring WASM processing**: the preview request supplies only the app's resource ID. The backend selects the input using `trackId` and runs the WASM processors declared on that input and the output, allowing resources that need decryption to be previewed.
- **Ordinary images**: without processors, the WebView loads them directly first, avoiding image CDNs that reject Go TLS connections. If that fails, the backend requests the image using the captured headers.
- **HLS**: master playlists, media playlists, segments, initialization segments, and AES key addresses are converted to local addresses with short-lived tokens. The backend forwards requests with the track's headers, so the source site needs no additional CORS configuration. The preview endpoint can access only these registered addresses; it is not an arbitrary-URL proxy.

### Incremental aggregation and correlation

`api.upsert(resource)` and `api.emit(resource)` have the same submission behavior. When the same plugin reports the same `groupKey` multiple times, the app combines those reports into one resource and updates tracks by `track.id`. Other concurrent updates cannot interrupt the merge. Once complete, the app notifies the UI to update.

Do not pair audio and video by request arrival order. First obtain the content ID and download addresses from the site's API, then register URLs that may refer to the same resource or track as aliases:

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

Each plugin has its own correlation table. One URL may be associated with several resources or tracks, and records have expiration times and a count limit. The app performs only basic URL normalization; it does not decide which signature parameters the plugin may remove. If the relationship is uncertain, report separate resources or keep the resource incomplete.

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

The API argument for each hook is:

| Hook | API argument |
| --- | --- |
| `onObservation(observation, api)` | Full `PluginAPI` |
| `onPageMessage(message, context, api)` | Full `PluginAPI` |
| `createDownloadPlan(input, api)` | Basic `PluginBaseAPI`, containing only `log` and `pluginVersion` |
| `refreshResource(input, api)` | Basic `PluginBaseAPI`, containing only `log` and `pluginVersion` |

The basic API needs no additional permission. Submit download plans and refresh results through return values. Every hook call runs in an independent environment, so API objects cannot be retained between calls.

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

Header values are always string arrays; do not assume a single value. The request stage has no `response`. A `body` is provided only when a matching rule allows reading and the plugin has the corresponding body permission. `truncated` is `true` when `bodyLimit` is reached.

Before parsing JSON, check that `response` exists, its status and Content-Type are appropriate, the body is not empty, and it is not `truncated`. Do not treat truncated content as complete JSON or media data.

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

- `handled: true` means the site plugin has recognized the response. The app skips the final generic detector.
- `decision: "stop"` immediately stops subsequent plugins. Use it only when exclusive handling is needed.
- `patch` requires `modify-response`; `syntheticResponse` requires `intercept-request`.
- `diagnostics` is for sanitized development diagnostics and must not contain request credentials or complete private URLs.

### Runtime API

`api.log` and `api.pluginVersion` are available in all four hooks. The other APIs below are available only in `onObservation` and `onPageMessage`.

| API | Returns | Description |
| --- | --- | --- |
| `api.emit(resource)` | `void` | Report a resource, using the same merge behavior as `upsert` |
| `api.upsert(resource)` | `void` | Recommended for incremental resource updates with a stable `groupKey` |
| `api.log(message)` | `void` | Write a log only when the current plugin's `enableLog` setting is boolean `true`; the caller must sanitize it |
| `api.pluginVersion` | `string` | Current Manifest version |
| `api.correlate.register(value)` | `void` | Associate URL aliases with resources and tracks |
| `api.correlate.find(url)` | `ResourceReference[]` | Look up associations registered by the current plugin |
| `api.page.send(sessionId, message)` | `boolean` | Send a message to one page session; requires `page-bridge` |
| `api.page.broadcast(filter, message)` | `number` | Broadcast to matching pages and return the number of recipient sessions |
| `api.page.sessions(filter?)` | `PageMessageContext[]` | List page sessions visible to the current plugin |

Calling `emit` adds the resource to the current call's pending output. The app then validates its fields, size, URLs, tracks, processors, and actions. Resources must meet these requirements to work correctly.

The app controls `api.log()` in all four hooks using the current call's settings. These come from `observation.settings`, page message `context.settings`, or `input.options.settings`.

- Plugin logs are written only when `enableLog` is boolean `true`. Missing, `false`, or incorrectly typed values disable them.
- Plugins can call `api.log()` directly without checking the switch again.
- App errors such as plugin load failures or hook exceptions are unaffected by this setting.

To show the log switch in plugin management, declare `enableLog` in `settingsSchema.properties`. Saved settings are still validated against the Manifest. See [application logs](../guide/troubleshooting.md#find-application-logs) for log locations.

To reuse data the browser successfully received but cannot request again using the same URL, return a capture instruction from the response hook. The app caches the current response bytes without interpreting the site's protocol. Capture keys are automatically limited to the current plugin:

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

`range-file` combines byte ranges using the response `Content-Range`, request `Range`, or `range` and `clen` in the URL. If the browser has not loaded every range, `capture-file` refuses to produce an incomplete file and asks the user to load more and retry. On startup, the app clears capture caches older than 24 hours. Each object may be at most 16 GiB. This capability does not bypass login, CSP, proxies, or site access controls.

The supported functions are `onObservation`, `onPageMessage`, `createDownloadPlan`, and `refreshResource`. All are optional synchronous top-level functions; Goja hooks cannot wait for Promises. Page scripts run in the browser and can use its asynchronous APIs, subject to the page's CSP, same-origin policy, and bridge restrictions.

When a site plugin takes responsibility for a response, it should return `handled: true`. Other high-priority site plugins still run, but the final `builtin.generic-detector` is skipped, avoiding an additional generic MIME/HLS resource. `decision: "stop"` immediately stops the remaining plugin chain and should be used only for exclusive handling. Do not set `handled` when only producing diagnostics or modifying a response while still expecting the generic detector to run.

If resource URLs, headers, or signatures expire, implement `refreshResource(input, api)`. It receives the same `{resource, options}` and basic API as `createDownloadPlan`, can use `api.log` for refresh diagnostics, and returns:

```javascript
return {
  status: "refreshed",
  resource: updatedResource,
  message: ""
}
```

`status` can be `refreshed`, `authenticationRequired`, or `recaptureRequired`. On success, return the complete updated resource. If sign-in or reopening the page is needed, return the original resource with the corresponding status and optionally a nonsensitive `message`. An absent implementation or a `null` result means refresh is unsupported.

Refresh logic can calculate new values only from existing information such as resource IDs, page URLs, and current settings. Plugins have no `fetch` and cannot generate login credentials. Usually, obtain new data through page scripts or fresh network observations, then update the resource using a stable `groupKey`.

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

`createDownloadPlan(input, api)` generates a download plan. It specifies the files or tracks to download, processing steps such as merging or decryption, and the final output:

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

The app downloads `inputs` in parallel, runs the `pipeline` steps in order, and then runs output processors. Only after all processing succeeds does it save the temporary result to the final path. The app manages progress, temporary files, cancellation, and cleanup after failures.

Pause and stop behavior depends on the current stage:

- Ordinary `http-file` downloads record progress for each Range segment and can resume after a pause. Separate-track and collection tasks also support pause and resume.
- Pausing is no longer accepted once input processors, WASM, or media processing steps begin.
- `hls` supports cancellation only, not pausing.
- `ffmpeg-hls` live recording uses **Stop and Save** to retain valid recorded content.

Available input executors:

- `http-file`: ordinary HTTP/HTTPS files, with request headers, concurrent Range downloads, and the download proxy.
- `capture-file`: reads complete response data saved during capture without requesting the remote URL again. Requires `capture-response-body`.
- `hls`: parses HLS master and media playlists, supports relative URLs, highest/lowest bandwidth or a bandwidth limit for variant selection, `EXT-X-MAP`, `BYTERANGE`, and AES-128. It can download VOD content or only the segments already listed in the current playlist; it does not continuously follow a live stream. Pausing is unsupported.
- `ffmpeg-hls`: user-installed FFmpeg downloads or records network streams directly, with request headers, reconnection, and a maximum recording duration. Requires `media.ffmpeg.network`. After recording stops, the app saves the valid recording.

Input and processing-step IDs must be valid and unique within the plan. Each processing step may reference declared inputs or steps preceding it. `output.input` selects the final result to save. `capture-file` uses a cache and cannot also provide a URL; other inputs must provide valid HTTP/HTTPS URLs.

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

Permissions required by processing steps:

| Executor | Required permission |
| --- | --- |
| `builtin.concat` | No additional permission |
| `builtin.media.mux` | `media.basic` |
| `builtin.media.remux` | `media.basic` |
| `builtin.media.extract_audio` | `media.basic` |
| `plugin.ffmpeg` | `media.ffmpeg` |

Media steps require compatible FFmpeg configured by the user. Plugins can declare a minimum version with Manifest `requires.ffmpeg`, such as `">=6.0"`.

These operations accept only host-managed inputs and outputs. `plugin.ffmpeg` takes an `args` array with <code v-pre>{{input.0}}</code> and <code v-pre>{{output}}</code> placeholders. The app uses FFmpeg detected from settings, without a shell, and does not accept arbitrary executable paths. Do not reference file paths that the app has not provided.

## Plugin-provided WASM processors

Private encryption or transformation algorithms can be distributed as `.wasm` files with a plugin. JavaScript identifies resources and supplies arguments; the Go core handles restricted execution, streaming reads and writes, and rollback.

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

Tracks and download plans select WASM modules through processor IDs declared in the Manifest, not arbitrary local file paths:

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

Plugins can expose a declared WASM processor as a resource action. For example, a user can copy a link, download the encrypted file with another tool, then return to the resource's action menu to decrypt it:

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

The resource carries the dynamic arguments needed for the action:

```javascript
actions: [{
  id: "decrypt-local-file",
  data: {options: {key: payload.key, nonce: payload.nonce}}
}]
```

The app displays and executes `process-file` actions. After the user chooses a file through a system dialog, the app invokes the WASM processor declared by the current plugin. The plugin does not receive file paths or arbitrary file access. The result is saved in the original directory with `.decrypted` added to the filename; the original file is preserved.

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

The resource carries plugin-defined dynamic arguments, up to 60 KiB when serialized:

```javascript
actions: [{
  id: "inspect-page-resource",
  data: {assetId: payload.id, expectedType: "video"}
}]
```

`action.data` is saved with the resource in `resources.db`. Store only persistable data such as content IDs and format options. Cookies, Authorization headers, page session tokens, and short-lived values such as `sessionBuffer` should remain in page memory and be used only after the page verifies the target.

The page script receives commands through its message listener and checks that the resource ID matches the current page:

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

By default, the app checks permissions and sends `data` to the matching page script. The page script decides how to process it. Ordinary page messages do not automatically appear in the app's interface.

The page can display its own feedback or return a result with `requestId` through `pageApi.send`, for the plugin's `onPageMessage` to handle. To generate a file, it can write data to Capture Store, report a resource, and create a download task through `enqueue-download`.

#### Execution ownership and progress reports

To display execution progress in the app, set `trackProgress: true` on the `page-command` action. Page commands require `inject-page-script` and `page-bridge`. Automatically creating download tasks also requires `enqueue-download`.

The page first checks that the command's content or resource ID matches the current page, then calls `claim` to request execution. It may start executing the command only after receiving `accepted: true`:

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

**Which page executes the command**

`claim(requestId)` ensures that only one page executes a command. Even if several pages claim it at the same time, only one receives `accepted: true`. Repeated claims return `accepted: false`. Pages that did not receive the command, and other plugins or scripts, cannot claim it.

A page with a different resource or one that is busy can report `state: "rejected"` before claiming. If all recipient pages reject the command, the app shows failure. Only the page that successfully claimed execution can update the command's state.

**Reporting state and progress**

After claiming execution, a page can report `running`, `completed`, `failed`, or `cancelled`. Once a command is completed, failed, or cancelled, its state cannot be changed again.

| Field or limit | Description |
| --- | --- |
| `progress` | Optional, or a finite number from 0 to 100. Omission means progress is unknown; no estimated percentage is generated |
| `message` | Up to 1024 UTF-8 bytes, displayed as plain text. Must not contain credentials, private URLs, or tokens |
| Complete request | Up to 4096 bytes. Unknown fields and invalid types are rejected |
| Recommended reporting frequency | At most once per second. Even when progress has not changed, report at least every 30 seconds to show that the task is still running |
| Request rate limit | Shared with ordinary messages: at most 100 requests per 10 seconds per session |

A command fails when:

- No page claims execution within 30 seconds of dispatch.
- No progress or heartbeat report arrives for 90 seconds during execution.
- Total running time exceeds 6 hours.

If a disconnected page cannot resume reporting progress, the app marks the command as failed when it times out. Reloading, disabling, or uninstalling the plugin also invalidates its existing commands.

**Restoring the connection after a page reload**

The page that successfully claims execution receives a short-lived `resumeToken`. If the plugin needs to reload the page automatically:

1. Temporarily save `resumeToken` in the current tab's `sessionStorage`.
2. After reloading, read the token and call `claim(requestId, resumeToken)` to restore the connection.
3. The app rechecks the plugin and script. Successful reconnection returns a new token and invalidates the old one. The old page can no longer report progress.

Limit automatic retries and token storage time, and delete the old token after use. If another reload is needed, retain only the latest token. Do not write tokens to resources, logs, fixtures, or URLs. A token cannot restore the connection after a plugin reload or app restart.

**Command records and display**

The app stores up to 256 command records. After a command completes, fails, or is cancelled, its record is retained for 10 minutes. Expired records are removed when querying status, sending commands, or reporting progress. While an operation is still active, the same plugin cannot start the same action on the same resource again. Command state is stored only in memory. It is not restored after an app restart or saved as a download task.

The resource list queries progress about every 1.5 seconds and displays the latest command's state, percentage, and message by `resourceId`. Once page processing finishes and a download task is created, the resource row switches to download or merge progress.

Ordinary `pageApi.send` replies do not automatically become progress information. Page command APIs cannot fabricate download tasks or specify file paths. The app currently has no button for cancelling page commands. Plugins can provide a cancel action on the webpage and report `cancelled`.

A complete sanitized example lives in `examples/plugins/page-command/`.

**Error codes**

The app uses fixed `errorCode` values for errors, and the UI displays the corresponding message in its current language. If starting an action fails, the local API response contains `code`, `message`, and `data.errorCode`. Errors after the command has started are returned in the command status's `errorCode`.

| Stage | Reason | `errorCode` |
| --- | --- | --- |
| Starting an action | The same action is still running | `page_command_already_active` |
| Starting an action | The command count limit has been reached | `page_command_limit_reached` |
| Starting an action | No eligible page is connected | `page_command_no_page` |
| Starting an action | Target page message queues are full | `page_command_queue_full` |
| Starting an action | The service is unavailable | `page_command_unavailable` |
| Starting an action | Message arguments are too large | `page_command_too_large` |
| Starting an action | Another startup error | `page_command_start_failed` |
| Waiting or executing | No page accepts the command | `page_command_not_accepted` |
| Waiting or executing | The page resource does not match or the page is busy | `page_command_target_unavailable` |
| Waiting or executing | The command times out | `page_command_timeout` |
| Waiting or executing | The plugin is reloaded | `page_command_reloaded` |

The app sets these codes; ordinary plugin progress reports cannot supply them.

The resource table's save-path column also displays page command progress and messages. Without a plugin message, it shows the app's status text. This content is not clickable. After a download task is created, the column shows download or processing progress; a clickable file path appears only after the download succeeds.

The app translates only its own state and error messages. A plugin's `message` is displayed unchanged, so the plugin must provide any translations it needs.

The UI queries command state through `POST /api/resources/page-commands`, which requires API session authentication. Each query waits up to 5 seconds, and failed queries are retried.

If a query fails, unfinished commands show **Progress unavailable** instead of the previous waiting state or percentage. This means progress cannot currently be retrieved, not that the command failed. Once the connection returns, the UI shows the latest state. If a new download task was created in the meantime, it shows that task's progress. A continuous query outage produces only one notification, rather than another popup on every retry.

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
- `rd_alloc` must return a valid linear memory address with space for the requested length. The app validates the address range.
- Input chunks are at most 256 KiB, and the module writes back into the same memory region. `rd_transform` returns an output byte count in `0..capacity`; a negative value means processing failed.
- `capacity` is currently 320 KiB. `offset_low/high` represent the current input's unsigned 64-bit byte offset.
- `final=1` marks the last call. If the file size is an exact multiple of the chunk size, the app makes one additional `rd_transform` call with input length 0 and `final` set to 1.
- `rd_free` is optional. If provided, it must release memory returned by `rd_alloc`. Modules must not retain addresses that the host has already freed.

WASM modules cannot import WASI or app-provided functions, so they cannot access networks, filesystems, environment variables, clocks, or system commands. Each module is limited to 8 MiB, linear memory to 64 MiB, and `options` to 64 KiB. Individual calls and the whole processing operation have time limits. If processing fails, the app deletes temporary output and preserves the original file. A complete example lives in `examples/plugins/wasm-xor/`.

## Declarative plugins

The declarative runtime is suitable for JSON APIs where one response directly produces a single-track resource:

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

The supported JSON Path syntax includes `$.a.b`, numeric array indexes, and a trailing `[*]`. Use the JavaScript runtime for combining multiple requests, correlation, or custom download plans.

Declarative fields:

| Field | Description |
| --- | --- |
| `stage` | `request` or `response`, usually `response` |
| `format` | Currently only `json` |
| `root` | Selects an object or an array of objects; defaults to the entire JSON |
| `resource.url` | Required download URL selector |
| `resource.title` / `coverUrl` | Optional title and cover URL |
| `resource.kind` | Resource kind; should match an ID declared in `resourceKinds` |
| `resource.role` | Track role; inferred from the last part of `kind` if omitted |
| `resource.executor` | Defaults to `http-file` |
| `resource.contentType` / `extension` / `size` | File type, extension, and byte count |
| `resource.preview` | `image`, `audio`, `video`, `pdf`, or `text` |
| `resource.metadata` | Custom metadata selectors; namespace site-specific fields |

Each selector uses `{path: "$.field"}` to read data or `{value: "constant"}` for a fixed value. Entries without a URL are skipped, and numeric `size` values are converted to integers. The declarative runtime does not execute custom code, retain state across responses, or generate multi-input download plans.

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

A fixture can contain one `observation` or use `observations` to replay several requests in order, such as page API, video, audio, and scrolling requests. Resources with the same `groupKey` are merged using the app's actual rules.

Expected results support `resourceCount`, `resourceUrls`, `processorTypes`, `decision`, and `patchBodyContains`. Templates live in `examples/plugins/`. See [Plugin SDK v1](plugin-sdk.md) for JSON Schema and TypeScript declarations.

Fixture guidelines:

- Keep only headers and body fields needed to match requests and generate resources.
- Retain real domains, but replace private paths, account details, and resource IDs with stable example values.
- Remove or replace cookies, Authorization headers, device identifiers, and temporary signatures.
- Use `observations` for a fixed replay order when testing correlation, while keeping the plugin independent of live request arrival order.
- Assert at least resource count and URLs. Processor plugins should also assert `processorTypes`.

## Debugging and common errors

Plugin cards show Manifest validation, entry compilation, and runtime errors. Troubleshoot in this order:

1. Run `plugin lint` and resolve directory, field, permission, and entry-file errors first.
2. Run `plugin replay` and confirm that the fixture consistently produces the expected resources.
3. Reload the plugin in the app and check the latest errors on its card and in the logs.
4. Confirm that the page generated new requests and that their domains and paths match `permissions.domains` and `match`.
5. Check body permissions, `readBody`, Content-Type, and `truncated`.
6. Finally, check whether site APIs, login state, or signatures have changed.

Common issues:

| Symptom | Common causes |
| --- | --- |
| Plugin fails to load | Invalid ID, version, API version, entry path, or permission dependency |
| Hook never runs | Stage, domain, path, method, or Content-Type does not match |
| Empty `response.body` | Missing read permission, `readBody: false`, or a different rule matched |
| JSON parsing fails | Truncated body, non-JSON response, or a login/anti-abuse page |
| Resource appears but cannot download | Missing required tracks, invalid URL, undeclared processor, or incomplete capabilities |
| Plugin temporarily stops processing | Consecutive errors or timeouts triggered suspension; fix the issue and reload |
| Page script is not injected | No TLS interception, compressed response, restrictive CSP, or no suitable injection point |
| Fixture passes but live traffic fails | Missing fixture branches, changed site data, expired credentials, or a different request order |

Each JavaScript hook may run for up to 5 seconds, including initialization and conversion of the returned result. Including queuing, the total limit is 10 seconds. Scripts running in webpages are not subject to that hook limit, but page commands still have limits on waiting for execution, progress reporting, and total running time.

Avoid processing oversized objects in loops. Do not log complete responses, cookies, or signed URLs. Prepare sanitized fixtures for issues that need repeated investigation.

## Pre-release checks

Before publishing, complete these recommended checks. See [Publishing requirements and recommendations](extension-store.md#publishing-requirements-and-recommendations) for the conditions required for store listing.

- The plugin has a stable ID following the [reserved ID rules](extension-store.md#plugin-sources-and-reserved-ids), and uses semantic versioning.
- Domains and capabilities are limited to actual needs.
- Manifest names, author information, descriptions, resource kinds, and settings have appropriate translations.
- At least one sanitized fixture passes offline replay. Complex branches and multi-request correlation have multiple fixtures.
- JavaScript, WASM, page scripts, and other runtime files are included in the plugin directory.
- Logs, fixtures, README, and example settings contain no account details, cookies, Authorization headers, or private URLs.
- `go run main.go plugin lint ...`, `go run main.go plugin replay ...`, and `go run main.go plugin pack ...` all succeed.
- For extension store releases providing a `dist/plugin.zip` acceleration package, it has been rebuilt and committed before tagging.
- Installation and basic operations have been checked with the final ZIP in the current stable app.

See [Publishing to the Extension Store](extension-store.md) for public repository structure, GitHub topic, release tag, and version requirements.

## Maintaining bundled plugins (project maintainers only)

This section covers the plugin snapshots shipped with the app. Independent plugin authors need only the development, packaging, and publishing steps above.

From the `res-downloader` source root, run `go run main.go plugin sync-bundled <plugin-directory>` to validate the source plugin and replace the old snapshot with the same ID under `internal/plugin/bundled/`, using the source directory's name.

Use `go run main.go plugin lint-bundled <directory>` to check a preinstalled snapshot. It accepts only IDs present in the application image used by the command, with directory contents exactly matching that embedded version. Ordinary plugin development uses `lint`; these commands do not grant official status.
