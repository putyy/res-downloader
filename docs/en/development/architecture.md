---
description: Understand the Wails, Vue, and Go architecture of res-downloader, including proxy capture, plugins, download tasks, CLI / MCP, and persistent state.
---

# Architecture

res-downloader is a cross-platform desktop app built with Wails. The Vue frontend handles interaction and state presentation. The Go backend handles the local proxy, certificates and system integration, resource detection, plugin execution, and download tasks. A versioned plugin protocol extends site support without putting frequently changing site logic into the host.

This page describes module boundaries, runtime dependencies, and main data flows. See [Plugin Development](plugins.md) for complete plugin fields, permissions, and runtime APIs.

## Overview

```text
Browser / phone / desktop app
          │ HTTP / HTTPS proxy traffic
          ▼
┌──────────────────────────────────────────────────────────────┐
│ server.Gateway: shared Host:Port listener                     │
│                                                              │
│  Local /api requests ──► httpapi.Server                       │
│  Other requests      ──► proxy.Engine (goproxy)                │
└──────────┬───────────────────────────────┬────────────────────┘
           │                               │
           │ Settings, resources,          │ Request/response
           │ tasks, plugins                │ observations
           ▼                               ▼
┌────────────────────┐          ┌──────────────────────────────┐
│ Runtime services   │          │ plugin.PluginManager         │
│                    │          │ - Built-in generic detector  │
│ resource.Resource  │◄─────────│ - Bundled official plugins   │
│ download.Scheduler │          │ - User-installed plugins     │
│ media.Engine       │          └───────────────┬──────────────┘
│ system.Setup       │                          │ ResourceCandidate
└─────────┬──────────┘                         ▼
          │                         Resource catalog,
          │ HTTP responses /        correlation, persistence
          │ Wails events                       │
          ▼                                    │
┌──────────────────────────────────────────────┴───────────────┐
│ Vue + Pinia: resources, downloads, plugins, settings          │
└──────────────────────────────────────────────────────────────┘
```

`internal/app.Runtime` is the application's composition root. It creates modules and injects dependencies, but construction does not start listeners or background tasks. Wails startup calls `Runtime.Start`; shutdown calls `Runtime.Close`.

CLI / MCP invoke existing HTTP business handlers through a separate local control entry point: `Agent / Shell → internal/automation → internal/control → httpapi.ControlHandler → resource and download services`. Clients do not construct the desktop Runtime, open business databases, or start the download scheduler.

## Runtime components

| Module | Main responsibility | Key dependencies or output |
| --- | --- | --- |
| Wails | Load the embedded frontend, provide window lifecycle and a few Bind methods | `frontend/dist`, `app.Bind` |
| `app.Runtime` | Create modules, wire dependencies, and manage startup and shutdown order | Settings, events, proxy, HTTP, plugins, resources, downloads |
| `server.Gateway` | Manage the capture proxy listener and route local API versus proxy requests | `httpapi.Server`, `proxy.Engine` |
| `control.Server` | Manage the local automation listener, separate credentials, and discovery file | Loopback only; reuses resource and download HTTP handlers |
| `automation` | Convert CLI commands and MCP stdio tools into local control requests | Reads connection information on every call; does not start desktop Runtime |
| `proxy.Engine` | Handle HTTP proxying and HTTPS MITM and generate request/response observations | Interception rules, device certificate, plugin manager |
| `plugin.PluginManager` | Load plugins, run observation hooks, correlate resources, refresh URLs, and generate download plans | Built-in, bundled, and user plugins |
| `resource.Resource` | Maintain the catalog, persist resource candidates, and execute resource actions and download plans | `resources.db`, capture cache, media engine |
| `download.Scheduler` | Persist tasks, manage workers and task state, and handle pause, resume, and retry | `tasks.db`, plugin download plans |
| `download.PlanRunner` | Acquire inputs, process them, and place final output for a download plan | HTTP, HLS, captured files, FFmpeg, WASM |
| `capture.Store` | Cache response bytes captured by the proxy or page scripts for download plans | `capture-cache/` |
| `media.Engine` | Invoke user-configured FFmpeg / ffprobe for media processing | Remuxing, merging, recording, and related capabilities |
| `system.Setup` | Manage device certificates, system proxies, and platform operations | Windows, macOS, and Linux implementations |
| `events.Emitter` | Encode resource and task changes and send them to the Wails frontend | A single `event` channel |

## Startup and shutdown lifecycle

### Construction

`NewRuntime` performs the following in dependency order:

1. Create application directories, logging, and the event emitter.
2. Initialize this device's certificate authority. Certificate failures are recorded in application state for the UI to explain.
3. Load settings and create the response capture cache, media engine, and system integration service.
4. Open the resource database and restore the catalog.
5. Load interception rules, plugin state, bundled plugins, and user plugins.
6. Open the task database and restore the download queue.
7. Connect plugins, resource services, and the download scheduler.
8. Create the proxy engine, local API, and shared HTTP gateway.

When settings are applied, the runtime updates upstream proxy transport, download worker count, and HTTPS interception rules as needed without reconstructing the entire app.

### Startup

