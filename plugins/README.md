# 插件开发工作区

将独立插件仓库克隆到此目录，源码不提交到宿主仓库。在项目根目录校验和打包，例如：

```bash
git clone https://github.com/putyy/resd-plugin-wechat.git plugins/resd-plugin-wechat
go run main.go plugin lint ./plugins/resd-plugin-wechat
go run main.go plugin pack ./plugins/resd-plugin-wechat
```

<<<<<<< Updated upstream
`wails.json` 的 `plugins` 配置中的官方插件会在 `wails dev` 和 `wails build` 时自动内嵌。详见[维护应用内嵌插件](../docs/zh/development/plugins.md#维护应用内嵌插件)。
=======
`app.json` 的 `plugins` 配置中的官方插件会在 `wails3 task dev` 和 `wails3 task build` 时自动内嵌。详见[维护应用内嵌插件](../docs/zh/development/plugins.md#维护应用内嵌插件)。
>>>>>>> Stashed changes
