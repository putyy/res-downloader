---
description: Use the res-downloader CLI and MCP to manage resources and downloads, invoke plugin operations, and connect agents.
---

# CLI and MCP

Control the **running desktop app** to query captured resources, manage downloads, and invoke plugin operations. Tasks continue in the desktop app after a command exits or an agent disconnects. Quitting the desktop app stops background downloads.

Before use, launch the res-downloader app, enable capture as described in [Quick Start](getting-started.md), and open the target content. CLI / MCP do not automatically open websites, sign in, or resolve arbitrary webpage links into videos.

## Locate the executable

These examples assume `res-downloader` is in your PATH. Otherwise, replace it with the actual executable path.

On macOS, invoke the executable inside the app bundle:

```bash
/Applications/res-downloader.app/Contents/MacOS/res-downloader cli resources list --json
```

On Windows, invoke the program in the installation directory from PowerShell. Pipe the GUI executable's command output to receive it:

```powershell
& "C:\YourInstallDirectory\res-downloader.exe" cli resources list --json | Out-String
```

On Linux, use the installed executable's path. Do not use the installer package path in its place.

If you need a standard console program, for example to reliably wait for exit and read the exit code in Windows scripts, build the client from source:

```bash
go build -o resdctl ./cmd/resdctl
```

On Windows, use `go build -o resdctl.exe ./cmd/resdctl`. It accepts the same arguments as the desktop executable, such as `resdctl cli downloads list --json`. This client still requires the desktop app; it is not a standalone headless download service.

## Query resources and create downloads

```bash
# Show help; the desktop app does not need to be running
res-downloader cli --help

# Query captured resources; the default page size is 100
res-downloader cli resources list --offset 0 --limit 100 --json

# Create a download using an ID returned by the resource list
res-downloader cli downloads create --resource-id RESOURCE_ID --json
```

The resource list returns `data.items`, `data.total`, `data.recordCount`, and `data.nextOffset`. When `nextOffset` is greater than 0, use it as the next query's `--offset`; 0 means the end of the list. `--limit` accepts 1–5000. The result represents the saved resource catalog, with collection children in each item's `children`. Pagination results may change while capture continues.

Creating a download uses the desktop app's save directory, filename, concurrency, and media tool settings, and returns task information immediately. **Resource IDs and task IDs differ**: pass a resource ID when creating a download, then use the returned `data.id` to manage the task. An existing active task is reused instead of queued again.

## Query and manage tasks

```bash
res-downloader cli downloads list --json
res-downloader cli downloads get --id TASK_ID --json
res-downloader cli downloads pause --id TASK_ID --json
res-downloader cli downloads resume --id TASK_ID --json
res-downloader cli downloads cancel --id TASK_ID --json
res-downloader cli downloads retry --id TASK_ID --json
```

`list` returns an array of tasks; `get` returns one task. Status, progress, and output information are in `data`, with the same behavior as the desktop download center. Support for pause, resume, or retry depends on the task state and download executor. Unsupported operations return an error.

Commands output JSON by default; scripts may keep `--json` explicitly. Successful responses go to standard output, for example:

```json
{"code":1,"message":"ok","data":[]}
```

Errors go to standard error in the form `{"code":0,"message":"error description","errorCode":"error type"}`. Scripts should also check the exit code:

| Exit code | Meaning |
| --- | --- |
| `0` | Operation succeeded or help was displayed |
| `1` | Task operation, API, or MCP communication failed |
| `2` | Invalid command or arguments |
| `3` | Desktop app is unreachable, connection file is unavailable, or session has expired |

Each request has a default timeout of 15 seconds. Add `--timeout 30s` after a specific command to change it, up to `5m`. A timeout does not mean the operation did not happen. If a connection error occurs after creating or changing a task, query its status before deciding whether to retry. The client does not retry operations automatically.

## Discover and invoke plugin operations

Available operations depend on installed plugins. Enable automation for the required operations under the plugin card's **Advanced operations**, then open the matching webpage and sign in if needed. Select the target page when several match.

```bash
res-downloader cli operations list --args '{}' --json
res-downloader cli operations get --args '{"pluginId":"com.example.operations","operationId":"search"}' --json
res-downloader cli operations sessions --args '{}' --json
res-downloader cli operations invoke --args '{"pluginId":"com.example.operations","operationId":"search","pageSessionId":"PAGE_ID","input":{"query":"example"},"limit":2,"idempotencyKey":"search-example-1"}' --json
res-downloader cli operations execution --args '{"id":"EXECUTION_ID"}' --json
res-downloader cli operations history --args '{"offset":0,"limit":50}' --json
```

This example uses the repository's demo plugin; replace the plugin, operation and page IDs with your own. Submission returns an `executionId` immediately. Query `data.state`, `data.resultStatus` and `data.result` to check the outcome. After a request times out, check history before deciding whether to retry.

See `cli --help` for batch, cancellation and artifact arguments, and [the operation contract](../development/operations.md) for detailed rules.

## Configure MCP

MCP exposes the same operations as tools that an agent can discover and call. It uses a **stdio** connection with this startup command:

```bash
res-downloader mcp --stdio
```

An MCP-compatible agent client should launch the process. Running it manually in a terminal leaves it waiting for protocol input.

For clients that support the `mcpServers` configuration format, add the following and replace `command` with the actual absolute path on your computer:

```json
{
  "mcpServers": {
    "res-downloader": {
      "command": "/Applications/res-downloader.app/Contents/MacOS/res-downloader",
      "args": ["mcp", "--stdio"]
    }
  }
}
```

On Windows, `command` can be `C:\\YourInstallDirectory\\res-downloader.exe`; on Linux, use the installed executable's absolute path. You can also use a client you built as `resdctl` / `resdctl.exe`. Configuration locations and formats vary between agent clients; use the same command and arguments in that client's MCP settings.

There is no need to copy a token. The client and desktop app should run as the same operating system user. Sandboxed agents need permission to read that user's connection file and access the local control port.

For example, ask your agent: “Find the videos just captured, download the one titled XXX, and tell me its progress.” The agent can call `list_resources`, `create_download`, and `get_download` in sequence.

Actual calls require the desktop app to be running. Restarting the desktop app usually does not require restarting the MCP process.

## Connection and troubleshooting

The desktop app writes its current connection information to `control/session.json` in the user configuration directory and removes it on normal exit. The connection uses a separate local port; there is no need to change the capture proxy's Host / Port settings.

Default connection file locations:

| System | Path |
| --- | --- |
| Windows | `%APPDATA%\res-downloader\control\session.json` |
| macOS | `~/Library/Preferences/res-downloader/control/session.json` |
| Linux | `${XDG_CONFIG_HOME:-$HOME/.config}/res-downloader/control/session.json` |

The connection file contains session credentials. Do not share it or commit it to a repository.

To specify another connection file, add `--session-file PATH` after a specific CLI or MCP command. This changes only the client's lookup path, not the desktop app's data directory.

- **Cannot read the connection file**: confirm you are running a version with CLI / MCP support and that the client and desktop app use the same system user.
- **Connection failed or session expired**: restart the desktop app, then query task status. A crash may leave a stale connection file; the next launch replaces it.
- **Desktop app works but connections always fail**: check the [application logs](troubleshooting.md#find-application-logs) for a `start local automation service` error. Check configuration directory permissions and the agent sandbox's local network access.
- **Empty resource list**: enable capture and play the target content first, and check the selected capture types. MCP does not browse websites or sign in.
- **Tools stop working after the desktop app exits**: this is expected. This version has no standalone headless service.
