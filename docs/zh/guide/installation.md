---
description: res-downloader 安装指南：选择适合 Windows、macOS 或 Linux 及 CPU 架构的安装包，完成安装与首次启动。
---

# 安装指南

请选择与操作系统和 CPU 架构匹配的安装包，可通过以下任一方式下载：

- [GitHub Releases](https://github.com/putyy/res-downloader/releases)
- [蓝奏云下载](https://wwjv.lanzoum.com/b04wgtfyb)，访问密码：`9vs5`

## Windows

::: warning 大多数 Windows 电脑请选择 amd64
**大多数使用 Intel 或 AMD 处理器的 64 位 Windows 电脑，应下载安装包名称中架构标识为 `amd64` 的版本。** `amd64` 同时适用于 Intel 和 AMD，并不只是 AMD 处理器。只有使用 ARM 处理器的 Windows 设备才选择 `arm64`。
:::

下载 Windows 安装包并按提示完成安装。首次安装证书时，请允许系统显示的 UAC 授权。

普通安装包使用系统 WebView2 Runtime；`fixed_webview2` 安装包自带固定版本。切换安装包类型时，先退出软件，再覆盖安装到原目录。普通版会使用系统运行时，并清理新版安装器记录过的固定版文件；旧版未记录的文件和目录中的其他文件会保留，但不会被普通版作为运行时加载。

安装中途失败时，请参照[安装失败后如何重试](troubleshooting.md#windows-安装失败后如何重试)。

Windows 7 只能使用旧版归档中的 `2.3.0`。

## macOS

下载 `.dmg` 文件，将 `res-downloader.app` 拖入“应用程序”。

首次启动如果被系统阻止，请在“系统设置 → 隐私与安全性”中允许打开。

## Linux

Debian、Ubuntu 等系统可以安装对应架构的 `.deb` 文件：

```bash
sudo apt install ./res-downloader_<version>_linux_amd64.deb
```

使用独立程序时，先添加执行权限后再启动：

```bash
chmod +x ./res-downloader_<version>_linux_amd64
./res-downloader_<version>_linux_amd64
```

安装完成后，继续查看[快速开始](getting-started.md)。
