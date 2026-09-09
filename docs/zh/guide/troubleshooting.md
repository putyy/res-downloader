---
description: 排查 res-downloader 的安装、证书、资源抓取、手机接入和下载问题，了解日志位置、CLI / MCP 连接与直播录制设置。
---

# 常见问题

## 视频号资源太多

在“插件管理”中关闭微信视频号插件的“完整抓取模式”，清空列表后重新打开目标详情页。

## 资源过多提醒

资源记录达到 1500 条时，软件提示资源较多、可能增加内存占用并影响运行稳定性；此后每增加 500 条再次提醒。数量包含合集子资源，不受当前分页、筛选或展开状态影响。启动恢复和导入资源也会检查数量，一批资源跨过多个阈值只提醒一次。

提醒不代表已经检测到内存不足，也不会自动清理资源。可使用资源列表现有的清理入口删除不再需要的资源；清理后按剩余数量重新计算下一次提醒阈值。

## 某个网站或应用无法捕获

部分应用不支持代理抓取，或需要专用插件。先确认抓取已开启、证书已安装，目标域名没有设置为“直接透传”。

## 安装证书后仍提示未安装

进入“系统设置 → 证书”，重新安装证书并刷新状态。Windows 请允许 UAC 授权；手机上的旧证书需要手动删除后重新安装。

## 手机如何接入

1. 手机和电脑连接同一网络。
2. 将监听地址设为 `0.0.0.0`，重启应用并开启抓取。
3. 手机代理填写电脑的局域网 IP 和应用端口。
4. 在手机上下载安装当前证书并开启信任。

部分手机应用即使完成设置也可能无法捕获。

## 应用退出后无法上网

在系统网络设置中关闭 HTTP / HTTPS 代理，然后重新启动应用。

## 应用无法启动或配置异常

使用“清理缓存并重启”，并按提示授权。该操作不会删除已下载文件和已安装插件。

## Windows 安装失败后如何重试

新版安装器会在写入应用文件前记录已通过校验的安装目录。首次安装因磁盘空间、文件占用或 WebView2 权限设置失败时，先解决错误提示中的原因，再运行安装包并选择同一目录即可重试，不必删除已经留下的文件。

如果是旧版安装器留下的未注册目录，新版无法确认其归属时仍会拒绝覆盖。请保留该目录，选择另一个空目录安装，或向 [GitHub Issues](https://github.com/putyy/res-downloader/issues) 反馈；不要为绕过检查而删除整个目录。

## Windows 无法卸载

如果提示文件正在使用，请关闭软件或重启电脑后重试。如果提示安装记录无效，可下载最新版安装包，尝试覆盖安装到原目录后再卸载；若安装器拒绝该目录，请保留原目录并向 [GitHub Issues](https://github.com/putyy/res-downloader/issues) 反馈错误提示，不要直接删除整个目录。

卸载会清理已记录的应用和固定版 WebView2 文件，保留下载、配置、插件及其他未知文件。不同 WebView2 安装包之间的切换方式见[安装指南](installation.md#windows)。

## CLI 或 MCP 无法连接

CLI 和 MCP 需要更新后的桌面应用保持运行，并与应用使用同一操作系统用户。连接文件不可用、会话失效、Agent 沙箱权限或资源列表为空等问题，请查看 [CLI 与 MCP：连接与排查](automation.md#连接与排查)。

## 如何查看软件日志

正式版会把运行日志写入 `app.log`。如果应用可以进入主界面，可在“系统设置 → 基础设置”中点击“打开日志目录”。请先复现问题并退出应用，再复制日志文件用于排查。

### Windows

日志路径：

```text
%APPDATA%\res-downloader\logs\app.log
```

按 `Win + R`，输入 `%APPDATA%\res-downloader\logs` 并回车，即可在资源管理器中打开日志目录。也可以在 PowerShell 中查看最后 200 行：

```powershell
Get-Content "$env:APPDATA\res-downloader\logs\app.log" -Tail 200
```

### macOS

日志路径：

```text
~/Library/Preferences/res-downloader/logs/app.log
```

在访达中按 `Command + Shift + G`，输入 `~/Library/Preferences/res-downloader/logs` 并回车。也可以通过终端打开日志目录或查看最后 200 行：

```bash
open ~/Library/Preferences/res-downloader/logs
tail -n 200 ~/Library/Preferences/res-downloader/logs/app.log
```

### Linux

默认日志路径：

```text
~/.config/res-downloader/logs/app.log
```

如果设置了 `XDG_CONFIG_HOME`，日志会保存在 `$XDG_CONFIG_HOME/res-downloader/logs/app.log`。可以在终端查看最后 200 行：

```bash
tail -n 200 "${XDG_CONFIG_HOME:-$HOME/.config}/res-downloader/logs/app.log"
```

如果日志目录或 `app.log` 不存在，应用可能在日志系统初始化前就已退出。反馈问题时请同时提供操作系统版本、系统架构、应用版本、安装包文件名、复现步骤和截图。

提交日志前请先检查并隐藏其中可能出现的账号、Cookie、Token、资源地址、本地用户名和文件路径等隐私信息。

## macOS 提示“已损坏，无法打开”

确认安装包来自项目官方发布页后，在“系统设置 → 隐私与安全性”中允许打开。仍无法打开时执行：

```bash
sudo xattr -d com.apple.quarantine /Applications/res-downloader.app
```

## 下载慢或大文件失败

检查保存空间、链接是否过期和下载代理是否可用。视频号链接过期时需要重新抓取。

## 下载后的文件名变成 resource-开头

软件会先使用文件命名模板生成目标名称，并自动清理不兼容符号和过长名称。如果操作系统仍拒绝该名称，下载完成后的安装步骤会改用 `resource-<短标识>.扩展名` 保存，避免丢弃已经下载和处理的数据。实际保存路径以任务中心显示为准；两次安装都失败时，临时输出会保留在当前保存目录的 `.res-downloader-work` 中，并可通过删除失败任务、清理已结束任务或“清理缓存并重启”删除。如需排查原名称被拒绝的原因，请提供脱敏后的日志。

## 直播录制无法使用

先安装 FFmpeg，再到“系统设置 → 媒体处理”中点击检测。

## 仍然无法解决

请前往 [GitHub Issues](https://github.com/putyy/res-downloader/issues) 反馈，并附上系统、应用版本、复现步骤和错误信息。请先隐藏账号、Cookie 等隐私数据。
