# 插件开发工作区

此目录用于本地开发和调试独立插件仓库。可以把插件仓库克隆到这里，再从项目根目录运行 `plugin lint`、`plugin replay` 和 `plugin pack`。

`plugins/` 下的插件源码默认不会提交到宿主仓库，也不会被宿主自动加载或打进应用；目录中只提交本说明。正式示例位于 `examples/plugins/`，随应用发布的内嵌插件位于 `internal/plugin/bundled/`。

例如：

```bash
git clone https://github.com/putyy/resd-plugin-wechat.git plugins/resd-plugin-wechat
go run main.go plugin lint ./plugins/resd-plugin-wechat
go run main.go plugin pack ./plugins/resd-plugin-wechat
```
