---
name: plugin-dev
description: Develop, debug, validate with sanitized offline fixtures, and package res-downloader site plugins from a target media URL. Use when adding website support, analyzing browser requests or client-side algorithms, creating or updating plugins under plugins/, generating sanitized fixtures, or running plugin lint, replay, and pack. Do not use for unrelated application features, host-application changes, unauthorized access, or attacking protected DRM/CDM and license enforcement.
---

# Plugin Development

Turn a target media page into a minimal-permission res-downloader plugin with reproducible fixtures, validation evidence, and an installable ZIP.

## Inputs and scope

- Require at least one representative target URL. Ask for one only when none is provided.
- Preserve user-specified plugin IDs, directories, capabilities, and acceptance criteria.
- For a new plugin without a requested directory, derive a stable site slug and use `plugins/resd-plugin-<site>`.
- Inspect the worktree before editing. Preserve unrelated changes and update an existing target plugin in place instead of overwriting it.
- Limit implementation changes to the target plugin directory. Do not modify the host application, shared plugin runtime, plugin protocol, CLI, UI, or bundled host capabilities as part of a plugin-development task.
- If the plugin needs a capability the host does not provide, do not add or change that capability. Clearly report the exact missing host capability, the affected plugin behavior, and the proposed host enhancement as a separate follow-up requirement. Do not claim the plugin is complete when that capability is required for acceptance.
- Do not commit, publish, or install the plugin unless the user explicitly requests that action.

## Source of truth

Before implementation, read `docs/plugins.md` completely and inspect the closest relevant plugins under `plugins/`, `examples/plugins/`, or `internal/plugin/bundled/`.

Follow the current documentation when it conflicts with this skill. Do not duplicate the plugin protocol inside the skill. Use existing project commands and examples rather than inventing alternate tooling.

## Workflow

1. Establish what value the plugin must add beyond generic resource capture, such as reliable metadata, multiple qualities, cross-request association, expiring URL refresh, media merging, or site-specific processing.
2. Use an available browser or browser-debugging tool to load the representative page, trigger playback when needed, and inspect actual requests and responses. Treat this as development observation, not acceptance verification. Prefer observed behavior over guessed private endpoints. If browser access is unavailable and implementation depends on live traffic, report the missing observation evidence instead of fabricating fixtures or claiming support.
3. Choose the simplest sufficient runtime:
   - Use `declarative` when one JSON response directly provides a single-track resource.
   - Use `javascript` for complex objects, multiple qualities, correlation, refresh, or custom download plans.
   - Add page scripts, response modification, capture, WASM, or advanced FFmpeg capabilities only when ordinary observation cannot meet the requirement.
4. Implement the plugin with narrowly scoped host, path, content-type, body-read, body-limit, and capability declarations. New community plugins must not claim reserved `builtin.*` or `official.*` identities. Preserve an existing official plugin identity only when updating that plugin.
5. Add a concise README covering behavior, usage, limitations, and exact development commands. Add the smallest representative fixtures needed for each supported observation path and meaningful edge case.
6. Sanitize every fixture and log artifact. Remove cookies, authorization values, access tokens, account data, administrator credentials, private URLs, and unrelated user content. Preserve only fields required for matching and extraction.
7. Perform repository-local validation from the repository root:
   - Run `go run main.go plugin lint ./plugins/<plugin-directory>`.
   - Run `go run main.go plugin replay ./plugins/<plugin-directory> <fixture>` for every documented replay fixture. Replay is deterministic offline fixture validation; it is not live-site or application acceptance.
   - If a file under `fixtures/` is intentionally not a replay input, identify its role instead of silently skipping it.
   - Inspect the manifest, declared capabilities, source layout, fixture sanitization, and package inputs for consistency with `docs/plugins.md`.
   - Run only other checks that are explicitly repository-local and documented by the plugin or repository.
   - Fix failures and repeat the affected checks.
   - Do not start the host application, install or reload the plugin, or perform live capture, preview, download, playback, or network integration as acceptance validation.
8. Prepare an exact manual-verification checklist for the user. Cover installation or reload, the representative page and required login state, capture and metadata, preview, download, output playback, and any relevant refresh, merge, or site-specific processing behavior. Browser observation used during development does not count as acceptance verification.
9. After implementation and static validation are final, package the plugin as the last artifact-producing step:

   ```bash
   go run main.go plugin pack ./plugins/<plugin-directory>
   ```

10. Verify that `plugins/<plugin-directory>/dist/plugin.zip` exists and is non-empty. If any packaged source changes afterward, rerun lint, all affected fixture replays, the relevant repository-local checks, and pack so the ZIP matches the final source.

## Safety and stopping conditions

- When the current user's own authenticated browser session can access and play the representative content, proceed with resource acquisition unless a stopping condition below applies.
- Login, CAPTCHA, obfuscation, client-side algorithms, and non-DRM encryption are not stopping conditions by themselves. Pause for the user to complete login or CAPTCHA in the browser, then continue with the resulting authorized session.
- For content available to that session, inspect observed network traffic and client JavaScript or WASM as needed. The plugin may reproduce resource discovery, request signing, URL refresh, deobfuscation, non-DRM decryption or byte transforms, and media assembly performed by the site player.
- The plugin may transiently reuse the current session's credentials, short-lived tokens, signed URLs, and non-DRM content keys obtained through normal playback, such as HLS AES keys. Do not persist them in plugin source, fixtures, logs, or packaged artifacts.
- Stop when the task would require any of the following:
  - forging purchase, subscription, or other server-side entitlement;
  - using another user's cookies, tokens, licenses, or keys;
  - bypassing a paywall, geographic restriction, account ban, or access permission;
  - attacking or modifying a protected DRM/CDM, extracting protected DRM keys, or forging or altering license restrictions.
- Do not use wildcard domain access when the required hosts can be enumerated.
- Treat page-script messages and observed response bodies as untrusted input and validate required types and bounds.
- If the content remains unavailable after the user completes ordinary authentication, or a stopping condition applies, stop that route and explain what the user must provide or choose next.
- Lint and offline replay do not prove live support. Report browser findings only as development observations, never as acceptance verification, and leave application integration, capture, download, and playback verification to the user's manual checklist.

## Completion report

Report:

- plugin ID and source directory;
- supported resource behavior and known limitations;
- domains and capabilities requested;
- fixtures created and sanitization performed;
- lint, every fixture replay, and every other repository-local check result;
- the final ZIP path and whether it was verified non-empty;
- any missing host capability, its impact, and the proposed separate host enhancement;
- an explicit statement that dynamic behavior has not been automatically verified;
- the exact manual-verification checklist the user must complete, including capture, metadata, preview, download, and output playback where applicable.

The implementation handoff is complete only when lint, all applicable fixture replays, and other repository-local checks pass, packaging succeeds, the final ZIP exists, and the manual-verification checklist is provided. Never claim live capture, application integration, download, or playback success from offline checks; always tell the user that manual verification is still required.
