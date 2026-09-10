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

普通安装包使用系统 WebView2 Runtime；`fixed_webview2` 安装包自带固定版本。切换安装包类型时，先退出软件，再覆盖安装到原目录。切换到普通版后，应用会使用系统运行时，安装器仅清理安装记录中列出的固定版 WebView2 文件；未记录的遗留文件和目录中的其他文件会保留，但不会被普通版作为运行时加载。

安装中途失败时，请参照[安装失败后如何重试](troubleshooting.md#windows-安装失败后如何重试)。

Windows 7 只能使用旧版归档中的 `2.3.0`。

## macOS

下载 `.dmg` 文件，将 `res-downloader.app` 拖入“应用程序”。

首次启动如果被系统阻止，请在“系统设置 → 隐私与安全性”中允许打开。

## Linux

### Debian / Ubuntu

Debian、Ubuntu 等系统可以安装对应架构的 `.deb` 文件：

```bash
sudo apt install ./res-downloader_<version>_linux_amd64.deb
```

### Arch Linux

已安装 `yay` 的用户可以通过以下命令更新系统并安装 [AUR 中的 res-downloader](https://aur.archlinux.org/packages/res-downloader)：

```bash
yay -Syu res-downloader
```

Arch Linux 自 2024 年起仅提供 `webkit2gtk-4.1`，不再提供 `webkit2gtk-4.0`。

::: details 自行构建或维护 AUR 包
自行构建或维护 PKGBUILD 时，需要使用 WebKit2GTK 4.1：

1. 将 `depends` / `makedepends` 中的 `webkit2gtk-4.0` 依赖改为 `webkit2gtk-4.1`。
2. 在 `build()` 中为 `wails build` 添加 `-tags webkit2_41` 构建标签。例如，构建 amd64 版本：

```bash
wails build -platform "linux/amd64" -tags webkit2_41 -upx
```

`-upx` 用于压缩可执行文件，需要安装 UPX；不需要压缩时可省略。通过 `yay` 安装且 PKGBUILD 已包含上述配置时，无需手动调整。
:::

### 独立程序

使用独立程序时，先添加执行权限后再启动：

```bash
chmod +x ./res-downloader_<version>_linux_amd64
./res-downloader_<version>_linux_amd64
```

安装完成后，继续查看[快速开始](getting-started.md)。
