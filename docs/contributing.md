# 参与贡献

欢迎通过反馈问题、改进文档、开发插件或提交代码参与 res-downloader。

## 反馈问题

提交 [GitHub Issue](https://github.com/putyy/res-downloader/issues) 时，请尽量提供：

- 操作系统和应用版本；
- 相关插件及其版本；
- 可以重复执行的操作步骤；
- 实际结果和期望结果；
- 已脱敏的错误提示或日志。

请勿公开 Cookie、Authorization、账号、管理员密码、带私人签名的下载地址或其他敏感数据。

## 改进文档

普通用户文档应优先说明“在哪里操作、选项有什么作用、遇到问题怎么办”，避免加入不影响使用的内部实现细节。修正文案、补充截图、完善安装步骤和帮助翻译都可以直接提交 Pull Request。

## 开发插件

新增站点适配时，优先开发独立插件，不要把站点判断写入通用下载器。请从[插件开发指南](plugins.md)和[示例项目](https://github.com/putyy/res-downloader/tree/master/examples/plugins)开始，并提交至少一个不含隐私数据的离线 fixture。

公开插件可以按照[扩展商店发布说明](extension-store.md)发布，无需把插件代码合并到主项目。

## 贡献代码

Bug 修复和小型文档更新可以直接提交 Pull Request。新功能、架构调整或大型重构建议先创建 Issue 讨论，以免实现方向与项目规划不一致。

完整的 PR 范围、标题格式和提交前检查要求，以仓库根目录的 [CONTRIBUTING.md](https://github.com/putyy/res-downloader/blob/master/CONTRIBUTING.md) 为准。
