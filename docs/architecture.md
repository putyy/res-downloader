# 架构说明

res-downloader 由 Wails 桌面界面、Go 后端、网络代理、下载任务和插件系统组成。

## 工作流程

1. 浏览器、手机或桌面应用通过本地代理访问网络。
2. 应用识别请求中的图片、视频、音频和其他资源。
3. 通用规则或网站插件整理资源信息。
4. 用户创建下载任务后，由任务中心负责下载和处理文件。

## 主要目录

- `frontend/`：桌面界面。
- `internal/app/`：应用启动、关闭和模块连接。
- `internal/proxy/`：网络代理与资源观察。
- `internal/plugin/`：插件加载和运行。
- `internal/download/`：下载任务。
- `internal/system/`：证书和系统代理。
- `internal/resource/`：资源记录。

站点适配应优先通过插件实现。插件开发请查看[插件开发指南](plugins.md)。

参与代码开发前，请先阅读[参与贡献](contributing.md)。
