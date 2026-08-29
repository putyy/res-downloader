# 安装指南

请从 [GitHub Releases](https://github.com/putyy/res-downloader/releases) 下载与操作系统和 CPU 架构匹配的安装包。

## Windows

下载 Windows 安装包并按提示完成安装。首次安装证书时，请允许系统显示的 UAC 授权。

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
