---
description: 在本地打包 res-downloader，包含 macOS、Windows 和 Linux Docker 的环境准备、构建命令与产物说明。
---

# 本地打包

所有命令从仓库根目录执行。应用版本在 `build/config.yml` 的 `info.version` 中设置，产物输出到 `bin/`。

## 准备工作

本机打包需要 Go（版本要求见 `go.mod`）、Node.js 22.12 或更高版本，以及与 `go.mod` 一致的 Wails v3 CLI。Linux Docker 打包只需在宿主机准备 Git 和 Docker，镜像内会安装构建工具。

打包任务使用本地插件源码。首次打包前，按 `build/bundled-plugins.json` 准备插件仓库，当前为：

```bash
git clone https://github.com/putyy/resd-plugin-wechat.git plugins/resd-plugin-wechat
```

配置见[维护应用内嵌插件](plugins.md#维护应用内嵌插件)。

## macOS

```bash
wails3 task darwin:package
```

生成 `bin/res-downloader.app` 和 `bin/res-downloader.dmg`，使用当前机器架构。需要同时支持 Intel 与 Apple Silicon 时执行：

```bash
wails3 task darwin:package:universal
```

## Windows

额外安装 NSIS，并将 `makensis.exe` 加入 `PATH`，在 Windows 上执行：

```powershell
wails3 task windows:package ARCH=amd64
```

生成 `bin/res-downloader-amd64-installer.exe`。ARM64 将 `ARCH` 改为 `arm64`，对应产物为 `bin/res-downloader-arm64-installer.exe`。

## Linux：Docker 打包

以下命令使用 Bash / Zsh，在同一个终端依次执行；Windows 可在 WSL 中操作。镜像基于 Ubuntu 24.04。

### 1. 构建镜像

AMD64 使用 `amd64`，ARM64 将第一行改为 `arm64`。跨架构构建需要 Docker 支持模拟运行。

```bash
BUILD_ARCH=amd64
BUILD_IMAGE="res-downloader-linux-${BUILD_ARCH}"
docker build --platform "linux/$BUILD_ARCH" -f build/linux/dockerfile -t "$BUILD_IMAGE" .
```

### 2. 生成 DEB

```bash
docker run --rm --platform "linux/$BUILD_ARCH" \
  -e "ARCH=$BUILD_ARCH" \
  -v "$PWD:/www/res-downloader" -w /www/res-downloader \
  "$BUILD_IMAGE" bash -c '
set -euo pipefail
VERSION=$(sed -n "s/^  version: *//p" build/config.yml)
test -n "$VERSION"
wails3 task linux:create:deb "ARCH=$ARCH" "RELEASE_VERSION=$VERSION" "PACKAGE_VERSION=$VERSION"
mv bin/res-downloader.deb "bin/res-downloader_${VERSION}_linux_${ARCH}.deb"
cp bin/res-downloader "bin/res-downloader_${VERSION}_linux_${ARCH}"
'
```

产物：`bin/res-downloader_<版本>_linux_<架构>.deb` 和同名无后缀程序文件。不同架构需依次打包。

上述命令适用于正式版本。预发布时将任务参数分别设为应用版本和 Debian 包版本，例如 `RELEASE_VERSION=4.0.0-rc.1`、`PACKAGE_VERSION=4.0.0~rc.1`。

### 网络问题

依赖下载遇到网络问题时，可在 `docker build` 或 `docker run` 后添加 `--network host`。Docker Desktop 4.34 及以上需先启用 **Enable host networking**；基础镜像拉取问题需检查 Docker 网络配置。参见 [Docker host 网络说明](https://docs.docker.com/engine/network/drivers/host/)。

### 可选：AppImage

沿用上面的镜像和架构变量：

```bash
docker run --rm --platform "linux/$BUILD_ARCH" \
  -e "ARCH=$BUILD_ARCH" \
  -v "$PWD:/www/res-downloader" -w /www/res-downloader \
  "$BUILD_IMAGE" bash -c '
set -euo pipefail
VERSION=$(sed -n "s/^  version: *//p" build/config.yml)
test -n "$VERSION"
wails3 task linux:create:appimage "ARCH=$ARCH" "RELEASE_VERSION=$VERSION"
'
```

产物为 `bin/res-downloader-x86_64.AppImage` 或 `bin/res-downloader-aarch64.AppImage`。跨架构模拟可能导致打包工具报 `Exec format error`，建议在目标架构的原生 Linux 环境生成。

Ubuntu 24.04 优先使用 DEB；AppImage 不安装 AppArmor 策略，遇到 `bwrap` 权限错误时改用 DEB。

## Linux：本机打包

安装 GTK4、WebKitGTK 6.0 开发包，以及 `dpkg-dev`、`binutils`、`desktop-file-utils` 等打包工具后执行：

```bash
wails3 task linux:create:deb
```

生成 `bin/res-downloader.deb`。需要同时生成 DEB 和 AppImage 时使用 `wails3 task linux:package`。
