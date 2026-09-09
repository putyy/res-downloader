---
description: Publish res-downloader plugins to the extension store using GitHub topics, versions, tags, and releases. Learn package structure and installation validation requirements.
---

# Publish Plugins to the Extension Store

This guide is for plugin authors and project maintainers. For installing, updating, or uninstalling plugins as a user, see [Plugin Management](../guide/plugin-management.md).

The extension store distributes plugins through GitHub repositories and releases; the website provides only the index. Developers do not need to submit code to a central plugin repository. Eligible public repositories are discovered periodically.

> The `res-downloader-ext` topic means the author has chosen to join the plugin ecosystem. It does not imply official review or a security endorsement.

## Publishing requirements and recommendations

An installable plugin needs the following:

1. A public, non-archived GitHub repository that is not a fork, with the `res-downloader-ext` topic.
2. One plugin per repository, with `plugin.json` at the repository root.
3. All runtime files, including JavaScript, WASM, and page scripts, committed to the repository. Do not depend on uncommitted local build output.
4. Preferably at least one sanitized fixture, with `plugin lint` and `plugin replay` passing before release.
5. A GitHub release that is neither a draft nor a prerelease, for example with tag `v1.2.0`.
6. A `plugin.json.version` of `1.2.0`, exactly matching the tag after removing an optional `v` prefix.

Repositories under the `putyy` account are marked **Official**; others are marked **Community**. Community plugin IDs cannot start with `builtin.` or `official.`, including case variants. Only official store entries and bundled plugins may use the `official.` prefix.

Commit a package at the fixed path `dist/plugin.zip` to improve download speeds for users in mainland China. The store retains the source archive for the GitHub release tag as a fallback; no separate release asset is required.

## Versions and tags

The store uses the latest stable release. `plugin.json.version` must exactly match its tag after removing an optional `v` prefix.

For any published content change:

1. Update the Manifest version.
2. Commit all runtime files and fixtures.
3. Create a new tag and release.
4. Wait for the store index to refresh.

Do not overwrite or move existing tags, or commit WASM, `dist/plugin.zip`, or other build files after the release. jsDelivr and GitHub source archives both use the commit referenced by the tag.

## Package structure and download acceleration

Store releases require `plugin.json` at the repository root. Recommended structure:

```text
repository-root/
├── plugin.json
├── main.js
├── fixtures/
├── tests/
├── decrypt.wasm
└── dist/
    └── plugin.zip
```

Run this command from the `res-downloader` source root to generate the package at its fixed path:

```bash
go run main.go plugin pack <plugin-directory>
```

Use `tests/` for your own JavaScript tests, preferably named `*.test.js`. The packer excludes `.git/`, `dist/`, `tests/`, and the output file itself; `fixtures/` is retained. `plugin.json` must be at the root of `plugin.zip`. Commit the latest `dist/plugin.zip` before creating the tag.

The store prefers `dist/plugin.zip` for faster installation and automatically falls back to the GitHub source archive if it is unavailable.

The installer ignores common macOS archive metadata and rejects symlinks, duplicate files, paths escaping the package, oversized files, and invalid Manifests. Do not include account details, capture logs, cookies, Authorization headers, real-user fixtures, or unrelated large files.

## Installation validation

The extension store does not publish a content SHA-256. The client still validates ZIP structure, extracted size, the Manifest, plugin ID, version, permissions, entry files, and runtime code. The package Manifest must match the `plugin.json` read by the index from the same tag.

Local ZIP installs can still display a canonical content digest for users to identify files. The store only checks whether a local package's plugin ID and version match an entry; this does not mean their contents are identical.

## Other distribution methods

Developers can share GitHub's automatically generated ZIP for the same tag through cloud storage or other channels. When a user selects it under **Plugins → Install ZIP**, the app first shows:

- Plugin ID, name, author, version, and API version.
- Requested domains and capabilities.
- The local ZIP's canonical content digest.
- Whether its plugin ID and version match a locally cached store entry.
- Whether it will replace an installed version.

Installation starts only after confirmation. Replacing an external plugin preserves its settings and retains the previous version for one-step rollback. An official store release can update a bundled JavaScript plugin with the same ID. Community or local ZIP packages cannot overwrite `official.` plugins.

## Release checklist

- `plugin.json` is at the repository root, with a stable ID associated with the repository and no reserved prefix.
- The version follows semantic versioning and matches the release tag.
- Permissions and domains are limited to what is actually needed.
- The README describes supported sites, main features, required settings, and known limitations.
- Fixtures, logs, and examples are sanitized.
- The final commit passes `go run main.go plugin lint <plugin-directory>` and all fixture replays.
- `go run main.go plugin pack <plugin-directory>` succeeds, and its output is committed before tagging.
- Installation checks succeed using both `dist/plugin.zip` and GitHub's generated source ZIP.
- The new version uses a new tag without moving or overwriting an old one.
