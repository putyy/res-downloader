# 常见问题

## 视频号资源太多

在“插件管理”中关闭微信视频号插件的“完整抓取模式”，清空列表后重新打开目标详情页。

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

## 直播录制无法使用

先安装 FFmpeg，再到“系统设置 → 媒体处理”中点击检测。

## 仍然无法解决

请前往 [GitHub Issues](https://github.com/putyy/res-downloader/issues) 反馈，并附上系统、应用版本、复现步骤和错误信息。请先隐藏账号、Cookie 等隐私数据。