`Runtime.Start` initializes proxy handlers, starts the shared HTTP gateway, then starts the download scheduler. The scheduler restores persisted tasks: waiting tasks are queued again; tasks previously resolving, downloading, or processing are marked interrupted until the user chooses to resume or retry.

Next, `control.Server` starts on a dynamic `127.0.0.1` port and writes `control/session.json`, protected by current-user access permissions. This entry point uses a separate random token and a route allowlist, independent of the capture proxy's Host / Port settings. Automation startup failures are logged; desktop capture and downloads continue.

### Shutdown

`Runtime.Close` first closes the automation service and removes this instance's connection file. It then attempts to disable the app-managed system proxy, stops the HTTP gateway and download scheduler, and closes the resource database, capture cache, and logging. A cleanup/reset operation removes application state and restarts only after these resources have been released.

## Resource capture flow

1. The user enables the system proxy or manually connects other devices and apps to the configured listen address.
2. `server.Gateway` accepts the connection. Recognized local `/api` requests go to the HTTP API; other requests go to `proxy.Engine`.
3. For HTTPS CONNECT, `rules.Set` decides between MITM and pass-through based on domain policy. Only host information is available at this point; resource-type and MIME matching happen later during observation.
4. The proxy converts requests or responses into versioned `Observation` objects. It reads bodies, bounded by `bodyLimit`, only when matching rules and permissions require them.
5. `plugin.PluginManager` invokes enabled plugins by priority, including the built-in generic detector, bundled official plugins, and user plugins. When a user triggers a `page-command` resource action, it also delivers a host-generated standard message to the bridge-enabled page script declared by that plugin.
6. Plugins can emit resource candidates, correlate requests, request controlled response modifications or page scripts, and declare capture tasks that write response bytes to `capture.Store`.
7. The resource service normalizes, correlates, and saves emitted `ResourceCandidate` objects in the in-memory catalog and `resources.db`.
8. Wails events push changes to the frontend, which updates the resource list using the current filters.

Reading a request body reconnects the consumed prefix to the original stream. Response observation likewise preserves the complete response for the client. Observation should not change normal proxy transport unless a plugin explicitly requests and returns a validated modification.

## Download execution flow

1. The frontend requests a download through the local API. `download.Scheduler` creates a task record and writes it to `tasks.db`.
2. The scheduler takes tasks from the queue according to configured concurrency. Collections split into child tasks while retaining parent task state.
3. If a resource has expired or requires refresh, the scheduler first asks its original plugin to refresh it. If this is impossible, it returns an explicit state requiring recapture.
4. The plugin creates a `DownloadPlan` describing inputs, executors, processing steps, and final output rather than directly controlling host objects.
5. `resource.Resource` calculates the final path from the filename template and conflict policy, then passes the plan to `download.PlanRunner`.
6. PlanRunner acquires inputs using HTTP, HLS, captured files, FFmpeg, or other executors, and runs host processing or plugin WASM processors as needed.
7. Completed temporary output is atomically placed at the final path. Progress and state are persisted in the task database and sent to the frontend through events.

Plain HTTP and captured-file plans can retain checkpoint state. Pause/resume support for live recording, HLS, or media processing depends on the executor. When the app exits, unfinished tasks are recorded as resumable or interrupted instead of keeping background processes running.

## Frontend/backend communication

The frontend does not reference internal Go modules directly. It primarily uses three channels:

- **Wails Bind**: a small set of startup capabilities such as app information, a settings snapshot, the API session token, and reset operations.
- **Local HTTP API**: resources, previews, download tasks, plugins, certificates, and settings. Except for a few public certificate-download or HLS-preview paths, requests require a per-launch random bearer token and pass origin and method checks.
- **Wails events**: the backend wraps business events as `{type, data}` and sends them through a single `event` channel. A Pinia event store dispatches them to pages.

The Wails AssetServer uses the same HTTP API middleware so the embedded frontend can access `/api`. The gateway's separate listener allows proxy traffic, phone certificate downloads, and local previews to share the configured Host and Port.

CLI and MCP use an additional local control listener for resource queries and download operations. It rejects browser Origins, non-local Hosts, and routes outside its allowlist. Its control token cannot be used directly with the desktop API. Clients ignore environment proxies, do not follow redirects, and do not automatically retry writes. MCP uses the official Go SDK for stdio; standard output contains protocol messages only. Tools query business progress. See [CLI and MCP](../guide/automation.md) for usage.

## Plugin system boundaries

Plugins receive structured observations and return structured resource candidates or download plans. They do not hold Go objects or have arbitrary access to the filesystem, shell, or host network interfaces. The host validates Manifests, domains, capabilities, body limits, page scripts, resource actions, download plans, and processor declarations at load and execution time. `page-command` sends only plugin-defined arguments from a saved resource to a bridge-enabled script in that same plugin. It does not expose general page control to the desktop frontend.

Plugins have three sources:

- **Built-in plugins**: implemented in Go, such as the generic resource detector.
- **Bundled official plugins**: source snapshots in `internal/plugin/bundled/`, shipped with the app and installed into the user's plugin directory.
- **User plugins**: installed under `plugins/` in the user data directory, from the extension store or a local ZIP.

