---
name: plugin-dev
description: Develop, debug, validate with sanitized offline fixtures, and package res-downloader site plugins from a target media URL. Use when adding website support, analyzing browser requests or client-side algorithms, creating or updating plugins under plugins/, generating sanitized fixtures, adjusting a local package version, or running plugin lint, replay, and pack. Do not use for unrelated application features, host-application changes, unauthorized access, or attacking protected DRM/CDM and license enforcement.
---

# Plugin Development

Turn a target media page into a minimal-permission res-downloader plugin with reproducible fixtures, validation evidence, and an installable ZIP.

## Inputs and scope

- For site-behavior changes, require at least one representative target URL. Ask for one only when none is provided. For README-only edits, follow `docs-sync` without repeating browser observation, plugin validation, or packaging unless the change affects those artifacts.
- Preserve user-specified plugin IDs, directories, capabilities, versions, and acceptance criteria. Do not increment the version for each debugging iteration of an unpublished change.
- Local version changes and repackaging stay in this workflow; they do not imply release, Git, or bundled-snapshot work. For version-only changes, reuse applicable validation evidence, lint the Manifest, repack, and verify the ZIP against the final source.
- For a new plugin without a requested directory, derive a stable site slug and use `plugins/resd-plugin-<site>`.
- Inspect the worktree before editing. Preserve unrelated changes and update an existing target plugin in place instead of overwriting it.
- Limit implementation changes to the target plugin directory. Do not modify the host application, shared plugin runtime, plugin protocol, CLI, UI, or bundled host capabilities as part of a plugin-development task.
- If the plugin needs a capability the host does not provide, do not add or change that capability. Clearly report the exact missing host capability, the affected plugin behavior, and the proposed host enhancement as a separate follow-up requirement. Do not claim the plugin is complete when that capability is required for acceptance.
- Do not commit, publish, or install the plugin unless the user explicitly requests that action.

## Source of truth

Before implementation, read `docs/zh/development/plugins.md` completely and inspect the closest relevant plugins under `plugins/`, `examples/plugins/`, or `internal/plugin/bundled/`.

Follow the current documentation when it conflicts with this skill. Do not duplicate the plugin protocol inside the skill. Use existing project commands and examples rather than inventing alternate tooling.

## Workflow

1. Establish what value the plugin must add beyond generic resource capture, such as reliable metadata, multiple qualities, cross-request association, expiring URL refresh, media merging, or site-specific processing.
2. When site behavior requires live evidence, observe normal playback and actual requests within the current user authorization. Distinguish website observation, bounded request diagnostics, and host acceptance; an observation label does not override a static-only restriction. Reuse explicit authorization for the same scope. For failed preview/download, noisy capture, or feed navigation, read [Browser media debugging](references/browser-media-debugging.md). For request failures, compare failing and working requests where possible; if evidence is unavailable, state the unresolved hypothesis instead of treating a guessed change as a confirmed fix.
3. Choose the simplest sufficient runtime:
   - Use `declarative` when one JSON response directly provides a single-track resource.
   - Use `javascript` for complex objects, multiple qualities, correlation, refresh, or custom download plans.
   - Add page scripts, response modification, capture, WASM, or advanced FFmpeg capabilities only when ordinary observation cannot meet the requirement.
4. For a new plugin, initialize the exact target directory with the project CLI before implementation:

   ```bash
   go run main.go plugin create ./plugins/resd-plugin-<site> <plugin-id> "<display name>"
   ```

   Do not run the scaffold command over an existing plugin. Keep the generated `README.md` and `.gitignore`; the default `.gitignore` entries are `.idea` and `.vscode`, and it must not ignore `dist/` unless the user explicitly requests that change.
   Keep the generated `settingsSchema.properties.enableLog` boolean setting with `default: false` and localized labels. Include the same setting when creating a new plugin by copying an example or writing a manifest. The host gates all `api.log()` calls on `enableLog === true`; do not duplicate this check in each hook. An absent setting disables plugin logging; do not add it to existing plugins unless requested. See `docs/zh/development/plugins.md` for the setting contract.
