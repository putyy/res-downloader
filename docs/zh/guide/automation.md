---
description: 使用 res-downloader CLI 与 MCP 查询已抓取资源、创建下载和管理任务，了解客户端配置、连接方式及常见故障。
---

# CLI 与 MCP

CLI 和 MCP 用于控制**正在运行的桌面应用**，支持查询已抓取资源、创建下载和管理下载任务。命令执行完或 Agent 断开后，任务继续由桌面应用运行；退出桌面应用会停止后台下载。

使用前先启动更新后的桌面应用，按[快速开始](getting-started.md)开启抓取并访问目标内容。CLI / MCP 不会自动打开网页、登录网站或把任意网页链接解析为视频。

## 找到可执行文件

下面的示例假设 `res-downloader` 已在 PATH 中；否则请替换成实际可执行文件路径。

macOS 直接调用应用包内的程序：

```bash
/Applications/res-downloader.app/Contents/MacOS/res-downloader cli resources list --json
```

Windows 在 PowerShell 中调用安装目录内的程序。通过管道接收 GUI 可执行文件的命令输出：

```powershell
& "C:\实际安装目录\res-downloader.exe" cli resources list --json | Out-String
```

Linux 调用安装后的程序路径。不要使用安装包路径代替已经安装的可执行文件。

如果需要标准控制台程序（例如 Windows 脚本需要可靠等待进程退出并读取退出码），可从源码构建客户端：

```bash
go build -o resdctl ./cmd/resdctl
```

Windows 使用 `go build -o resdctl.exe ./cmd/resdctl`。其参数与桌面程序相同，例如 `resdctl cli downloads list --json`。这个客户端仍然需要桌面应用运行，不是独立无界面下载服务。

## 查询资源和创建下载

```bash
# 查看帮助，无需桌面应用运行
res-downloader cli --help

# 查询已抓取资源，默认每页 100 条
res-downloader cli resources list --offset 0 --limit 100 --json

# 使用资源列表返回的资源 ID 创建下载
res-downloader cli downloads create --resource-id RESOURCE_ID --json
```

资源列表返回 `data.items`、`data.total`、`data.recordCount` 和 `data.nextOffset`。`nextOffset` 大于 0 时，可作为下一次查询的 `--offset`；为 0 表示已到末尾。`--limit` 范围为 1–5000。返回已保存的资源目录，合集子资源位于对应条目的 `children` 内；抓取持续进行时，分页数据也可能变化。

创建下载使用桌面的保存目录、文件命名、并发数及媒体工具设置，立即返回任务信息。**资源 ID 和任务 ID 不同**：创建时传资源 ID，后续管理时使用返回的 `data.id`。已有活动任务时复用该任务，不重复排队。

## 查询和管理任务

```bash
res-downloader cli downloads list --json
res-downloader cli downloads get --id TASK_ID --json
res-downloader cli downloads pause --id TASK_ID --json
res-downloader cli downloads resume --id TASK_ID --json
res-downloader cli downloads cancel --id TASK_ID --json
res-downloader cli downloads retry --id TASK_ID --json
```

`list` 返回任务数组，`get` 返回单个任务。任务状态、进度和输出信息均在 `data` 内，行为与桌面任务中心一致。是否支持暂停、恢复或重试取决于任务状态和下载执行器；不支持时会返回错误。

命令默认输出 JSON，`--json` 可显式保留在脚本中。成功响应写到标准输出，例如：

```json
{"code":1,"message":"ok","data":[]}
```

错误写到标准错误，格式为 `{"code":0,"message":"错误说明","errorCode":"错误类型"}`。脚本应同时检查退出码：

| 退出码 | 含义 |
| --- | --- |
| `0` | 操作成功或显示帮助 |
| `1` | 任务操作、API 或 MCP 通信失败 |
| `2` | 命令或参数错误 |
| `3` | 桌面应用不可连接、连接文件不可用或会话已失效 |

每次请求默认超时 15 秒，可在具体命令后添加 `--timeout 30s`，最大为 `5m`。超时不等于操作没有发生；创建下载或修改任务后遇到连接错误，应先查询任务状态，再决定是否重试。客户端不会自动重试操作。

