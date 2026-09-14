# resd-plugin-wechat

`res-downloader` 的微信视频号资源插件，用于识别视频号视频和图片，并处理需要解密的视频文件。

## 功能

- 从微信视频号页面捕获视频和图片资源。
- 支持按可用清晰度选择下载画质。
- 使用插件自带的 WASM 处理加密视频。
- 支持预览、下载、打开、复制和解密本地视频。

## 安装

发布后可在 `res-downloader` 的“插件管理”页面安装。也可以下载对应版本的源码 ZIP，通过“从压缩包安装”导入。

## 设置

- `完整抓取模式`：从页面媒体对象和详情捕获资源；关闭后仅从详情捕获。
- `下载画质`：按资源实际提供的清晰度选择默认、超清、高清、中等或低清。
- `启用日志`：默认关闭，保留为通用调试开关；当前插件没有主动输出调试日志。

### 支持的魔法变量

| 变量 | 说明 |
| --- | --- |
| `{{title}}` | 作品标题 |
| `{{created_at}}` | 作品创建日期，按本地时区格式化，默认 `20060102`；支持 `{{created_at:2006-01-02}}` 等格式，未提供有效时间时为空 |
| `{{ext}}` | 下载文件扩展名，不含开头的点 |
| `{{id}}` | 应用中的资源 ID |
| `{{kind}}` | 资源类型：`media.video` 或 `media.image` |
| `{{plugin}}` | 插件 ID：`official.wechat` |
| `{{host}}` | 下载地址的主域名 |
| `{{track}}` | 下载轨道 ID：`video-primary` 或 `image-primary` |
| `{{date}}` | 生成保存路径时的本地日期，默认 `20060102` |
| `{{time}}` | 生成保存路径时的本地时间，默认 `150405` |

## 开发与校验

在 `res-downloader` 项目根目录执行：

```bash
go run main.go plugin lint ./plugins/resd-plugin-wechat
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/video.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/image.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/detail-dedupe.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/page-script.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/dependency-script.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/page-script-log.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/dependency-script-log.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/inject-hook.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/media-created-at-hook.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/media-created-at.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/detail-hook.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/detail-created-at.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/finder-video.json
go run main.go plugin replay ./plugins/resd-plugin-wechat ./plugins/resd-plugin-wechat/fixtures/full-created-at.json
node --test ./plugins/resd-plugin-wechat/tests/creation-time.test.js
go run main.go plugin pack ./plugins/resd-plugin-wechat
```

Fixture 只包含脱敏后的虚构数据和示例地址。

## License

[MIT](LICENSE)
