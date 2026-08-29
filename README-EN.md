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

> A cross-platform app for discovering, previewing, and downloading network resources. It provides a clean interface and supports a wide range of media and file types.

## ✨ Features

- 🚀 **User-Friendly**: Simple operation with an intuitive and beautiful UI
- 🖥️ **Cross-Platform**: Available on Windows / macOS / Linux
- 🌐 **Supports Multiple Resource Types**: Video / Audio / Images / M3U8 / Live streams, and more
- 📱 **Site Support**: Includes the WeChat Channels plugin; Douyin and other sites are available through the plugin store
- 🌍 **Network Capture**: Finds HTTP and HTTPS resources used by browsers, phones, and desktop apps
- 🧩 **Plugin Extensions**: Adds site-specific discovery, download, and processing capabilities
- 📥 **Download Center**: Manages progress, pause, resume, cancellation, retry, and history in one place
- 📡 **HLS & Live**: Previews and downloads M3U8 video and records live streams after FFmpeg is configured

## 📚 Docs & Versions

- 📘 [Online Documentation (Chinese)](https://res.putyy.com/)
- 🧩 [Plugin Management (Chinese)](./docs/plugin-management.md)
- 🛠️ [Plugin Development](./docs/plugins.md)
- 🤝 [Contributing](./CONTRIBUTING.md)
- 🧩 [Releases](https://github.com/putyy/res-downloader/releases) ｜ [Mini Version](https://github.com/putyy/resd-mini) ｜ [Legacy Archive](https://github.com/putyy/res-downloader/tree/old)
- 💬 [Join the User Group (Chinese)](https://www.putyy.com/app/admin/upload/img/20250418/6801d9554dc7.webp)
  > *If full, you can add WeChat `AmorousWorld` with a note “github”*

## 🧩 Download Links

- 🆕 [Download from GitHub](https://github.com/putyy/res-downloader/releases)
- 🆕 [Download via Lanzou Cloud (Password: 9vs5)](https://wwjv.lanzoum.com/b04wgtfyb)
- ⚠️ *Windows 7 can only use legacy version `2.3.0`, which does not include the current release's features.*

## 🚀 How to Use

> Follow these steps to use the software correctly:

1. Install and launch the app, allowing network access when prompted by the system.
2. Open **Setting → Certificate** and install the certificate for this device.
3. Return to **Intercept** and click **Start Grabbing**.
4. Select capture types, then visit the target content in a browser, phone, or desktop app.
5. Download from the resource list and manage created jobs in **Downloads**.

---

## ❓ FAQ

### 📺 HLS / M3U8 Video Resources

- Built-in HLS preview and download support relative segment URLs, master playlists, AES keys, and captured request headers.

### 📡 Live Stream Resources

- HLS and FLV streams can be previewed directly. After configuring a user-installed FFmpeg, live streams can be recorded and stopped with a valid saved output.

### 🔐 Encrypted WeChat Channels Resources

- Downloads created in the app run the plugin processing pipeline automatically. If an encrypted file was downloaded from a copied link with another tool, use “Decrypt Local Video” from the matching resource menu.

### 🧩 Unable to Intercept Resources?

- Confirm that the device certificate is installed and capture is enabled. The default proxy is `127.0.0.1:8899`; if the listener was changed, use the value shown in System Settings.

### 🌐 Can't Access Internet After Closing the App?

- A normal app shutdown disables the proxy managed by the app. If the process exited unexpectedly, disable the proxy manually in the operating system's network settings.

### 🧠 More Questions?

- [GitHub Issues](https://github.com/putyy/res-downloader/issues)
- [Aixiang Forum Thread (Chinese)](https://s.gowas.cn/d/4089)

## 💡 How It Works

The app uses a local proxy to find useful resources in network requests, then provides approachable filtering, preview, and download controls. Developers can read the [architecture documentation](./docs/architecture.md) for internal details.

---

## ⚠️ Disclaimer

> This project is released under the [Apache License 2.0](./LICENSE). See LICENSE for details.

Users are responsible for ensuring that they have the legal right to process each resource and that their use complies with applicable laws, platform terms, and copyright requirements. The software is provided on an “AS IS” basis; the disclaimers and limitations of liability in the Apache License 2.0 apply.

## ©️ Copyright & Brand

Copyright © 2023–present putyy and res-downloader contributors.

The Apache License 2.0 does not grant permission to use the `res-downloader` name or project logo in a way that implies official status, partnership, or endorsement. Reasonable attribution remains permitted. See [NOTICE](./NOTICE).
