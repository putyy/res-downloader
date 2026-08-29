# resd-plugin-wechat

`res-downloader` 的微信视频号资源插件，用于识别视频号视频和图片，并处理需要解密的视频文件。

## 功能

- 从微信视频号页面捕获视频和图片资源。
- 支持按可用清晰度选择下载画质。
- 使用插件自带的 WASM 处理加密视频。
- 支持预览、下载、打开、复制和解密本地视频。

## 安装

发布后可在 `res-downloader` 的“插件管理”页面安装。也可以下载对应版本的源码 ZIP，通过“从压缩包安装”导入。

插件会申请访问微信和腾讯媒体域名，并申请读取及修改相关页面响应、处理下载文件等权限。安装时请核对权限提示。

## 设置

- `完整抓取模式`：从页面媒体对象捕获资源；关闭后仅从详情捕获。
- `下载画质`：按资源实际提供的清晰度选择默认、超清、高清、中等或低清。

## 开发与校验

在 `res-downloader` 项目根目录执行：

```bash
go run main.go plugin lint ./plugins/resd-plugin-wechat
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/video.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/image.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/page-script.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/inject-hook.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/finder-video.json
go run main.go plugin pack ./plugins/resd-plugin-wechat
```

Fixture 只包含脱敏后的虚构数据和示例地址。

## License

[MIT](LICENSE)
