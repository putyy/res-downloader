---
name: docs-sync
description: Synchronize res-downloader documentation after product, plugin, SDK, CLI, configuration, or workflow changes. Use when updating README variants, docs navigation, cross-links, examples, or public plugin documentation. Do not use for code-only implementation with no documentation impact.
---

# Documentation Sync

Keep public documentation aligned with the current implementation without copying implementation detail into every page.

## Establish the documentation impact

- Inspect the actual code or configuration change before editing documentation. Do not describe planned or assumed behavior as shipped behavior.
- Read the affected documentation completely and follow the existing terminology, audience, and language.
- Check `README.md` and its linked English counterpart `README-EN.md` when either is affected. If a linked counterpart is missing, report or restore the broken contract rather than silently ignoring it.
- Check `docs/.vitepress/config.mts` and the navigation in `docs/.vitepress/locales/` whenever pages are added, removed, renamed, or moved.
- Check `CONTRIBUTING.md` and `docs/zh/development/contributing.md` only when contributor workflow changes.

Use the implementation change to route documentation review. This is an impact map, not a requirement to edit every listed file:

| Change area | Check first |
| --- | --- |
| Settings UI, defaults, configuration, or file selection | `docs/zh/guide/settings.md` |
| Startup failures, logs, recovery, platform prerequisites, or user troubleshooting | `docs/zh/guide/troubleshooting.md` |
| Module boundaries, lifecycle, persistence, communication, or major data flow | `docs/zh/development/architecture.md` |
| Installation, first-run behavior, supported platforms, or headline capability | `README.md`, `README-EN.md`, and the relevant user guide |
| Plugin protocol, permissions, SDK, CLI, packaging, or publication | the plugin and SDK documentation listed below |

When one change affects both normal usage and failure recovery, update the task-oriented guide and troubleshooting guide together, using one canonical explanation and cross-links where appropriate.

## Plugin and SDK consistency

For plugin-facing changes, read `docs/zh/development/plugins.md`, `docs/zh/guide/plugin-management.md`, `docs/zh/development/extension-store.md`, and `docs/zh/development/plugin-sdk.md` as applicable.

When the public plugin protocol changes, synchronize the relevant portions of:

- the Go protocol and validation behavior used as the source of truth;
- `docs/public/plugin-sdk/plugin-v1.schema.json`;
- `docs/public/plugin-sdk/plugin-v1.d.ts`;
- `docs/zh/development/plugins.md`;
- affected examples under `examples/plugins/`;
- navigation and cross-links.

Do not update the SDK files speculatively. If code and SDK disagree, identify which behavior is authoritative and resolve the mismatch explicitly.

## Editing rules

- Preserve meaning across Chinese and English README variants; use natural language rather than sentence-by-sentence literal translation.
- Keep user guides task-oriented and move developer-only detail to developer documentation.
- Reuse one canonical explanation and link to it when duplication would drift.
- Update commands, paths, option names, UI labels, defaults, limitations, and screenshots only when supported by current repository state.
- Remove or repair stale links and navigation entries. Check relative paths and filename case because the documentation site and GitHub may resolve them differently.
- Preserve unrelated user edits and avoid broad prose rewrites unless the user requested them.

## Plugin README conventions

- Maintain Chinese `README.md` and English `README-EN.md` together, with `[中文](README.md) | [English](README-EN.md)` near the top of both files. Keep support, settings, limitations, and commands consistent across languages.
- In both introductions, link the application name as `[res-downloader](https://github.com/putyy/res-downloader)`.
- Keep the introduction focused on the plugin's purpose; omit the plugin ID and current version unless the user requests them.
- Follow the concise style of the WeChat plugin or the user's chosen reference. Focus on supported content, relevant settings, and necessary cautions. Keep implementation internals, API details, debugging history, fixture inventories, and acceptance evidence out of user-facing sections.
- Use a separate `安装` / `Installation` section with the same wording across site plugins. Do not append generic proxy or HTTPS setup, opening a website, browsing, or playback instructions. Use the shared wording below; adjust it only when the actual distribution method changes or the user requests it.
- Put essential plugin-specific operations, such as YouTube's capture action, in a short `使用` / `Usage` section only when needed. Put requirements such as FFmpeg, unsupported content, expired-link recovery, and restrictions during capture in `注意事项` / `Notes`.
- Retain `## 开发与校验` / `## Development and Validation`, including applicable lint, replay, test, and pack commands. Simplifying the user-facing explanation must not remove this section. Keep commands accurate for the target plugin; do not invent checks or remove relevant ones for brevity.

Shared Chinese installation text:

> 发布后可在 `res-downloader` 的“插件管理”页面安装。也可以下载对应版本的源码 ZIP，通过“从压缩包安装”导入。

Shared English installation text:

> Once published, the plugin can be installed from Plugin Management in `res-downloader`. You can also download the source ZIP for the desired version and import it using the option to install from an archive.

## Validation and handoff

Perform static validation only: inspect changed Markdown structure, relative links, navigation coverage, JSON validity for SDK schemas, and code/example consistency. Do not start the documentation site or use browser rendering as automated acceptance validation.

For README-only edits, check accuracy against the implementation, Markdown structure, and links. Do not rerun plugin lint or fixture replay without a relevant change. Check package inclusion before repacking: if only an excluded README changed, the existing ZIP remains valid and does not need regeneration.

Report which source behavior drove the documentation changes, every synchronized document family, link or navigation checks, and any remaining translation or visual-rendering work. Tell the user exactly what to inspect manually on GitHub or the documentation site when rendering matters.