## 配置 MCP

MCP 把相同操作提供为 Agent 可以发现和调用的工具。使用 **stdio** 连接，启动命令为：

```bash
res-downloader mcp --stdio
```

应由支持 MCP 的 Agent 客户端启动该进程。手动在终端执行时，它会等待协议输入。

在支持 `mcpServers` 配置格式的客户端中，添加以下配置，并将 `command` 改为本机的真实绝对路径：

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

Windows 的 `command` 可填写 `C:\\实际安装目录\\res-downloader.exe`；Linux 填写安装后的程序绝对路径。也可使用自行构建的 `resdctl` / `resdctl.exe`。不同 Agent 客户端的配置文件位置和格式可能不同，按其 MCP 设置填写同样的命令与参数即可。

无需复制 Token。客户端与桌面应用应使用同一操作系统用户；沙箱中的 Agent 需要能够读取该用户的连接文件并访问本机控制端口。

| MCP 工具 | 参数 | 用途 |
| --- | --- | --- |
| `list_resources` | 可选 `offset`、`limit` | 分页查询已抓取资源 |
| `create_download` | `resourceId` | 创建下载，返回任务 |
| `list_downloads` | 无 | 查询全部任务 |
| `get_download` | `id` | 查询单个任务 |
| `pause_download` | `id` | 暂停任务 |
| `resume_download` | `id` | 恢复任务 |
| `cancel_download` | `id` | 取消任务 |
| `retry_download` | `id` | 重试任务 |

例如，对 Agent 说“查询刚才抓到的视频，下载其中标题为 XXX 的资源，然后告诉我进度”。Agent 可以依次调用 `list_resources`、`create_download` 和 `get_download`。

工具成功时返回 JSON 文本及同样内容的 `structuredContent`；业务错误使用 MCP 的 `isError` 标记。查询工具标记为只读，任务操作标记为会修改状态，客户端可据此展示操作确认。

MCP 可以在桌面应用启动前完成连接和工具发现，但实际调用需要桌面应用运行。每次工具调用都会重新读取当前连接信息，因此桌面应用重启后通常无需重启 MCP 进程。标准输出仅用于 MCP 消息，诊断信息写入标准错误。

## 连接与排查

桌面应用启动后，在用户配置目录的 `control/session.json` 中写入本次连接信息，正常退出时删除。控制服务仅监听 `127.0.0.1` 上的动态端口，独立于抓取代理的 Host / Port 设置。连接凭据每次启动随机生成，只允许上述资源和任务操作，不能用于修改设置、安装证书或控制系统代理。

默认连接文件位置：

| 系统 | 路径 |
| --- | --- |
| Windows | `%APPDATA%\res-downloader\control\session.json` |
| macOS | `~/Library/Preferences/res-downloader/control/session.json` |
| Linux | `${XDG_CONFIG_HOME:-$HOME/.config}/res-downloader/control/session.json` |

macOS / Linux 的控制目录仅供当前用户访问；Windows 使用仅允许当前用户与 SYSTEM 的访问控制。不要共享连接文件或把其中内容提交到仓库。

需要指定连接文件位置时，在具体 CLI 命令或 MCP 命令后添加 `--session-file PATH`。这只改变客户端查找位置，不会改变桌面应用的数据目录。

- **提示无法读取连接文件**：确认运行的是包含 CLI / MCP 功能的新版本，且客户端与桌面应用属于同一系统用户。
- **提示连接失败或会话失效**：重新启动桌面应用，再查询任务状态。应用异常退出可能留下旧连接文件，新启动会替换它。
- **桌面正常但一直无法连接**：查看[应用日志](troubleshooting.md#如何查看软件日志)中是否有 `start local automation service` 错误，检查配置目录权限及 Agent 沙箱的本机网络权限。
- **资源列表为空**：先开启抓取并播放目标内容，检查所选抓取类型。MCP 不负责网页浏览和登录。
- **桌面应用退出后工具不可用**：这是预期行为，本版本没有独立无界面服务。
