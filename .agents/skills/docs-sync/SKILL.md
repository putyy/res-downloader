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
- Check `docs/_sidebar.md` and `docs/_navbar.md` whenever pages are added, removed, renamed, or moved.
- Check `CONTRIBUTING.md` and `docs/contributing.md` only when contributor workflow changes.

Use the implementation change to route documentation review. This is an impact map, not a requirement to edit every listed file:

| Change area | Check first |
| --- | --- |
| Settings UI, defaults, configuration, or file selection | `docs/settings.md` |
| Startup failures, logs, recovery, platform prerequisites, or user troubleshooting | `docs/troubleshooting.md` |
| Module boundaries, lifecycle, persistence, communication, or major data flow | `docs/architecture.md` |
| Installation, first-run behavior, supported platforms, or headline capability | `README.md`, `README-EN.md`, and the relevant user guide |
| Plugin protocol, permissions, SDK, CLI, packaging, or publication | the plugin and SDK documentation listed below |

When one change affects both normal usage and failure recovery, update the task-oriented guide and troubleshooting guide together, using one canonical explanation and cross-links where appropriate.

## Plugin and SDK consistency

For plugin-facing changes, read `docs/plugins.md`, `docs/plugin-management.md`, `docs/extension-store.md`, and `docs/plugin-sdk/README.md` as applicable.

When the public plugin protocol changes, synchronize the relevant portions of:

- the Go protocol and validation behavior used as the source of truth;
- `docs/plugin-sdk/plugin-v1.schema.json`;
- `docs/plugin-sdk/plugin-v1.d.ts`;
- `docs/plugins.md`;
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

## Validation and handoff

Perform static validation only: inspect changed Markdown structure, relative links, navigation coverage, JSON validity for SDK schemas, and code/example consistency. Do not start the documentation site or use browser rendering as automated acceptance validation.

Report which source behavior drove the documentation changes, every synchronized document family, link or navigation checks, and any remaining translation or visual-rendering work. Tell the user exactly what to inspect manually on GitHub or the documentation site when rendering matters.
