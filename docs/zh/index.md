---
layout: home
title: res-downloader 爱享素材下载器 - 跨平台资源发现与下载工具
titleTemplate: false
description: res-downloader（爱享素材下载器）是一款简洁易用的跨平台资源发现与下载工具，支持 Windows、macOS、Linux，提供网络抓取、资源预览、插件扩展、下载管理和 CLI / MCP 功能。
hero:
  name: res-downloader
  text: 爱享素材下载器
  tagline: 简洁易用的跨平台资源发现与下载工具，支持多种资源抓取、预览和下载。
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 下载与安装
      link: /guide/installation
    - theme: alt
      text: GitHub
      link: https://github.com/putyy/res-downloader
features:
  - title: 跨平台使用
    details: 支持 Windows、macOS 和 Linux，操作简单，界面清晰，提供统一的资源发现与下载体验。
    link: /guide/installation
    linkText: 查看安装指南
  - title: 网络资源抓取
    details: 捕获浏览器、手机和桌面应用中的 HTTP / HTTPS 资源，支持视频、音频、图片等多种类型。
    link: /guide/examples
    linkText: 了解资源抓取
  - title: 站点与插件扩展
    details: 预装微信视频号插件，抖音等站点可通过插件商店扩展，为更多站点增加专用识别、下载和处理能力。
    link: /guide/plugin-management
    linkText: 管理与安装插件
  - title: 下载任务中心
    details: 独立管理下载进度、暂停、继续、取消、重试和历史记录，让资源获取与下载任务各有条理。
    link: /guide/getting-started#_4-查看下载
    linkText: 开始使用下载任务
  - title: HLS 与直播
    details: 支持 M3U8 点播预览与下载，HLS / FLV 直播可直接预览，配置 FFmpeg 后可以录制直播。
    link: /guide/settings#媒体处理
    linkText: 配置媒体处理
  - title: CLI 与 MCP
    details: 通过命令或 Agent 查询已抓取资源、创建下载和管理任务，使用时需要桌面应用保持运行。
    link: /guide/automation
    linkText: 接入命令与 Agent
---

## 下载与安装

从 [GitHub Releases](https://github.com/putyy/res-downloader/releases) 或[蓝奏云](https://wwjv.lanzoum.com/b04wgtfyb)下载安装包，蓝奏云访问密码为 `9vs5`。请按操作系统和 CPU 架构选择对应版本，具体步骤见[安装指南](guide/installation.md)。

Windows 7 仅可使用[旧版归档](https://github.com/putyy/res-downloader/tree/old)中的 `2.3.0`，不支持当前版本功能。另有 [Mini 版](https://github.com/putyy/resd-mini)可供了解。

## 使用方法

1. **安装并启动**：按系统提示允许必要的网络访问。
2. **安装证书**：进入“系统设置 → 证书”，安装当前设备证书。
3. **开启抓取**：返回“获取资源”页面，点击“开启抓取”。
4. **访问目标内容**：选择抓取类型，在浏览器、手机或桌面应用中打开需要的内容。
5. **预览与下载**：从资源列表创建下载，并在“下载任务”页面管理进度。

完整操作和软件界面见[快速开始](guide/getting-started.md)。手机需要单独配置代理与证书，见[手机接入说明](guide/troubleshooting.md#手机如何接入)；抓取或下载遇到问题时，可查看[常见问题](guide/troubleshooting.md)。

## 工作原理

res-downloader 通过本地代理发现网络请求中的可用资源，提供直观的筛选、预览和下载操作，方便管理网页资源。插件为特定站点提供专用的识别与处理能力。

开发者可以查看[架构说明](development/architecture.md)了解内部组成，或从[插件开发指南](development/plugins.md)和[插件 SDK](development/plugin-sdk.md)开始适配新站点。

## 参与项目

欢迎通过 [GitHub Issues](https://github.com/putyy/res-downloader/issues) 反馈问题和功能建议，也欢迎提交 Pull Request。提交代码前请阅读[参与贡献](development/contributing.md)。

维护者近期可投入的时间有限，Issue 回复、PR Review 和版本发布可能有所延迟。感谢每一位使用者、反馈者和贡献者的支持。

[查看更新日志](https://github.com/putyy/res-downloader/releases)

## 交流群

欢迎交流软件使用和插件开发。添加微信 `AmorousWorld`，由维护者邀请加入交流群。

> 请确保对所处理的资源拥有合法权利，并遵守所在地法律、平台协议和版权规定。
