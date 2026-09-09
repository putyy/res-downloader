<div align="center">

<a href="https://github.com/putyy/res-downloader"><img src="build/appicon.png" width="120"/></a>
<h1>res-downloader</h1>
<h4>📖 English | <a href="https://github.com/putyy/res-downloader/blob/master/README.md">中文</a></h4>

[![GitHub stars](https://img.shields.io/github/stars/putyy/res-downloader)](https://github.com/putyy/res-downloader/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/putyy/res-downloader)](https://github.com/putyy/res-downloader/fork)
[![GitHub release](https://img.shields.io/github/release/putyy/res-downloader)](https://github.com/putyy/res-downloader/releases)
![GitHub All Releases](https://img.shields.io/github/downloads/putyy/res-downloader/total)
[![License](https://img.shields.io/github/license/putyy/res-downloader)](https://github.com/putyy/res-downloader/blob/master/LICENSE)

</div>

---

### 🎉 Aixiang Resource Downloader

> A cross-platform resource downloader built with Go + [Wails](https://github.com/wailsapp/wails).  
Clean UI, easy to use, and supports a wide range of resource sniffing and downloading.

## ✨ Features

- 🚀 **User-Friendly**: Simple operation with an intuitive and beautiful UI
- 🖥️ **Cross-Platform**: Available on Windows / macOS / Linux
- 🌐 **Supports Multiple Resource Types**: Video / Audio / Images / m3u8 / Live streams, and more
- 📱 **Wide Platform Compatibility**: Works with WeChat Channels, Mini Programs, Douyin, Kuaishou, Xiaohongshu, KuGou Music, QQ Music, and more
- 🌍 **Proxy Capture**: Built-in proxy allows fetching resources behind network restrictions

## 📚 Docs & Versions

- 📘 [Online Documentation (Chinese)](https://res.putyy.com/)
- 🧩 [Mini Version Ui Display using default browser](https://github.com/putyy/res-downloader) ｜ [Old Electron Version Support Win7](https://github.com/putyy/res-downloader/tree/old)
- 💬 [Join the User Group (Chinese)](https://www.putyy.com/app/admin/upload/img/20250418/6801d9554dc7.webp)
  > *If full, you can add WeChat `AmorousWorld` with a note “github”*

## 🧩 Download Links

- 🆕 [Download from GitHub](https://github.com/putyy/res-downloader/releases)
- 🆕 [Download via Lanzou Cloud (Password: 9vs5)](https://wwjv.lanzoum.com/b04wgtfyb)
- ⚠️ *Windows 7 users: Please use version `2.3.0`*


## 🖼️ Preview

![Preview](docs/images/show.webp)

## 🚀 How to Use

> Follow these steps to use the software correctly:

1. During installation, be sure to **allow certificate installation** and **grant network access**
2. Launch the software → Click **"Start Proxy"** at the top left
3. Choose the resource types to capture (default is all)
4. Open the target content externally (WeChat, Mini App, Browser, etc.)
5. Return to the homepage to view the captured resource list

---

## 🛠️ Local Development

The desktop app uses Go + Wails v2.12.0, with Vue 3 + TypeScript + Vite in `frontend/`. Run Wails commands from the repository root, where `wails.json` is located.

### Prerequisites

- Go 1.23.2 or newer (`go.mod` declares Go 1.22.0 and toolchain `go1.23.2`). On macOS 15+, use Go 1.23.3 or newer.
- Node.js with npm. The Linux build image uses Node.js 20; the locked frontend dependencies require at least Node.js 18.12.
- Platform dependencies: Xcode Command Line Tools on macOS (`xcode-select --install`), WebView2 Runtime on Windows, or a C compiler, GTK3 and WebKitGTK development packages on Linux. See the [Wails installation guide](https://wails.io/docs/gettingstarted/installation/) for platform setup.

Install the CLI version matching `go.mod`, then check your environment:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails doctor
```

If `wails` is not found, add Go's binary directory to `PATH` (normally `~/go/bin` on macOS/Linux or `%USERPROFILE%\go\bin` on Windows). For macOS/Linux with the default Go binary location:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Start development

```sh
git clone https://github.com/putyy/res-downloader.git
cd res-downloader
wails dev
```

If you already have a checkout, run `wails dev` from its root. Wails uses these settings in [`wails.json`](./wails.json):

| Setting | Command / behavior |
| --- | --- |
| `frontend:install` | `npm install` in `frontend/` when dependencies need installing |
| `frontend:dev:watcher` | `npm run dev` starts Vite |
| `frontend:dev:serverUrl` | `auto` detects Vite's development URL |
| `frontend:build` | `npm run build` checks TypeScript and builds frontend assets |

Use the desktop window for development. Wails reloads frontend changes and rebuilds/restarts the app for Go changes. Running Vite alone does not start the Go backend required by the app. See the [Wails CLI reference](https://wails.io/docs/reference/cli/) for development flags.

On Linux systems using WebKitGTK 4.1, add the tag to both development and build commands:

```sh
wails dev -tags webkit2_41
wails build -tags webkit2_41
```

### Build and check changes

```sh
# From the repository root: type-check and build only the frontend
npm --prefix frontend ci
npm --prefix frontend run build

# Build the desktop app for your current platform
wails build
```

Desktop output is written to `build/bin/`. See [packaging instructions](./build/README.md) for platform-specific installers and release packages.

Backend code lives in `core/`, the desktop entry point is `main.go`, and UI code lives in `frontend/src/`. Wails generates the Go/JavaScript bindings in `frontend/wailsjs/`; update the Go methods and regenerate through Wails instead of editing those bindings manually. Read the [contributing guide](./CONTRIBUTING.md) before submitting a PR.

## ❓ FAQ

### 📺 m3u8 Video Resources

- Online Preview: [m3u8play](https://m3u8play.com/)
- Download Tool: [m3u8-down](https://m3u8-down.gowas.cn/)

### 📡 Live Stream Resources

- We recommend [OBS](https://obsproject.com/) for recording (search for setup tutorials)

### 🐢 Slow Downloads or Large File Failures?

- Recommended download managers:
    - [Neat Download Manager](https://www.neatdownloadmanager.com/index.php/en/)
    - [Motrix](https://motrix.app/download)
- For WeChat videos, click `Decrypt Video` after download

### 🧩 Unable to Intercept Resources?

- Check your system proxy settings:  
  Address: 127.0.0.1  
  Port: 8899

### 🌐 Can't Access Internet After Closing the App?

- Manually disable the system proxy settings

### 🧠 More Questions?

- [GitHub Issues](https://github.com/putyy/res-downloader/issues)
- [Aixiang Forum Thread (Chinese)](https://s.gowas.cn/d/4089)

## 💡 Principles & Motivation

This tool captures traffic via a local proxy and filters useful resources.  
Its working principle is similar to tools like Fiddler, Charles, or browser DevTools, but with a more user-friendly display and enhanced filtering, making it suitable for everyday users with minimal tech background.

---

## ⚠️ Disclaimer

> This software is for educational and research purposes only.  
Commercial or illegal use is strictly prohibited.  
The author is not responsible for any consequences arising from misuse.
