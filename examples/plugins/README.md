# 插件示例

本目录提供可复制、可离线测试的最小插件项目，不会被打包或安装到 res-downloader：

- `declarative-basic`：声明式 JSON/YAML 资源提取。
- `javascript-basic`：JavaScript observation、设置和下载计划。
- `page-command`：由资源操作向指定页面脚本发送插件自定义参数，领取唯一执行权并回报宿主可见的执行进度。
- `wasm-xor`：插件自带 WASM 下载处理器及其 WAT 源码。
- `x`：较完整的真实站点示例，展示复杂 JSON 遍历、多清晰度资源、设置和多个离线 fixture。

随应用分发的真实站点插件位于 `internal/plugin/bundled`。它们使用相同的公开插件协议，但承担生产适配职责，不作为最小教学模板。

建议先复制前四个最小示例之一；`x` 更适合在掌握基本协议后参考。可以使用项目提供的插件命令校验、回放 fixture 和打包，具体命令见[插件开发文档](../../docs/plugins.md)。
