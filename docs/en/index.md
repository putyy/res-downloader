---
layout: home
title: res-downloader - Cross-platform Resource Discovery and Downloads
titleTemplate: false
description: res-downloader is an easy-to-use resource discovery and download tool for Windows, macOS, and Linux, with network capture, previews, plugins, download management, and CLI / MCP automation.
hero:
  name: res-downloader
  text: Discover. Preview. Download.
  tagline: A simple cross-platform tool for finding, previewing, and downloading network resources.
  actions:
    - theme: brand
      text: Quick Start
      link: /en/guide/getting-started
    - theme: alt
      text: Download and Install
      link: /en/guide/installation
    - theme: alt
      text: GitHub
      link: https://github.com/putyy/res-downloader
features:
  - title: Cross-platform
    details: A clear, easy-to-use interface for discovering and downloading resources on Windows, macOS, and Linux.
    link: /en/guide/installation
    linkText: Installation guide
  - title: Network Capture
    details: Capture HTTP / HTTPS resources from browsers, phones, and desktop apps, including video, audio, images, and more.
    link: /en/guide/examples
    linkText: Explore resource capture
  - title: Sites and Plugins
    details: A WeChat Channels plugin comes preinstalled. Add dedicated detection, download, and processing support for Douyin and other sites through the extension store.
    link: /en/guide/plugin-management
    linkText: Install and manage plugins
  - title: Download Center
    details: Track progress, pause, resume, cancel, retry, and review download history in a dedicated task view.
    link: /en/guide/getting-started#_4-manage-downloads
    linkText: Start managing downloads
  - title: HLS and Live Streams
    details: Preview and download M3U8 video on demand, preview HLS / FLV live streams, and record live streams after configuring FFmpeg.
    link: /en/guide/settings#media-processing
    linkText: Configure media tools
  - title: CLI and MCP
    details: Use commands or an agent to query captured resources, create downloads, and manage tasks while the desktop app is running.
    link: /en/guide/automation
    linkText: Connect commands and agents
---

## Download and install

Download an installer from [GitHub Releases](https://github.com/putyy/res-downloader/releases) or [Lanzou Cloud](https://wwjv.lanzoum.com/b04wgtfyb) (access code: `9vs5`). Choose the version for your operating system and CPU architecture. See the [installation guide](guide/installation.md) for details.

Windows 7 only supports version `2.3.0` in the [legacy archive](https://github.com/putyy/res-downloader/tree/old), without current features. A [Mini edition](https://github.com/putyy/resd-mini) is also available.

## How to use it

1. **Install and launch**: allow the network access requested by your operating system.
2. **Install the certificate**: open **Setting → Certificate** and install this device's certificate.
3. **Start capturing**: return to **Intercept** and click **Start Grabbing**.
4. **Open the content**: select resource types, then open the content in a browser, phone, or desktop app.
5. **Preview and download**: create downloads from the resource list and track their progress in **Downloads**.

See [Quick Start](guide/getting-started.md) for the full workflow and an app screenshot. Phones need their own proxy and certificate setup; see [connecting a phone](guide/troubleshooting.md#connect-a-phone). For capture or download issues, see [Troubleshooting](guide/troubleshooting.md).

## How it works

res-downloader uses a local proxy to discover available resources in network requests. Filtering, previews, and download controls make web resources easier to manage. Plugins add dedicated detection and processing for specific sites.

Developers can read the [architecture guide](development/architecture.md) to understand the internals, or start with [plugin development](development/plugins.md) and the [Plugin SDK](development/plugin-sdk.md) to support a new site.

## Contribute

Report bugs and suggest features through [GitHub Issues](https://github.com/putyy/res-downloader/issues). Pull requests are welcome; please read the [contribution guide](development/contributing.md) before submitting code.

The maintainer currently has limited time, so issue responses, PR reviews, and releases may take longer. Thank you to everyone who uses, tests, and contributes to the project.

[Release notes](https://github.com/putyy/res-downloader/releases) · [User group (Chinese)](https://www.putyy.com/app/admin/upload/img/20250418/6801d9554dc7.webp)

> Make sure you have the rights to the resources you process and comply with local laws, platform terms, and copyright requirements.
