---
description: Package res-downloader locally on macOS, Windows, and Linux with Docker, including prerequisites, commands, and output files.
---

# Local packaging

Run all commands from the repository root. Set the application version in `build/config.yml` under `info.version`. Output files are written to `bin/`.

## Prerequisites

Native builds require Go (see `go.mod` for the required version), Node.js 22.12 or later, and a Wails v3 CLI matching `go.mod`. Linux Docker builds only require Git and Docker on the host; the image installs the build tools.

Packaging tasks use local plugin sources. Before the first build, prepare the repositories listed in `build/bundled-plugins.json`, currently:

```bash
git clone https://github.com/putyy/resd-plugin-wechat.git plugins/resd-plugin-wechat
```

See [Maintaining bundled plugins](plugins.md#maintaining-bundled-plugins) for configuration details.

## macOS

```bash
wails3 task darwin:package
```

This creates `bin/res-downloader.app` and `bin/res-downloader.dmg` for the current machine's architecture. To include both Intel and Apple Silicon, run:

```bash
wails3 task darwin:package:universal
```

## Windows

Install NSIS and add `makensis.exe` to `PATH`, then run on Windows:

```powershell
wails3 task windows:package ARCH=amd64
```

This creates `bin/res-downloader-amd64-installer.exe`. For ARM64, change `ARCH` to `arm64`; the output is `bin/res-downloader-arm64-installer.exe`.

## Linux: Docker packaging

Run these Bash / Zsh commands in the same terminal; on Windows, use WSL. The image is based on Ubuntu 24.04.

### 1. Build the image

Use `amd64` for AMD64 or change the first line to `arm64` for ARM64. Cross-architecture builds require Docker emulation support.

```bash
BUILD_ARCH=amd64
BUILD_IMAGE="res-downloader-linux-${BUILD_ARCH}"
docker build --platform "linux/$BUILD_ARCH" -f build/linux/dockerfile -t "$BUILD_IMAGE" .
```

### 2. Create the DEB

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

Output: `bin/res-downloader_<version>_linux_<arch>.deb` and an executable with the same name without the extension. Build different architectures sequentially.

These commands target stable versions. For prereleases, pass the application version and Debian package version separately, for example `RELEASE_VERSION=4.0.0-rc.1` and `PACKAGE_VERSION=4.0.0~rc.1`.

### Network issues

If dependency downloads fail, try adding `--network host` after `docker build` or `docker run`. Docker Desktop 4.34 or later requires **Enable host networking** in Settings. For base-image pull failures, check Docker networking configuration. See [Docker host networking](https://docs.docker.com/engine/network/drivers/host/).

### Optional: AppImage

Reuse the image and architecture variables above:

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

The output is `bin/res-downloader-x86_64.AppImage` or `bin/res-downloader-aarch64.AppImage`. Cross-architecture emulation can cause packaging tools to fail with `Exec format error`; prefer a native Linux environment matching the target architecture.

Prefer DEB on Ubuntu 24.04. AppImage does not install an AppArmor profile; switch to DEB if you encounter a `bwrap` permission error.

## Linux: native packaging

Install GTK4 and WebKitGTK 6.0 development packages, plus packaging tools including `dpkg-dev`, `binutils`, and `desktop-file-utils`, then run:

```bash
wails3 task linux:create:deb
```

The output is `bin/res-downloader.deb`. Use `wails3 task linux:package` to create both DEB and AppImage.
