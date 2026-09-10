---
description: Install res-downloader on Windows, macOS, or Linux. Choose the package for your operating system and CPU architecture and complete the first launch.
---

# Installation

Choose the package matching your operating system and CPU architecture. Download it from either source:

- [GitHub Releases](https://github.com/putyy/res-downloader/releases)
- [Lanzou Cloud](https://wwjv.lanzoum.com/b04wgtfyb), access code: `9vs5`

## Windows

::: warning Choose amd64 for most Windows PCs
**Most 64-bit Windows PCs with Intel or AMD processors should use the installer labeled `amd64`.** The `amd64` architecture supports both Intel and AMD processors, not just AMD. Choose `arm64` only for Windows devices with an ARM processor.
:::

Download the Windows installer and follow its instructions. Approve the UAC prompt when installing the certificate for the first time.

The standard installer uses the system WebView2 Runtime; the `fixed_webview2` installer bundles a fixed version. To switch between them, close the app and install over the same directory. After switching to the standard version, the app uses the system runtime, and the installer removes only fixed WebView2 files listed in installation records. Untracked legacy files and other files in the directory are preserved, but the standard version will not load them as its runtime.

If installation stops partway through, see [Retry a failed Windows installation](troubleshooting.md#retry-a-failed-windows-installation).

Windows 7 only supports version `2.3.0` from the legacy archive.

## macOS

Download the `.dmg` file and drag `res-downloader.app` to **Applications**.

If macOS blocks the first launch, allow the app under **System Settings → Privacy & Security**.

## Linux

### Debian / Ubuntu

On Debian, Ubuntu, and similar systems, install the `.deb` package for your architecture:

```bash
sudo apt install ./res-downloader_<version>_linux_amd64.deb
```

### Arch Linux

If `yay` is installed, update your system and install [res-downloader from the AUR](https://aur.archlinux.org/packages/res-downloader):

```bash
yay -Syu res-downloader
```

Since 2024, Arch Linux has provided only `webkit2gtk-4.1`, with `webkit2gtk-4.0` no longer available.

::: details Building from source or maintaining the AUR package
When building from source or maintaining the PKGBUILD, use WebKit2GTK 4.1:

1. Replace `webkit2gtk-4.0` with `webkit2gtk-4.1` in `depends` / `makedepends`.
2. Add the `-tags webkit2_41` build tag to `wails build` in `build()`. For example, to build for amd64:

```bash
wails build -platform "linux/amd64" -tags webkit2_41 -upx
```

`-upx` compresses the executable and requires UPX to be installed; omit it if compression is unnecessary. When installing through `yay` with a PKGBUILD that already includes these settings, no manual changes are needed.
:::

### Standalone executable

For a standalone executable, add execute permission before launching:

```bash
chmod +x ./res-downloader_<version>_linux_amd64
./res-downloader_<version>_linux_amd64
```

Once installed, continue to [Quick Start](getting-started.md).