5. Implement the plugin with narrowly scoped host, path, content-type, body-read, body-limit, and capability declarations. New community plugins must not claim reserved `builtin.*` or `official.*` identities. Preserve an existing official plugin identity only when updating that plugin.
   - Reuse existing host resource kinds wherever possible, choosing by the actual resource content and purpose (video, audio, images, documents, or other resources), not by site. Add a custom resource kind only when the user explicitly requests it; do not create a resource kind for each site. Keep shared-kind labels generic and Manifest declarations consistent with emitted `kind` values. Use `source.pluginId` for provenance, `groupKey` for resource identity, and `handled` for generic-detector suppression.
   - Check interaction with `builtin.generic-detector`, not only successful resource extraction. Use a resource's `source.pluginId` to distinguish generic duplicates from legitimate plugin resources, including recommended works with metadata.
   - Define which observed player requests the plugin claims with `handled: true` and which retain generic fallback. Cover relevant default-port spellings, Range headers or query parameters, separate player tracks, preloads, and media arriving before metadata; do not rely solely on exact URL correlation when those requests differ.
   - Scope suppression to the site's supported traffic and explain any loss of fallback when metadata is unavailable. Do not suppress an entire CDN by default or modify playback responses merely to hide duplicate records.
6. Complete the plugin READMEs according to the [Plugin README conventions in docs-sync](../docs-sync/SKILL.md#plugin-readme-conventions). Use that section as the canonical writing guidance for new plugins and documentation updates.
   Add the smallest representative fixtures needed for each supported observation path and meaningful edge case.
7. Sanitize every fixture and log artifact. Remove cookies, authorization values, access tokens, account data, administrator credentials, private URLs, and unrelated user content. Preserve only fields required for matching and extraction.
8. Perform repository-local validation from the repository root:
   - Run `go run main.go plugin lint ./plugins/<plugin-directory>`.
   - For behavior changes, run `go run main.go plugin replay ./plugins/<plugin-directory> <fixture>` for every documented replay fixture. Replay is deterministic offline fixture validation; it is not live-site or application acceptance.
   - Check relevant host return values and merge semantics; offline mocks must reflect the actual contract.
   - Check the current replay schema and runner before choosing assertions. Zero emitted resources does not prove that generic fallback was suppressed: when replay cannot assert a critical behavior such as `handled`, add a minimal documented offline contract check. Do not infer full plugin-chain behavior from a single-plugin replay.
   - Observations outside the Manifest's match scope belong in offline contract checks, not fixtures expected to pass replay. Never broaden production permissions merely to make a negative fixture match. If a file under `fixtures/` is intentionally not a replay input, identify its role instead of silently skipping it.
   - Inspect the manifest, declared capabilities, source layout, fixture sanitization, and package inputs for consistency with `docs/zh/development/plugins.md`.
   - Run only other checks that are explicitly repository-local and documented by the plugin or repository.
   - Fix failures and repeat the affected checks.
   - Do not start the host application, install or reload the plugin, or perform live capture, preview, download, playback, or network integration as acceptance validation.
9. Record browser observations, request diagnostics, offline checks, and user acceptance separately. Preserve the scope of user-confirmed results; a version-only repackage does not invalidate unchanged behavior. Hand off remaining acceptance checks as described below.
10. After implementation and static validation are final, package the plugin as the last artifact-producing step:

   ```bash
   go run main.go plugin pack ./plugins/<plugin-directory>
   ```

11. Verify that `plugins/<plugin-directory>/dist/plugin.zip` exists, is non-empty, and contains the final Manifest version and matching packaged source files. If any packaged source changes afterward, rerun lint, all affected fixture replays, the relevant repository-local checks, and pack so the ZIP matches the final source.

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

## Completion report

Report the plugin ID/path/version, behavior and limitations, domains and capabilities, sanitized fixtures, validation results, and verified ZIP path. Identify any missing host capability as a separate follow-up. State what was observed, checked offline, confirmed by the user, or remains unverified; none substitutes for another.

For unverified or newly affected behavior, give a concrete manual checklist: installation/reload and page refresh, representative routes and login state, capture/title correspondence, preview, download, and output playback including audio or processing. Reuse prior user acceptance when applicable instead of requiring the entire checklist after a version-only change.

Finish the applicable validation and final package verification before handing off implementation. Do not claim application capture, preview, or download success from offline checks.