Site-specific API detection, correlation, signature calculation, URL refresh, and non-DRM byte transformations belong in plugins. Extend the host API only for reusable capabilities that multiple plugins may need and that the existing protocol cannot safely express. Such changes must synchronize Go models, runtime validation, Schema, TypeScript declarations, examples, and documentation.

See [Plugin Development](plugins.md), [Plugin SDK v1](plugin-sdk.md), and [Extension Store Publishing](extension-store.md) for details.

## Data and state

Application state lives in the operating system's user configuration directory for `res-downloader`:

| Path | Contents | Lifecycle |
| --- | --- | --- |
| `config.json` | UI, listen address, proxy, download, naming, and media tool settings | Atomically updated when settings are saved |
| `logs/app.log` and timestamped backups | Release-build logs and enabled plugin debug logs | 10 MiB per file, up to 5 backups; backups older than 7 days are cleaned on open and rotation. See [logs](../guide/troubleshooting.md#find-application-logs) |
| `mitm-ca.crt`, `mitm-ca.key` | Device-generated HTTPS interception certificate and private key | Cleared on reset |
| `resources.db` | Persistent catalog of discovered resources | Updated when resources are cleared or the app is reset |
| `tasks.db` | Download tasks, children, progress, and recovery state | Updated with task changes |
| `control/session.json` | This launch's automation address and separate token | Written at automation startup, removed on normal shutdown, replaced on the next launch after a crash |
| `capture-cache/` | Temporary bytes from proxy responses and page media segments | Expires and is cleared on reset |
| `plugins/` | Installed external and bundled plugin copies | Updated on install, upgrade, rollback, or uninstall |
| `plugin-state.json` | Plugin enablement state | Updated when plugin settings change |
| `plugin-settings.json` | Per-plugin user settings | Updated when plugin settings are saved |
| `plugin-removed.json` | Records of bundled plugins explicitly removed by the user | Prevents unexpected restoration on upgrade |
| `plugin-sources.json` | Installed plugin origins | Updated on installation and upgrade |
| `plugin-backups/` | One retained rollback copy for external plugin upgrades | Updated on upgrade, rollback, and uninstall |

If the resource or task database cannot open, that module logs an error and falls back to in-memory state. The app can still start, but that session's state cannot survive a restart.

## Main directories

| Directory | Responsibility |
| --- | --- |
| `frontend/` | Vue, Pinia, Naive UI frontend and Wails-generated bridge code |
| `internal/app/` | Composition root, Wails Bind, and backend module adapters |
| `internal/server/` | Shared listener and API/proxy routing, without business handlers |
| `internal/control/` | Local control listener, discovery, and platform-specific credential file permissions |
| `internal/automation/` | CLI arguments, MCP tools, and shared local API client |
| `internal/httpapi/` | Local API, authentication, previews, and desktop operations |
| `internal/proxy/` | HTTP proxy, HTTPS MITM, observations, and page script injection |
| `internal/rules/` | Domain interception and pass-through policies for CONNECT |
| `internal/plugin/` | Plugin loading, permissions, runtime, store, installation, and developer CLI |
| `internal/resource/` | Catalog, correlation, persistence, resource actions, and download plan entry point |
| `internal/download/` | Task scheduling, persistence, executors, and plan runner |
| `internal/capture/` | Temporary response cache supporting range writes or appended segments |
| `internal/media/` | FFmpeg / ffprobe capability detection and process execution |
| `internal/system/` | Certificates, system proxies, and platform differences |
| `internal/config/` | Defaults, validation, snapshots, and persistence |
| `internal/events/` | Events from Go to the Wails frontend |
| `internal/model/` | Shared resource, plugin, and download protocol models |
| `internal/naming/` | Download filename templates and conflict policies |
| `internal/logging/` | Application logging wrapper |
| `examples/plugins/` | Committable protocol examples and sanitized fixtures |
| `plugins/` | Local workspace for standalone site plugins; excluded from host commits and packages by default |
| `cmd/extension-index/` | Script generating the store index from GitHub repositories |
| `cmd/resdctl/` | Separately buildable CLI / MCP console client; still requires the running desktop app |
| `build/` | Wails platform configuration, icons, and installer resources |
| `docs/` | User guides, Plugin SDK, and developer documentation |

## Change boundaries

- Add or fix site support in standalone plugins first. Do not add private site logic to `internal/proxy/` or `internal/resource/`.
- For new host plugin capabilities, define the generic protocol and permissions first, then synchronize runtime, validation, SDK files, examples, and documentation.
- When changing the local API, check origins, authentication, methods, body limits, and frontend types together.
- When changing download states or resource models, check BoltDB recovery, event payloads, and frontend state mappings.
- Keep constructors free of listener and background-task side effects when changing startup/shutdown, and release created resources on failure paths.
- Keep platform-specific certificate and proxy behavior in `internal/system/`. Shared business code should call it through interfaces or adapters.

Read [Contributing](contributing.md) before developing code. For plugin work, also read [Plugin Development](plugins.md) and the relevant SDK documentation.
